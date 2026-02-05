// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package main implements the maintenance manager service.
package main

import (
	"context"
	"flag"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/flags"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/oam"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	inv_util "github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/maintmgr"
	util "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

var zlog = logging.GetLogger("MaintenanceManagerMain")

var (
	RepoURL   = "https://github.com/open-edge-platform/infra-managers/maintenance.git"
	Version   = "<unset>"
	Revision  = "<unset>"
	BuildDate = "<unset>"
)

var (
	servaddr              = flag.String(flags.ServerAddress, ":50002", flags.ServerAddressDescription)
	invsvcaddr            = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	oamservaddr           = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	insecureGrpc          = flag.Bool(client.InsecureGrpc, true, client.InsecureGrpcDescription)
	caCertPath            = flag.String(client.CaCertPath, "", client.CaCertPathDescription)
	tlsCertPath           = flag.String(client.TLSCertPath, "", client.TLSCertPathDescription)
	tlsKeyPath            = flag.String(client.TLSKeyPath, "", client.TLSKeyPathDescription)
	enableTracing         = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	traceURL              = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)
	enableSanitizeGrpcErr = flag.Bool(util.EnableSanitizeGrpcErr, true, util.EnableSanitizeGrpcErrDescription)
	enableAuth            = flag.Bool(rbac.EnableAuth, true, rbac.EnableAuthDescription)
	rbacRules             = flag.String(rbac.RbacRules, "/rego/authz.rego", rbac.RbacRulesDescription)
	invCacheUUIDEnable    = flag.Bool(client.InvCacheUUIDEnable, false, client.InvCacheUUIDEnableDescription)
	invCacheStaleTimeout  = flag.Duration(
		client.InvCacheStaleTimeout, client.InvCacheStaleTimeoutDefault, client.InvCacheStaleTimeoutDescription)
	invCacheStaleTimeoutOffset = flag.Uint(
		client.InvCacheStaleTimeoutOffset, client.InvCacheStaleTimeoutOffsetDefault, client.InvCacheStaleTimeoutOffsetDescription)

	enableMetrics  = flag.Bool(metrics.EnableMetrics, false, metrics.EnableMetricsDescription)
	metricsAddress = flag.String(metrics.MetricsAddress, metrics.MetricsAddressDefault, metrics.MetricsAddressDescription)
)

func printSummary() {
	zlog.Info().Msg("Starting Maintenance Manager")
	zlog.InfraSec().Info().Msgf("RepoURL: %s, Version: %s, Revision: %s, BuildDate: %s\n", RepoURL, Version, Revision, BuildDate)
}

func SetTracing(traceURL string) func(context.Context) error {
	cleanup, exportErr := tracing.NewTraceExporterHTTP(traceURL, "MaintenanceManager", nil)
	if exportErr != nil {
		zlog.Err(exportErr).Msg("Error creating trace exporter")
	}
	if cleanup != nil {
		zlog.Info().Msgf("Tracing enabled %s", traceURL)
	} else {
		zlog.Info().Msg("Tracing disabled")
	}
	return cleanup
}

func main() {
	// Print a summary of the build
	printSummary()

	flag.Parse()

	// Start routine to handle any interrupt signals
	termChan := make(chan bool)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		close(termChan)
		maintmgr.CloseInvGrpcCli()
	}()

	if *enableTracing {
		cleanup := SetTracing(*traceURL)
		if cleanup != nil {
			defer func() {
				err := cleanup(context.Background())
				if err != nil {
					zlog.Err(err).Msg("Error in tracing cleanup")
				}
			}()
		}
	}
	// waitgroup so main will wait for all gRPC servers to exit cleanly
	wg := sync.WaitGroup{}
	// channel to signal the readiness
	var readyChan chan bool

	// Add OAM gRPC server
	if *oamservaddr != "" {
		wg.Add(1)
		readyChan = make(chan bool)

		go func() {
			if err := oam.StartOamGrpcServer(termChan, readyChan, &wg, *oamservaddr, *enableTracing); err != nil {
				zlog.InfraSec().Err(err).Msg("Cannot start Maintenance Manager OAM gRPC server, continuing...")
			}
		}()
	}

	cacheStaleTimeoutOffset, err := inv_util.UintToInt(*invCacheStaleTimeoutOffset)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Couldn't properly parse Cache Stale Timeout Offset")
	}

	err = maintmgr.StartInvGrpcCli(
		&wg,
		*enableTracing,
		*invsvcaddr,
		*caCertPath, *tlsKeyPath, *tlsCertPath,
		*insecureGrpc,
		*invCacheUUIDEnable, *invCacheStaleTimeout,
		cacheStaleTimeoutOffset,
		*enableMetrics,
	)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Couldn't create a NewInventoryClient")
	}

	// Create a listener on TCP port
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", *servaddr)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Error listening with TCP: %s", lis.Addr().String())
	}

	maintmgr.StartGrpcSrv(lis, readyChan, termChan, &wg,
		maintmgr.EnableAuth(*enableAuth),
		maintmgr.EnableSanitizeGrpcErr(*enableSanitizeGrpcErr),
		maintmgr.EnableTracing(*enableTracing),
		maintmgr.WithRbacRulesPath(*rbacRules),
		maintmgr.EnableMetrics(*enableMetrics),
		maintmgr.WithMetricsAddress(*metricsAddress),
	)

	// wait until servers terminate
	wg.Wait()
}

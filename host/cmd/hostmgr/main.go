// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package main implements the Host Manager service.
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
	"github.com/open-edge-platform/infra-managers/host/internal/hostmgr/handlers"
	"github.com/open-edge-platform/infra-managers/host/pkg/config"
	"github.com/open-edge-platform/infra-managers/host/pkg/hostmgr"
)

var zlog = logging.GetLogger("HostManagerMain")

var (
	RepoURL   = "https://github.com/open-edge-platform/infra-managers/host.git"
	Version   = "<unset>"
	Revision  = "<unset>"
	BuildDate = "<unset>"
)

var (
	servaddr           = flag.String(flags.ServerAddress, "0.0.0.0:50001", flags.ServerAddressDescription)
	invsvcaddr         = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	oamservaddr        = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	insecureGrpc       = flag.Bool(client.InsecureGrpc, true, client.InsecureGrpcDescription)
	caCertPath         = flag.String(client.CaCertPath, "", client.CaCertPathDescription)
	tlsCertPath        = flag.String(client.TLSCertPath, "", client.TLSCertPathDescription)
	tlsKeyPath         = flag.String(client.TLSKeyPath, "", client.TLSKeyPathDescription)
	enableTracing      = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	traceURL           = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)
	allowHostDiscovery = flag.Bool(
		hostmgr.AllowHostDiscovery,
		hostmgr.AllowHostDiscoveryValue,
		hostmgr.AllowHostDiscoveryDescription,
	)
	enableAuth           = flag.Bool(rbac.EnableAuth, true, rbac.EnableAuthDescription)
	rbacRules            = flag.String(rbac.RbacRules, "/rego/authz.rego", rbac.RbacRulesDescription)
	invCacheUUIDEnable   = flag.Bool(client.InvCacheUUIDEnable, false, client.InvCacheUUIDEnableDescription)
	invCacheStaleTimeout = flag.Duration(
		client.InvCacheStaleTimeout, client.InvCacheStaleTimeoutDefault, client.InvCacheStaleTimeoutDescription)
	invCacheStaleTimeoutOffset = flag.Uint(
		client.InvCacheStaleTimeoutOffset, client.InvCacheStaleTimeoutOffsetDefault, client.InvCacheStaleTimeoutOffsetDescription)

	enableMetrics  = flag.Bool(metrics.EnableMetrics, false, metrics.EnableMetricsDescription)
	metricsAddress = flag.String(metrics.MetricsAddress, metrics.MetricsAddressDefault, metrics.MetricsAddressDescription)
)

var (
	wg        = sync.WaitGroup{}        // waitgroup so main will wait for all go routines to exit cleanly
	readyChan = make(chan bool, 1)      // channel to signal the readiness.
	termChan  = make(chan bool, 1)      // channel to signal termination of main process.
	sigChan   = make(chan os.Signal, 1) // channel to handle any interrupt signals
)

func printSummary() {
	zlog.Info().Msg("Starting Host Manager")
	zlog.Info().Msgf("RepoURL: %s, Version: %s, Revision: %s, BuildDate: %s\n", RepoURL, Version, Revision, BuildDate)
}

func SetTracing(traceURL string) func(context.Context) error {
	cleanup, exportErr := tracing.NewTraceExporterHTTP(traceURL, "HostManager", nil)
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

func setOAM(oamAddress string, termChan, readyChan chan bool, wg *sync.WaitGroup) {
	if oamAddress != "" {
		// Add oam grpc server
		wg.Add(1)
		zlog.Info().Msg("initReadinessServer start.")
		go func() {
			if err := oam.StartOamGrpcServer(termChan, readyChan, wg, oamAddress, false); err != nil {
				zlog.InfraSec().InfraErr(err).Msg("Cannot start Host manager OAM gRPC server")
			}
		}()
	}
}

func main() {
	flag.Parse()

	conf := config.HostMgrConfig{
		EnableTracing:       *enableTracing,
		EnableMetrics:       *enableMetrics,
		TraceURL:            *traceURL,
		InventoryAddr:       *invsvcaddr,
		CACertPath:          *caCertPath,
		TLSKeyPath:          *tlsKeyPath,
		TLSCertPath:         *tlsCertPath,
		InsecureGRPC:        *insecureGrpc,
		EnableHostDiscovery: *allowHostDiscovery,
		EnableUUIDCache:     *invCacheUUIDEnable,
		UUIDCacheTTL:        *invCacheStaleTimeout,
		UUIDCacheTTLOffset:  int(*invCacheStaleTimeoutOffset),
	}
	if err := conf.Validate(); err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Failed to start due to invalid configuration: %v", conf)
	}

	zlog.Info().Msgf("Starting Host Manager conf %v", conf)
	// Print a summary of the build
	printSummary()

	if conf.EnableTracing {
		cleanup := SetTracing(conf.TraceURL)
		if cleanup != nil {
			defer func() {
				cleanErr := cleanup(context.Background())
				if cleanErr != nil {
					zlog.Err(cleanErr).Msg("Error in tracing cleanup")
				}
			}()
		}
	}
	go hostmgr.StartAvailableManager(termChan)

	setOAM(*oamservaddr, termChan, readyChan, &wg)

	invClient, invEvents, err := hostmgr.StartInvGrpcCli(
		&wg,
		conf,
	)
	if err != nil {
		zlog.Fatal().Msgf("Couldn't create a NewInventoryClient: %v", err)
	}

	nbHandler, err := handlers.NewNBHandler(invClient, invEvents)
	if err != nil {
		zlog.InfraSec().Fatal().Msgf("Failed to create HRM northbound handler: %v", err)
	}

	err = nbHandler.Start()
	if err != nil {
		zlog.InfraSec().Fatal().Msgf("Failed to start HRM northbound handler: %v", err)
	}

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		zlog.InfraSec().Info().Msg("Received Term Signal, will close all grpc connections.")
		// stops availability_mgr, oam_server and grpc_server
		close(termChan)
		// stops northbound handler
		nbHandler.Stop()
		// stops inventory client
		hostmgr.CloseInvGrpcCli()
	}()

	// Create a listener on TCP port
	lc := net.ListenConfig{}
	lis, listenErr := lc.Listen(context.Background(), "tcp", *servaddr)
	if listenErr != nil {
		zlog.InfraSec().Fatal().Err(listenErr).Msgf("Error listening with TCP on %s", *servaddr)
	}

	// Add host manager grpc server - it sets readyCHan to true
	hostmgr.StartGrpcSrv(lis, readyChan, termChan, &wg,
		hostmgr.EnableTracing(*enableTracing),
		hostmgr.EnableAuth(*enableAuth),
		hostmgr.WithRbacRulesPath(*rbacRules),
		hostmgr.EnableMetrics(*enableMetrics),
		hostmgr.WithMetricsAddress(*metricsAddress),
	)
	wg.Wait()
}

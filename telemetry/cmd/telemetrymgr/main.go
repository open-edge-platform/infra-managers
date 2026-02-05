// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package main provides the telemetry manager service entrypoint.
package main

import (
	"context"
	"flag"
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
	"github.com/open-edge-platform/infra-managers/telemetry/internal/handlers/northbound"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/handlers/southbound"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

var (
	name = "TelemetryManager"
	zlog = logging.GetLogger(name + "Main")

	inventoryAddress = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	oamServerAddress = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	enableTracing    = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	traceURL         = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)

	ServerAddress         = flag.String("ServerAddress", "localhost:5566", flags.ServerAddressDescription)
	enableAuth            = flag.Bool(rbac.EnableAuth, true, rbac.EnableAuthDescription)
	rbacRules             = flag.String(rbac.RbacRules, "/rego/authz.rego", rbac.RbacRulesDescription)
	enableSanitizeGrpcErr = flag.Bool("enableSanitizeGrpcErr", true, "enable to sanitize grpc error of each RPC call")

	enableVal = flag.Bool("enableVal", true, "Enables Inventory Telemetry Profile Validation.")

	invCacheUUIDEnable   = flag.Bool(client.InvCacheUUIDEnable, false, client.InvCacheUUIDEnableDescription)
	invCacheStaleTimeout = flag.Duration(
		client.InvCacheStaleTimeout, client.InvCacheStaleTimeoutDefault, client.InvCacheStaleTimeoutDescription)
	invCacheStaleTimeoutOffset = flag.Uint(
		client.InvCacheStaleTimeoutOffset, client.InvCacheStaleTimeoutOffsetDefault, client.InvCacheStaleTimeoutOffsetDescription)

	enableMetrics  = flag.Bool(metrics.EnableMetrics, false, metrics.EnableMetricsDescription)
	metricsAddress = flag.String(metrics.MetricsAddress, metrics.MetricsAddressDefault, metrics.MetricsAddressDescription)

	wg        = sync.WaitGroup{}
	readyChan = make(chan bool, 1)
	termChan  = make(chan bool, 1)
	sigChan   = make(chan os.Signal, 1)
)

var (
	RepoURL   = "https://github.com/open-edge-platform/infra-managers/telemetry.git"
	Version   = "<unset>"
	Revision  = "<unset>"
	BuildDate = "<unset>"
)

func printSummary() {
	zlog.Info().Msgf("Starting Telemetry Resource Manager")
	zlog.InfraSec().Info().Msgf("RepoURL: %s, Version: %s, Revision: %s, BuildDate: %s\n", RepoURL, Version, Revision, BuildDate)
}

func setupTracing(traceURL string) func(context.Context) error {
	cleanup, exportErr := tracing.NewTraceExporterHTTP(traceURL, name, nil)
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

func setupOamServerAndSetReady(enableTracing bool, oamServerAddress string) {
	if oamServerAddress != "" {
		// Add oam grpc server
		wg.Add(1)
		go func() {
			if err := oam.StartOamGrpcServer(termChan, readyChan, &wg, oamServerAddress, enableTracing); err != nil {
				zlog.InfraSec().Fatal().Err(err).Msg("Cannot start Inventory OAM gRPC server")
			}
		}()
		readyChan <- true
	}
}

func main() {
	// Print a summary of the build
	printSummary()
	flag.Parse()

	// Startup order, respecting deps
	// 1. Setup tracing
	// 2. Start Inventory client
	// 3. Start NBHandler and the reconcilers
	// 4. Start southbound handler
	// 5. Start the OAM server
	if *enableTracing {
		cleanup := setupTracing(*traceURL)
		if cleanup != nil {
			defer func() {
				err := cleanup(context.Background())
				if err != nil {
					zlog.Err(err).Msg("Error in tracing cleanup")
				}
			}()
		}
	}

	invClient, err := invclient.NewTelemetryInventoryClientWithOptions(
		&wg,
		invclient.WithInventoryAddress(*inventoryAddress),
		invclient.WithEnableTracing(*enableTracing),
		invclient.WithEnableUUIDCache(*invCacheUUIDEnable),
		invclient.WithUUIDCacheTTL(*invCacheStaleTimeout),
		invclient.WithUUIDCacheTTLOffset(*invCacheStaleTimeoutOffset),
		invclient.WithEnableMetrics(*enableMetrics),
	)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start TelemetryRM client")
	}

	nbHandler, err := northbound.NewNBHandler(invClient)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to create northbound handler")
	}
	err = nbHandler.Start()
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start northbound handler")
	}
	sbHandler, err := southbound.NewSBHandler(*ServerAddress, readyChan,
		*enableTracing, invClient, *enableAuth, *rbacRules, *enableVal, *enableSanitizeGrpcErr,
		*enableMetrics, *metricsAddress,
	)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to create southbound handler")
	}
	err = sbHandler.Start()
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start southbound handler")
	}

	setupOamServerAndSetReady(*enableTracing, *oamServerAddress)

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan // blocking

		// Terminate Telemetry RM when termination signal received
		close(termChan)
		sbHandler.Stop()
		nbHandler.Stop()
		invClient.Close()
	}()

	wg.Wait()
}

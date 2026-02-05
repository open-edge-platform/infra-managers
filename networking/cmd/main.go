// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the networking manager service.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/oam"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
	"github.com/open-edge-platform/infra-managers/networking/internal/handlers"
)

// Includes synch start/stop related objects.
var (
	name             = "NetworkingManager"
	zlog             = logging.GetLogger(name + "Main")
	inventoryAddress = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	oamservaddr      = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	enableTracing    = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	enableMetrics    = flag.Bool(metrics.EnableMetrics, false, metrics.EnableMetricsDescription)
	metricsAddress   = flag.String(metrics.MetricsAddress, metrics.MetricsAddressDefault, metrics.MetricsAddressDescription)
	traceURL         = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)
	wg               = sync.WaitGroup{}
	readyChan        = make(chan bool, 1)
	termChan         = make(chan bool, 1)
	sigChan          = make(chan os.Signal, 1)
)

var (
	RepoURL   = "https://github.com/open-edge-platform/infra-managers/networking.git"
	Version   = "<unset>"
	Revision  = "<unset>"
	BuildDate = "<unset>"
)

func printSummary() {
	zlog.Info().Msgf("Starting Networking Manager")
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

func startMetricsServer() {
	metrics.StartMetricsExporter([]prometheus.Collector{metrics.GetClientMetricsWithLatency()},
		metrics.WithListenAddress(*metricsAddress))
}

func setupOamServer(enableTracing bool, oamservaddr string) {
	if oamservaddr != "" {
		// Add oam grpc server
		wg.Add(1)
		go func() {
			if err := oam.StartOamGrpcServer(termChan, readyChan, &wg, oamservaddr, enableTracing); err != nil {
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
	// Startup process, respecting deps
	// 1. Setup tracing
	// 2. Start NetRM Inventory client
	// 3. Start NBHandler and the reconcilers
	// 4. Start the OAM server
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
	if *enableMetrics {
		startMetricsServer()
	}
	netClient, err := clients.NewNetInventoryClientWithOptions(
		clients.WithInventoryAddress(*inventoryAddress),
		clients.WithEnableTracing(*enableTracing),
		clients.WithEnableMetrics(*enableMetrics),
	)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start NetRM client")
	}
	nbHandler, err := handlers.NewNBHandler(netClient, *enableTracing)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to create NBHandler")
	}
	err = nbHandler.Start()
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start NBHandler")
	}
	setupOamServer(*enableTracing, *oamservaddr)

	// After the initialization blocks on sigChan
	// waiting on the term channel.
	// 1. Stop the OAM server
	// 2. Stop the NBHandler
	// 3. Stop the NetRM Inventory client
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	<-sigChan
	close(termChan)
	nbHandler.Stop()
	netClient.Stop()

	wg.Done()
}

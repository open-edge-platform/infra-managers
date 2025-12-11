// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/oam"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/common"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/controller"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/invclient"
)

var (
	name = "OSRM"
	zlog = logging.GetLogger(name + "main")

	inventoryAddress = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	oamServerAddress = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	enableTracing    = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	traceURL         = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)

	insecureGrpc = flag.Bool(client.InsecureGrpc, true, client.InsecureGrpcDescription)

	caCertPath  = flag.String(client.CaCertPath, "", client.CaCertPathDescription)
	tlsCertPath = flag.String(client.TLSCertPath, "", client.TLSCertPathDescription)
	tlsKeyPath  = flag.String(client.TLSKeyPath, "", client.TLSKeyPathDescription)

	osProfileRevision = flag.String(common.OsProfileRevision, "", common.OsProfileRevisionDescription)
	enabledProfiles   = flag.String(common.EnabledProfiles, "", common.EnabledProfilesDescription)
	defaultProfile    = flag.String(common.DefaultProfile, "", common.DefaultProfileDescription)
	autoProvision     = flag.Bool(common.AutoprovisionFlag, false, common.AutoprovisionDescription)

	enableMetrics  = flag.Bool(metrics.EnableMetrics, false, metrics.EnableMetricsDescription)
	metricsAddress = flag.String(metrics.MetricsAddress, metrics.MetricsAddressDefault,
		metrics.MetricsAddressDescription)
	osSecurityFeatureEnable = flag.Bool(common.OSSecurityFeatureEnable, false, common.OSSecurityFeatureEnableDescription)
	inventoryTickerPeriod   = flag.String("inventory-ticker-period", "12h", "Inventory ticker period (e.g., 12h, 1h, 30m)")
)

var (
	// waitgroups and channels.
	wg           = sync.WaitGroup{}
	oamReadyChan = make(chan bool, 1) // used for readiness indicators to OAM
	termChan     = make(chan bool, 1)
	sigChan      = make(chan os.Signal, 1)
)

var (
	RepoURL   = "https://github.com/open-edge-platform/infra-managers/os-resource.git"
	Version   = "<unset>"
	Revision  = "<unset>"
	BuildDate = "<unset>"
)

func StartupSummary() {
	zlog.Info().Msg("Starting " + name)
	zlog.Info().Msgf("RepoURL: %s, Version: %s, Revision: %s, BuildDate: %s\n", RepoURL, Version, Revision, BuildDate)
}

func SetupTracing(traceURL string) func(context.Context) error {
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

func SetupOamServerAndSetReady(enableTracing *bool, oamServerAddress *string) {
	zlog.Info().Msgf("OAM server address: %s", *oamServerAddress)

	if *oamServerAddress != "" {
		wg.Add(1) // Add oam grpc server to waitgroup

		zlog.Info().Msg("Starting OAM server")

		go func() {
			if err := oam.StartOamGrpcServer(termChan, oamReadyChan, &wg, *oamServerAddress, *enableTracing); err != nil {
				zlog.InfraSec().Fatal().Err(err).Msg("Cannot start " + name + " gRPC server")
			}
		}()
		oamReadyChan <- true
	}
}

//nolint:cyclop // main function complexity is acceptable
func main() {
	// Print a summary of build information
	StartupSummary()

	// Parse flags
	flag.Parse()

	// Tracing, if enabled
	if *enableTracing {
		cleanup := SetupTracing(*traceURL)
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

	// connect to Inventory
	invClient, err := invclient.NewInventoryClient(termChan, &wg, *enableTracing,
		*inventoryAddress, *caCertPath, *tlsKeyPath, *tlsCertPath, *insecureGrpc, *enableMetrics)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Cannot create a new InventoryClient")
	}

	// Parse inventory ticker period from environment variable or flag
	tickerPeriodStr := *inventoryTickerPeriod
	if envTickerPeriod := os.Getenv("INVENTORY_TICKER_PERIOD"); envTickerPeriod != "" {
		tickerPeriodStr = envTickerPeriod
		zlog.Debug().Msgf("Using INVENTORY_TICKER_PERIOD from environment: %s", tickerPeriodStr)
	}

	tickerPeriod, parseErr := time.ParseDuration(tickerPeriodStr)
	if parseErr != nil {
		zlog.InfraSec().Fatal().Err(parseErr).Msgf(
			"Invalid inventory-ticker-period format: %s (use Go duration format, e.g., 12h, 1h, 30m)",
			tickerPeriodStr)
	}

	osConfig := common.OsConfig{
		EnabledProfiles:         strings.Split(*enabledProfiles, ","),
		OsProfileRevision:       *osProfileRevision,
		DefaultProfile:          *defaultProfile,
		AutoProvision:           *autoProvision,
		OSSecurityFeatureEnable: *osSecurityFeatureEnable,
		InventoryTickerPeriod:   tickerPeriod,
	}

	if validateErr := osConfig.Validate(); validateErr != nil {
		zlog.InfraSec().Fatal().Err(validateErr).Msg("Failed to start due to invalid config")
	}

	osResourceController, err := controller.New(invClient, osConfig)
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to create OS resource controller")
	}

	err = osResourceController.Start()
	if err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Unable to start OS resource controller")
	}

	// start OAM server
	SetupOamServerAndSetReady(enableTracing, oamServerAddress)

	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT, syscall.SIGTSTP)
	go func() {
		<-sigChan // block until signals received

		close(termChan) // closes the SBgRPC server, OAM Server

		invClient.Close() // stop inventory client
	}()

	wg.Wait()
}

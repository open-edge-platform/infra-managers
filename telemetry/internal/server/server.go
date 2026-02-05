// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package server provides gRPC server functionality for telemetry manager.
package server

import (
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
	telemetrymgr "github.com/open-edge-platform/infra-managers/telemetry/internal/telemetrymgrsvc"
	pb "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
)

var zlog = logging.GetLogger("TelemetryServergRPC")

// EnableAuth enables authentication for the server.
func EnableAuth(enable bool) Option {
	return func(o *Options) {
		o.enableAuth = enable
	}
}

// EnableSanitizeGrpcErr enables gRPC error sanitization.
func EnableSanitizeGrpcErr(enable bool) Option {
	return func(o *Options) {
		o.enableSanitizeGrpcErr = enable
	}
}

// EnableTracing enables distributed tracing.
func EnableTracing(enable bool) Option {
	return func(o *Options) {
		o.enableTracing = enable
	}
}

// EnableValidate enables request validation.
func EnableValidate(enable bool) Option {
	return func(o *Options) {
		o.enableValidate = enable
	}
}

// WithRbacRulesPath sets the RBAC rules path.
func WithRbacRulesPath(rbacPath string) Option {
	return func(o *Options) {
		o.rbacRulesPath = rbacPath
	}
}

// EnableMetrics enables metrics collection.
func EnableMetrics(enable bool) Option {
	return func(o *Options) {
		o.enableMetrics = enable
	}
}

// WithMetricsAddress sets the metrics server address.
func WithMetricsAddress(metricsAddress string) Option {
	return func(o *Options) {
		o.metricsAddress = metricsAddress
	}
}

func parseOptions(opts ...Option) *Options {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	return options
}

// Options contains server configuration options.
type Options struct {
	enableAuth            bool
	enableValidate        bool
	enableSanitizeGrpcErr bool
	enableTracing         bool
	rbacRulesPath         string
	enableMetrics         bool
	metricsAddress        string
}

// Option is a functional option for configuring the server.
type Option func(*Options)

func telemetrymgrGrpcServer(
	lis net.Listener,
	termChan chan bool,
	readyChan chan bool,
	wg *sync.WaitGroup,
	telCli *invclient.TelemetryInventoryClient,
	options ...Option,
) {
	var srvOpts []grpc.ServerOption
	var unaryInter []grpc.UnaryServerInterceptor

	opts := parseOptions(options...)

	srvMetrics := metrics.GetServerMetricsWithLatency()
	if opts.enableMetrics {
		zlog.Info().Msgf("Metrics exporter is enabled")
		unaryInter = append(unaryInter, srvMetrics.UnaryServerInterceptor())
	}

	// Enables to sanitize grpc error per gRPC call
	if opts.enableSanitizeGrpcErr {
		zlog.InfraSec().Info().Msgf("enabling to sanitize grpc error per gRPC call")
		unaryInter = append(unaryInter, inv_errors.GetSanitizeErrorGrpcInterceptor())
	}

	// Enables tracing in gRPC southbound server
	if opts.enableTracing {
		srvOpts = tracing.EnableGrpcServerTracing(srvOpts)
	}

	unaryInter = append(unaryInter, tenant.GetExtractTenantIDInterceptor(tenant.GetAgentsRole()))

	srvOpts = append(srvOpts, grpc.ChainUnaryInterceptor(unaryInter...))

	gsrv := grpc.NewServer(srvOpts...)

	var opaPolicy *rbac.Policy
	if opts.enableAuth {
		zlog.Info().Msg("Authentication is enabled, starting RBAC server")
		var err error
		// start OPA server with policies
		opaPolicy, err = rbac.New(opts.rbacRulesPath)
		if err != nil {
			zlog.Fatal().Msg("Failed to start RBAC OPA server")
		}
	}

	// register server
	pb.RegisterTelemetryMgrServer(gsrv, telemetrymgr.NewTelemetrymgrServer(
		telCli, opts.enableAuth, opaPolicy, opts.enableValidate,
	))

	// enable reflection
	reflection.Register(gsrv)

	if opts.enableMetrics {
		// Register metrics
		srvMetrics.InitializeMetrics(gsrv)
		metrics.StartMetricsExporter([]prometheus.Collector{metrics.GetClientMetricsWithLatency(), srvMetrics},
			metrics.WithListenAddress(opts.metricsAddress))
	}

	wg.Add(1)
	// in goroutine signal is ready and then serve
	go func() {
		// On testing will be nil
		if readyChan != nil {
			readyChan <- true
		}

		err := gsrv.Serve(lis)
		if err != nil {
			zlog.InfraSec().Fatal().Err(err).Msg("failed to serve")
		}
	}()

	// handle termination signals
	termSig := <-termChan
	if termSig {
		gsrv.Stop()
		zlog.Info().Msg("stopping server")
	}

	// exit WaitGroup when done
	wg.Done()
}

// StartTelemetrymgrGrpcServer starts the telemetry manager gRPC server.
func StartTelemetrymgrGrpcServer(
	termChan chan bool,
	readyChan chan bool,
	wg *sync.WaitGroup,
	lis net.Listener,
	telCli *invclient.TelemetryInventoryClient,
	options ...Option,
) {
	zlog.InfraSec().Info().Str("address", lis.Addr().String()).Msg("started to listen")
	telemetrymgrGrpcServer(lis, termChan, readyChan, wg, telCli, options...)
}

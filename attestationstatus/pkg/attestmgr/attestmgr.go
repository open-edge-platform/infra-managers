// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package attestmgr implements the Attestation Status Manager gRPC service.
package attestmgr

import (
	"net"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	attestmgr_sb "github.com/open-edge-platform/infra-managers/attestationstatus/pkg/api/attestmgr/v1"
)

var zlog = logging.GetLogger("AttestationStatusManager")

// Options contains configuration options for the Attestation Status Manager.
type Options struct {
	enableAuth     bool
	enableTracing  bool
	rbacRulesPath  string
	enableMetrics  bool
	metricsAddress string
}

// Option is a function that configures Options.
type Option func(*Options)

func parseOptions(opts ...Option) *Options {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	return options
}

// EnableAuth returns an Option that enables or disables authentication.
func EnableAuth(enable bool) Option {
	return func(o *Options) {
		o.enableAuth = enable
	}
}

// EnableTracing returns an Option that enables or disables distributed tracing.
func EnableTracing(enable bool) Option {
	return func(o *Options) {
		o.enableTracing = enable
	}
}

// WithRbacRulesPath returns an Option that sets the path to RBAC rules.
func WithRbacRulesPath(rbacPath string) Option {
	return func(o *Options) {
		o.rbacRulesPath = rbacPath
	}
}

// EnableMetrics returns an Option that enables or disables metrics collection.
func EnableMetrics(enable bool) Option {
	return func(o *Options) {
		o.enableMetrics = enable
	}
}

// WithMetricsAddress returns an Option that sets the metrics exporter address.
func WithMetricsAddress(metricsAddress string) Option {
	return func(o *Options) {
		o.metricsAddress = metricsAddress
	}
}

// StartSBGrpcSrv starts the southbound gRPC server for Attestation Status Manager.
func StartSBGrpcSrv(
	lis net.Listener,
	readyChan chan bool,
	termChan chan bool,
	wg *sync.WaitGroup,
	options ...Option,
) {
	zlog.Info().Msg("Start Attestation Status Manager gRPC Server")

	opts := parseOptions(options...)

	var srvOpts []grpc.ServerOption
	var unaryInter []grpc.UnaryServerInterceptor

	srvMetrics := metrics.GetServerMetricsWithLatency()
	if opts.enableMetrics {
		zlog.Info().Msgf("Metrics exporter is enabled")
		unaryInter = append(unaryInter, srvMetrics.UnaryServerInterceptor())
	}

	// Enables tracing in gRPC southbound server
	if opts.enableTracing {
		srvOpts = tracing.EnableGrpcServerTracing(srvOpts)
	}

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

	unaryInter = append(unaryInter, tenant.GetExtractTenantIDInterceptor(tenant.GetAgentsRole()))

	srvOpts = append(srvOpts, grpc.ChainUnaryInterceptor(unaryInter...))

	// Create a gRPC server object
	s := grpc.NewServer(srvOpts...)
	// Attach the attestmgr service to the server
	attestmgr_sb.RegisterAttestationStatusMgrServiceServer(s, &server{
		rbac:        opaPolicy,
		authEnabled: opts.enableAuth,
	})
	reflection.Register(s)
	// Serve gRPC server when signal is ready
	zlog.Info().Msgf("Serving Southbound gRPC on %s", lis.Addr().String())

	if opts.enableMetrics {
		// Register metrics
		srvMetrics.InitializeMetrics(s)
		metrics.StartMetricsExporter([]prometheus.Collector{metrics.GetClientMetricsWithLatency(), srvMetrics},
			metrics.WithListenAddress(opts.metricsAddress))
	}

	wg.Add(1)
	go func() {
		// mark as ready before block function
		if readyChan != nil {
			readyChan <- true
		}

		if err := s.Serve(lis); err != nil {
			zlog.Fatal().Msg("Cannot start Attestation Status Manager gRPC server")
		}
	}()

	// handle termination signals
	termSig := <-termChan
	if termSig {
		s.Stop()
		zlog.Info().Msg("Stopping Attestation Status Manager gRPC server")
	}

	// exit WaitGroup when done
	wg.Done()
}

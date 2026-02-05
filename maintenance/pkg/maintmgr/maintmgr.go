// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package maintmgr implements the core maintenance manager functionality.
package maintmgr

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
)

var zlog = logging.GetLogger("MaintenanceManager")

// eventsWatcherBufSize is the buffer size for the events channel.
var eventsWatcherBufSize = 10

// TODO(max): remove global instances.
var invMgrCli invclient.InvGrpcClient

// EnableAuth enables authentication for the maintenance manager.
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

// EnableTracing enables tracing for the maintenance manager.
func EnableTracing(enable bool) Option {
	return func(o *Options) {
		o.enableTracing = enable
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

// WithMetricsAddress sets the metrics address.
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

// Options contains configuration options for the maintenance manager.
type Options struct {
	enableAuth            bool
	enableSanitizeGrpcErr bool
	enableTracing         bool
	rbacRulesPath         string
	enableMetrics         bool
	metricsAddress        string
}

// Option is a functional option for configuring the maintenance manager.
type Option func(*Options)

// StartInvGrpcCli starts the inventory gRPC client.
func StartInvGrpcCli(
	wg *sync.WaitGroup,
	enableTracing bool,
	invsvcaddr, caCertPath, tlsKeyPath, tlsCertPath string,
	insecureGrpc, enableUUIDCache bool,
	uuidCacheTTL time.Duration, uuidCacheTTLOffset int, enableMetrics bool,
) error {
	zlog.InfraSec().Info().Msgf("Starting Inventory client. invAddress=%s", invsvcaddr)

	ctx := context.Background()
	kinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_OS,
		inv_v1.ResourceKind_RESOURCE_KIND_HOST,
		inv_v1.ResourceKind_RESOURCE_KIND_SINGLESCHEDULE,
		inv_v1.ResourceKind_RESOURCE_KIND_REPEATEDSCHEDULE,
	}

	events := make(chan *inv_client.WatchEvents, eventsWatcherBufSize)

	cfg := inv_client.InventoryClientConfig{
		Name:                      "maintmgr",
		Address:                   invsvcaddr,
		Events:                    events,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		ClientKind:                inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		ResourceKinds:             kinds,
		EnableTracing:             enableTracing,
		EnableMetrics:             enableMetrics,
		Wg:                        wg,
		SecurityCfg: &inv_client.SecurityConfig{
			CaPath:   caCertPath,
			KeyPath:  tlsKeyPath,
			CertPath: tlsCertPath,
			Insecure: insecureGrpc,
		},
		ClientCache: inv_client.InvClientCacheConfig{
			EnableUUIDCache: enableUUIDCache,
			StaleTime:       uuidCacheTTL,
			StateTimeOffset: uuidCacheTTLOffset,
		},
	}
	gcli, err := inv_client.NewTenantAwareInventoryClient(ctx, cfg)
	if err != nil {
		return err
	}

	scheduleCache, err := schedule_cache.NewScheduleCacheClientWithOptions(ctx,
		schedule_cache.WithInventoryAddress(cfg.Address),
		schedule_cache.WithEnableTracing(cfg.EnableTracing),
	)
	if err != nil {
		return err
	}
	hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
	if err != nil {
		return err
	}

	invGrpcClient := invclient.NewInvGrpcClient(gcli, hScheduleCache)

	SetInvGrpcCli(invGrpcClient)

	go func() {
		for {
			_, ok := <-events
			if !ok {
				zlog.InfraSec().Fatal().Msg("gRPC stream with inventory closed")
			}
		}
	}()
	return nil
}

// SetInvGrpcCli sets the inventory gRPC client instance.
func SetInvGrpcCli(cli invclient.InvGrpcClient) {
	invMgrCli = cli
}

// CloseInvGrpcCli closes the inventory gRPC client connection.
func CloseInvGrpcCli() {
	zlog.InfraSec().Info().Msg("Stopping Inventory client")

	if invMgrCli.InvClient == nil {
		zlog.InfraSec().InfraError("invMgrCli InvClient not set").Msg("")
		return
	}
	err := invMgrCli.InvClient.Close()
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
	}
	zlog.InfraSec().Info().Msg("Stopping Inventory Cache client")
	err = invMgrCli.HScheduleCacheClient.Close()
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
	}
	invMgrCli.HScheduleCacheClient = nil
}

// StartGrpcSrv starts the maintenance manager gRPC server.
func StartGrpcSrv(
	lis net.Listener,
	readyChan chan bool,
	termChan chan bool,
	wg *sync.WaitGroup,
	options ...Option,
) {
	zlog.InfraSec().Info().Msgf("Starting SB gRPC server on: %s", lis.Addr().String())

	opts := parseOptions(options...)

	var srvOpts []grpc.ServerOption
	var unaryInter []grpc.UnaryServerInterceptor

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

	// Create a gRPC server with UnaryInterceptor and tracing
	s := grpc.NewServer(srvOpts...)
	// Attach the maintmgr service to the server
	pb.RegisterMaintmgrServiceServer(s, &server{
		rbac:        opaPolicy,
		authEnabled: opts.enableAuth,
	})
	// enable reflection
	reflection.Register(s)

	if opts.enableMetrics {
		// Register metrics
		srvMetrics.InitializeMetrics(s)
		metrics.StartMetricsExporter([]prometheus.Collector{
			metrics.GetClientMetricsWithLatency(),
			srvMetrics,
		}, metrics.WithListenAddress(opts.metricsAddress))
	}

	wg.Add(1)
	// mark as ready before block function
	go func() {
		// mark as ready before block function
		if readyChan != nil {
			readyChan <- true
		}

		if err := s.Serve(lis); err != nil {
			zlog.InfraSec().Fatal().Msg("Cannot start SB gRPC server")
		}
	}()

	// handle termination signals
	termSig := <-termChan
	if termSig {
		s.Stop()
		zlog.InfraSec().Info().Msg("Stopping gRPC server")
	}

	wg.Done()
}

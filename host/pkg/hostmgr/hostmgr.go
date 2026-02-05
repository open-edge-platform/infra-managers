// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	"github.com/open-edge-platform/infra-managers/host/pkg/config"
	inv_mgr_cli "github.com/open-edge-platform/infra-managers/host/pkg/invclient"
)

var zlog = logging.GetLogger("HostManager")

// TODO(max): remove global instances.
var (
	invClientInstance       inv_client.TenantAwareInventoryClient
	AllowHostDiscoveryValue = true // Default value in flag
)

const (
	// AllowHostDiscovery enables automatic host discovery.
	AllowHostDiscovery = "allowHostDiscovery"
	// AllowHostDiscoveryDescription provides description of the AllowHostDiscovery flag.
	AllowHostDiscoveryDescription = "Flag to allow Host discovery automatically when it does not exist in the Inventory"
	// Backoff config for retrying the SetHostConnectionLost.
	backoffInterval = 5 * time.Second
	backoffRetries  = uint64(5)
	// SetHostConnectionLost does 2 operation with Inventory, plus we leave some slack time.
	nOperationInventoryHostConnLost = 2.05
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

// EnableAuth enables authentication for the host manager.
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

// WithRbacRulesPath sets the path to RBAC rules configuration.
func WithRbacRulesPath(rbacPath string) Option {
	return func(o *Options) {
		o.rbacRulesPath = rbacPath
	}
}

// EnableMetrics enables metrics collection for the host manager.
func EnableMetrics(enable bool) Option {
	return func(o *Options) {
		o.enableMetrics = enable
	}
}

// WithMetricsAddress sets the address for metrics server.
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

// Options contains configuration options for the host manager.
type Options struct {
	enableAuth     bool
	enableTracing  bool
	rbacRulesPath  string
	enableMetrics  bool
	metricsAddress string
}

// Option is a functional option for configuring the host manager.
type Option func(*Options)

// StartInvGrpcCli starts the inventory gRPC client.
func StartInvGrpcCli(
	wg *sync.WaitGroup,
	conf config.HostMgrConfig,
) (inv_client.TenantAwareInventoryClient, chan *inv_client.WatchEvents, error) {
	ctx := context.Background()
	resourceKinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_HOST,
		inv_v1.ResourceKind_RESOURCE_KIND_INSTANCE,
	}
	zlog.InfraSec().Info().Msg("initial Inv Grpc Client start.")

	events := make(chan *inv_client.WatchEvents, eventsWatcherBufSize)
	cfg := inv_client.InventoryClientConfig{
		Name:                      "hostmgr",
		Address:                   conf.InventoryAddr,
		Events:                    events,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		ClientKind:                inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		ResourceKinds:             resourceKinds,
		EnableTracing:             conf.EnableTracing,
		EnableMetrics:             conf.EnableMetrics,
		Wg:                        wg,
		SecurityCfg: &inv_client.SecurityConfig{
			CaPath:   conf.CACertPath,
			KeyPath:  conf.TLSKeyPath,
			CertPath: conf.TLSCertPath,
			Insecure: conf.InsecureGRPC,
		},
		ClientCache: inv_client.InvClientCacheConfig{
			EnableUUIDCache: conf.EnableUUIDCache,
			StaleTime:       conf.UUIDCacheTTL,
			StateTimeOffset: conf.UUIDCacheTTLOffset,
		},
	}

	gcli, err := inv_client.NewTenantAwareInventoryClient(ctx, cfg)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Cannot create new inventory client")
		return nil, nil, err
	}

	SetInvGrpcCli(gcli)
	zlog.InfraSec().Info().Msg("initial Grpc Client preparation is done.")
	AllowHostDiscoveryValue = conf.EnableHostDiscovery

	return gcli, events, nil
}

// SetInvGrpcCli sets the inventory gRPC client.
func SetInvGrpcCli(gcli inv_client.TenantAwareInventoryClient) {
	invClientInstance = gcli
}

// CloseInvGrpcCli closes the inventory gRPC client connection.
func CloseInvGrpcCli() {
	if err := invClientInstance.Close(); err != nil {
		zlog.Warn().Err(err).Msg("Failed to close inventory client")
	}
}

// StartGrpcSrv starts the host manager gRPC server.
func StartGrpcSrv(
	lis net.Listener,
	readyChan chan bool,
	termChan chan bool,
	wg *sync.WaitGroup,
	options ...Option,
) {
	zlog.Info().Msg("Start gRPC Server")

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
	// Attach the hostmgr service to the server
	pb.RegisterHostmgrServer(s, &server{
		rbac:        opaPolicy,
		authEnabled: opts.enableAuth,
	})
	reflection.Register(s)
	// Serve gRPC server when signal is ready
	zlog.Info().Msgf("Serving gRPC on %s", lis.Addr().String())

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
			zlog.Fatal().Msg("Cannot start Host Manager")
		}
	}()

	// handle termination signals
	termSig := <-termChan
	if termSig {
		s.Stop()
		zlog.Info().Msg("stopping server")
	}

	// exit WaitGroup when done
	wg.Done()
}

// StartAvailableManager starts the availability manager.
func StartAvailableManager(termChan chan bool) {
	ctx := context.Background()
	zlog.Info().Msg("Start AvailableManager!!!")
	loseConnHosts := alivemgr.StartAlvMgr(termChan)
	connLostTimeout := time.Duration(nOperationInventoryHostConnLost * float64(*inv_mgr_cli.InventoryTimeout))

	for {
		if hbk := <-loseConnHosts; !hbk.IsEmpty() {
			go func() {
				// Unix timestamps are always positive, so conversion from int64 to uint64 is safe
				now := time.Now().Unix()
				timestampConnLost := uint64(now)
				if err := backoff.Retry(func() error {
					childCtx, cancel := context.WithTimeout(ctx, connLostTimeout)
					defer cancel()
					err := inv_mgr_cli.SetHostAsConnectionLost(
						childCtx, invClientInstance, hbk.TenantID, hbk.ResourceID, timestampConnLost)
					if err != nil {
						zlog.InfraSec().Warn().Msgf(
							"Failed to update %s status as CONNECTION_LOST, retrying in the next backoff interval",
							hbk,
						)
					}
					return err
				}, backoff.WithMaxRetries(backoff.NewConstantBackOff(backoffInterval), backoffRetries)); err != nil {
					zlog.InfraSec().InfraError(
						"Failed to update %s status as CONNECTION_LOST, even after backoff",
						hbk,
					).Send()
				}
			}()
		}
	}
}

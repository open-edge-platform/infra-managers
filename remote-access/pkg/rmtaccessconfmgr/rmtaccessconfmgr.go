package rmtaccessconfmgr

import (
	"log"
	"net"
	"sync"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	pb "github.com/open-edge-platform/infra-managers/remote-access/pkg/api/remaccessmgr/v1"
	"github.com/open-edge-platform/infra-managers/remote-access/pkg/clients"
	"github.com/open-edge-platform/infra-managers/remote-access/pkg/config"
	"golang.org/x/net/context"

	//inv_client "github.com/open-edge-platform/infra-managers/remote-access/pkg/clients"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// TODO(max): remove global instances.
var (
	invClientInstance       inv_client.TenantAwareInventoryClient
	AllowHostDiscoveryValue = true // Default value in flag
)

const (
	backoffInterval = 5 * time.Second
	backoffRetries  = uint64(5)
	// SetHostConnectionLost does 2 operation with Inventory, plus we leave some slack time.
	nOperationInventoryHostConnLost = 2.05
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

var zlog = logging.GetLogger("RemoteAccessManager")

type Options struct {
	enableTracing bool
	enableMetrics bool
	metricsAddr   string
}

type Option func(*Options)

func EnableTracing(v bool) Option {
	return func(o *Options) { o.enableTracing = v }
}

func EnableMetrics(addr string) Option {
	return func(o *Options) {
		o.enableMetrics = true
		o.metricsAddr = addr
	}
}

func parseOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func StartGrpcSrv(
	ctx context.Context,
	wg *sync.WaitGroup,
	lis net.Listener,
	invClient *clients.RmtAccessInventoryClient,
	opts ...Option,
) error {
	options := parseOptions(opts...)

	var grpcOpts []grpc.ServerOption
	//var unaryInts []grpc.UnaryServerInterceptor

	if options.enableTracing {
		grpcOpts = tracing.EnableGrpcServerTracing(grpcOpts)
	}

	//srvMetrics := metrics.GetServerMetricsWithLatency()
	//if options.enableMetrics {
	//	unaryInts = append(unaryInts, srvMetrics.UnaryServerInterceptor())
	//}
	//
	//unaryInts = append(unaryInts,
	//	tenant.GetExtractTenantIDInterceptor(tenant.GetAgentsRole()),
	//)
	//
	//grpcOpts = append(grpcOpts,
	//	grpc.ChainUnaryInterceptor(unaryInts...),
	//)

	s := grpc.NewServer(grpcOpts...)

	pb.RegisterRemaccessmgrServiceServer(
		s,
		NewServer(invClient),
	)
	reflection.Register(s)

	//if options.enableMetrics {
	//	srvMetrics.InitializeMetrics(s)
	//	metrics.StartMetricsExporter(
	//		[]prometheus.Collector{srvMetrics},
	//		metrics.WithListenAddress(options.metricsAddr),
	//	)
	//}

	wg.Add(1)
	go func() {
		defer wg.Done()
		zlog.Info().Msgf("Serving RemoteAccessManager gRPC on %s", lis.Addr())
		log.Println("🧠 Manager gRPC listening on :50051")
		_ = s.Serve(lis)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()
		zlog.Info().Msg("Stopping RemoteAccessManager gRPC server")
		s.GracefulStop() // albo Stop() jeśli chcesz hard stop
	}()

	return nil
}

func StartInvGrpcCli(
	ctx context.Context,
	wg *sync.WaitGroup,
	conf config.RemoteAccessConfigMgrConfig,
) (inv_client.TenantAwareInventoryClient, chan *inv_client.WatchEvents, error) {

	resourceKinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF,
		// opcjonalnie, jeśli potrzebujesz doczytywać instance:
		// inv_v1.ResourceKind_RESOURCE_KIND_INSTANCE,
	}

	events := make(chan *inv_client.WatchEvents, eventsWatcherBufSize)

	cfg := inv_client.InventoryClientConfig{
		Name:                      "remaccessmgr",
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
		zlog.InfraSec().InfraErr(err).Msg("Cannot create new inventory client")
		return nil, nil, err
	}

	zlog.InfraSec().Info().Msg("Inventory client started")
	return gcli, events, nil
}

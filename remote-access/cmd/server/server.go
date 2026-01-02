package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	resourcev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/flags"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/metrics"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/oam"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/remote-access/internal/handlers"
	pb "github.com/open-edge-platform/infra-managers/remote-access/pkg/api/remaccessmgr/v1"
	"github.com/open-edge-platform/infra-managers/remote-access/pkg/clients"
	"github.com/open-edge-platform/infra-managers/remote-access/pkg/config"
	rmtAccessMgr "github.com/open-edge-platform/infra-managers/remote-access/pkg/rmtaccessconfmgr"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var zlog = logging.GetLogger("RemoteAccessConfigManagerMain")

var (
	wg        = sync.WaitGroup{}        // waitgroup so main will wait for all go routines to exit cleanly
	readyChan = make(chan bool, 1)      // channel to signal the readiness.
	termChan  = make(chan bool, 1)      // channel to signal termination of main process.
	sigChan   = make(chan os.Signal, 1) // channel to handle any interrupt signals
)

var (
	servaddr   = flag.String(flags.ServerAddress, "0.0.0.0:50005", flags.ServerAddressDescription)
	invsvcaddr = flag.String(client.InventoryAddress, "localhost:50051", client.InventoryAddressDescription)
	//invsvcaddr           = flag.String(client.InventoryAddress, "remaccessmgr-db:5432", client.InventoryAddressDescription)
	oamservaddr          = flag.String(oam.OamServerAddress, "", oam.OamServerAddressDescription)
	insecureGrpc         = flag.Bool(client.InsecureGrpc, true, client.InsecureGrpcDescription)
	caCertPath           = flag.String(client.CaCertPath, "", client.CaCertPathDescription)
	tlsCertPath          = flag.String(client.TLSCertPath, "", client.TLSCertPathDescription)
	tlsKeyPath           = flag.String(client.TLSKeyPath, "", client.TLSKeyPathDescription)
	enableTracing        = flag.Bool(tracing.EnableTracing, false, tracing.EnableTracingDescription)
	traceURL             = flag.String(tracing.TraceURL, "", tracing.TraceURLDescription)
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

type server struct {
	pb.UnimplementedRemaccessmgrServiceServer
	mu sync.RWMutex
	db map[string]*resourcev1.RemoteAccessConfiguration
}

func newServer() *server {
	return &server{db: make(map[string]*resourcev1.RemoteAccessConfiguration)}
}

//// CreateResourceAccess — called by API when new access is requested
//func (s *server) CreateResourceAccess(ctx context.Context, req *servicev1.CreateResourceAccessRequest) (*servicev1.CreateResourceAccessResponse, error) {
//	if req == nil || req.GetDeviceId() == "" {
//		return nil, status.Error(codes.InvalidArgument, "device_id is required")
//	}
//
//	if _, err := uuid.Parse(req.GetDeviceId()); err != nil {
//		return nil, status.Errorf(codes.InvalidArgument, "invalid device_id: %v", err)
//	}
//
//	now := time.Now().UTC()
//
//	reversePort := uint32(8000 + len(s.db))
//
//	ra := &resourcev1.ResourceAccess{
//		Uuid:          uuid.NewString(),
//		DeviceId:      req.GetDeviceId(),
//		ProxyEndpoint: "http://10.0.2.2:8080",
//
//		// Reverse bind port exposed by RAP:
//		ReverseBindPort: reversePort,
//
//		TargetHost: "127.0.0.1",
//		TargetPort: 22,
//		SshUser:    "ubuntu",
//
//		SessionToken: "admin:secret",
//
//		DesiredState:        resourcev1.RemoteAccessState_REMOTE_ACCESS_STATE_ENABLED,
//		ExpirationTimestamp: uint64(now.Add(time.Hour).Unix()),
//		TenantId:            "00000000-0000-0000-0000-000000000001",
//		CreatedAt:           now.Format(time.RFC3339),
//		UpdatedAt:           now.Format(time.RFC3339),
//	}
//
//	s.mu.Lock()
//	s.db[ra.DeviceId] = ra
//	s.mu.Unlock()
//	log.Printf("📦 Created ResourceAccess: device=%s, reverse_port=%d, uuid=%s", ra.DeviceId, ra.ReverseBindPort, ra.Uuid)
//
//	return &servicev1.CreateResourceAccessResponse{WsUrl: "ws://" + ra.TargetHost + ":50052/term", ExpiresAt: ra.ExpirationTimestamp}, nil
//}

// GetAgentSpec — used by agent to retrieve connection spec
func (s *server) GetAgentSpec(
	ctx context.Context,
	req *pb.GetRemoteAccessConfigByGuidRequest,
) (*pb.GetResourceAccessConfigResponse, error) {
	if req == nil || req.Uuid == "" {
		return nil, status.Error(codes.InvalidArgument, "guid is required")
	}

	s.mu.RLock()
	a, ok := s.db[req.GetUuid()]
	s.mu.RUnlock()
	if !ok {
		return nil, status.Error(codes.NotFound, "resource access not found")
	}

	spec := &pb.AgentRemoteAccessSpec{
		RemoteAccessProxyEndpoint: a.GetProxyHost(),
		SessionToken:              a.GetSessionToken(),
		ReverseBindPort:           a.GetLocalPort(),
		TargetHost:                a.GetTargetHost(),
		TargetPort:                a.GetTargetPort(),
		SshUser:                   a.GetUser(),
		ExpirationTimestamp:       a.GetExpirationTimestamp(),
		Uuid:                      a.GetInstance().GetHost().GetUuid(),
	}

	log.Printf("📦 Returned AgentSpec: reverse_port=%d, uuid=%s", spec.ReverseBindPort)

	return &pb.GetResourceAccessConfigResponse{Spec: spec}, nil
}

func main() {
	flag.Parse()

	conf := config.RemoteAccessConfigMgrConfig{
		EnableTracing:      *enableTracing,
		EnableMetrics:      *enableMetrics,
		TraceURL:           *traceURL,
		InventoryAddr:      *invsvcaddr,
		CACertPath:         *caCertPath,
		TLSKeyPath:         *tlsKeyPath,
		TLSCertPath:        *tlsCertPath,
		InsecureGRPC:       *insecureGrpc,
		EnableUUIDCache:    *invCacheUUIDEnable,
		UUIDCacheTTL:       *invCacheStaleTimeout,
		UUIDCacheTTLOffset: int(*invCacheStaleTimeoutOffset),
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wg := &sync.WaitGroup{}

	coreInv, events, err := rmtAccessMgr.StartInvGrpcCli(rootCtx, wg, conf)
	if err != nil {
		//
	}

	raInv, err := clients.NewRAInventoryClient(coreInv, events)
	if err != nil {
		zlog.Fatal().Err(err).Msg("failed to create RA inventory wrapper")
	}

	// 3) NB handler (reconcileAll + event loop + ticker fallback)
	nbh, err := handlers.NewNBHandler(raInv, conf.EnableTracing)
	if err != nil {
		zlog.Fatal().Err(err).Msg("failed to create NB handler")
	}
	if err := nbh.Start(); err != nil {
		zlog.Fatal().Err(err).Msg("failed to start NB handler")
	}
	// shutdown NB handler
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-rootCtx.Done()
		nbh.Stop()
	}()

	// 4) Southbound gRPC (agent polling)
	//lis, err := net.Listen("tcp", ":50051")
	lis, err := net.Listen("tcp", *servaddr) // np. 0.0.0.0:50001
	if err != nil {
		//zlog.Fatal().Err(err).Msgf("failed to listen on %s", conf.ServerAddr)
		zlog.Fatal().Err(err).Msgf("failed to listen on %s", "50051")
	}
	if err := rmtAccessMgr.StartGrpcSrv(rootCtx, wg, lis, raInv,
		rmtAccessMgr.EnableTracing(*enableTracing),
		//rmtAccessMgr.EnableAuth(*enableAuth),
		//rmtAccessMgr.WithRbacRulesPath(*rbacRules),
		//rmtAccessMgr.EnableMetrics(*enableMetrics),
		//rmtAccessMgr.WithMetricsAddress(*metricsAddress),
	); err != nil {
		zlog.Fatal().Err(err).Msg("failed to start grpc server")
	}

	<-rootCtx.Done()
	zlog.Info().Msg("Shutdown signal received, waiting for goroutines...")
	wg.Wait()
	zlog.Info().Msg("RemoteAccessManager stopped")
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package maintmgr_test

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/maintmgr"
)

var (
	MaintManagerTestClient     pb.MaintmgrServiceClient
	MaintManagerTestClientConn *grpc.ClientConn
	zlog                       = logging.GetLogger("Maint-Manager-Testing")
	termChan                   = make(chan bool)
	wg                         = sync.WaitGroup{}
	BufconnLis                 *bufconn.Listener
	rulesDir                   = "../../rego/authz.rego"
)

// Internal parameters for bufconn testing.
const (
	bufferSize = util.Megabyte
)

// Starts all Inventory and Maintenance Manager requirements to test maintenance manager southbound client.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd))

	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	maintenanceManagerInvClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	scheduleCache := schedule_cache.NewScheduleCacheClient(maintenanceManagerInvClient)
	hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
	if err != nil {
		panic(err)
	}

	StartMaintenanceManagerTestingEnvironment(maintenanceManagerInvClient, hScheduleCache)

	run := m.Run() // run all tests

	StopMaintenanceManagerTestingEnvironment()
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

// Helper function to start all requirements to test southbound maintenance manager client API.
func StartMaintenanceManagerTestingEnvironment(invClient client.TenantAwareInventoryClient,
	cacheClient *schedule_cache.HScheduleCacheClient,
) {
	// Boostrap c/s connectivity using bufconn
	createBufConn()
	// Sets gcli (inventory client) in maintmgr

	cli := invclient.NewInvGrpcClient(invClient, cacheClient)
	maintmgr.SetInvGrpcCli(cli)

	// Bootstrap server
	createMaintenanceManagerServer()
	// Bootstrap the clients
	if err := createMaintenanceManagerClient(); err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Maintenance Manager client")
	}
}

// This function is used to stop the maintenance manager test environment.
func StopMaintenanceManagerTestingEnvironment() {
	if err := MaintManagerTestClientConn.Close(); err != nil {
		zlog.Warn().Err(err).Msg("Failed to close maintenance manager test client connection")
	}
	close(termChan) // stop the maintenance manager server after tests
	wg.Wait()       // wait until servers terminate
}

// Helper function to create a southbound gRPC server for maintenance manager.
func createMaintenanceManagerServer() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		close(termChan)
	}()
	go func() {
		maintmgr.StartGrpcSrv(BufconnLis, nil, termChan, &wg,
			maintmgr.EnableSanitizeGrpcErr(true),
			maintmgr.EnableAuth(true),
			maintmgr.EnableMetrics(true),
			maintmgr.EnableTracing(true),
			maintmgr.WithRbacRulesPath(rulesDir),
		)
	}()
	zlog.Info().Msgf("Started Maintenance Manager server...\n")
}

// Create a maintenance manager southbound gRPC client.
//
//nolint:staticcheck // Use deprecated functions.
func createMaintenanceManagerClient() error {
	opts := make([]grpc.DialOption, 0, 3)
	opts = append(opts,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return BufconnLis.Dial() }),
		grpc.WithBlock(),
	)
	dialOpt := grpc.WithTransportCredentials(insecure.NewCredentials())
	opts = append(opts, dialOpt)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", opts...)
	if err != nil {
		return err
	}
	maintManagerClient := pb.NewMaintmgrServiceClient(conn)

	MaintManagerTestClientConn = conn
	MaintManagerTestClient = maintManagerClient

	return nil
}

// Create the bufconn listener used for the c/s maintenance manager communication.
func createBufConn() {
	// https://pkg.go.dev/google.golang.org/grpc/test/bufconn#Listener
	buffer := bufferSize
	BufconnLis = bufconn.Listen(buffer)
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package attestmgr_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	attestmgr_sb "github.com/open-edge-platform/infra-managers/attestationstatus/pkg/api/attestmgr/v1"
	"github.com/open-edge-platform/infra-managers/attestationstatus/pkg/attestmgr"
	"github.com/open-edge-platform/infra-managers/attestationstatus/pkg/invclient"
)

var (
	AttestMgrTestClient     attestmgr_sb.AttestationStatusMgrServiceClient
	AttestMgrTestClientConn *grpc.ClientConn
	zlog                    = logging.GetLogger("AttestationManagerTesting")
	termChan                = make(chan bool)
	wg                      = sync.WaitGroup{}
	BufconnLis              *bufconn.Listener
	rbacRules               = "../../rego/authz.rego"
)

const (
	bufferSize     = util.Megabyte
	defaultTimeout = 120 * time.Second
)

func createBufConn() {
	// https://pkg.go.dev/google.golang.org/grpc/test/bufconn#Listener
	buffer := bufferSize
	BufconnLis = bufconn.Listen(buffer)
}

// Helper function to create a southbound gRPC server for Attestation Status manager.
func createAttestMgrServer() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		close(termChan)
	}()
	go func() {
		attestmgr.StartSBGrpcSrv(BufconnLis, nil, termChan, &wg,
			attestmgr.EnableAuth(true),
			attestmgr.EnableTracing(true),
			attestmgr.WithRbacRulesPath(rbacRules),
		)
	}()
	zlog.Info().Msgf("Started Attestation Status Manager server...\n")
}

//nolint:staticcheck // currently using deprecated functions.
func createAttestMgrClient(
	target string,
	bufconnLis *bufconn.Listener,
) (
	attestmgr_sb.AttestationStatusMgrServiceClient,
	*grpc.ClientConn,
	error,
) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if bufconnLis != nil {
		opts = append(opts,
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufconnLis.Dial() }))
	}

	dialOpt := grpc.WithTransportCredentials(insecure.NewCredentials())
	opts = append(opts, dialOpt)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, nil, err
	}
	attestMgrClient := attestmgr_sb.NewAttestationStatusMgrServiceClient(conn)

	return attestMgrClient, conn, nil
}

// Helper function to start all requirements to test southbound attestation status manager client API.
func StartAttestMgrTestingEnvironment() {
	// create inventory test client
	invTestClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	// Boostrap c/s connectivity using bufconn
	createBufConn()
	// set up inventory connection
	invclient.SetInventoryClient(invTestClient)
	// Bootstrap Southbound gRPC server
	createAttestMgrServer()
	// Bootstrap the clients
	cli, conn, err := createAttestMgrClient("", BufconnLis)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Attestation Status Manager client")
	}
	AttestMgrTestClient = cli
	AttestMgrTestClientConn = conn
}

func StopAttestMgrTestingEnvironment() {
	if err := AttestMgrTestClientConn.Close(); err != nil {
		zlog.Warn().Err(err).Msg("Failed to close AttestMgrTestClientConn")
	}
	close(termChan)
	wg.Wait()
}

// Starts all Inventory and Attestation Manager requirements to test southbound client.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd))

	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	StartAttestMgrTestingEnvironment()

	run := m.Run() // run all tests

	StopAttestMgrTestingEnvironment()
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

func GetInstanceByResourceID(tb testing.TB, instResID string) *computev1.InstanceResource {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb)
	defer cancel()

	instFilter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Instance{}},
		Filter:   fmt.Sprintf("%s = %q", computev1.InstanceResourceFieldResourceId, instResID),
	}

	listres, err := inv_testing.TestClients[inv_testing.RMClient].List(ctx, instFilter)
	require.NoError(tb, err, "Get Instance failed")

	resources := make([]*computev1.InstanceResource, 0, len(listres.Resources))
	for _, r := range listres.Resources {
		resources = append(resources, r.GetResource().GetInstance())
	}

	instRes := resources[0]
	return instRes
}

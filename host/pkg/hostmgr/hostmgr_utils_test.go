// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	"github.com/open-edge-platform/infra-managers/host/pkg/hostmgr"
	hutils "github.com/open-edge-platform/infra-managers/host/pkg/utils"
	test_utils "github.com/open-edge-platform/infra-managers/host/test/utils"
)

const (
	TooLongString = "4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C" +
		"4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C" +
		"4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C" +
		"4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C4050ED108F5C"
)

var (
	HostManagerTestClient     pb.HostmgrClient
	HostManagerTestClientConn *grpc.ClientConn
	zlog                      = logging.GetLogger("Host-Manager-Testing4")
	termChan                  = make(chan bool)
	wg                        = sync.WaitGroup{}
	BufconnLis                *bufconn.Listener
	rbacRules                 = "../../rego/authz.rego"
)

var (
	hostGUID                = "BFD3B398-9A4B-480D-AB53-4050ED108F5C"
	hostSN                  = "87654321"
	hostStatusDetails       = "Details"
	hostStatusHumanReadable = "HumanReadableStatus"
	hostCPUCores            = uint32(12)
)

// Internal parameters for bufconn testing.
const bufferSize = util.Megabyte

const DefaultTenantID = "10000000-0000-0000-0000-000000000000"

// ######################################
// ############ Requirements ############
// ######################################

// Create the bufconn listener used for the c/s host manager communication.
func createBufConn() {
	// https://pkg.go.dev/google.golang.org/grpc/test/bufconn#Listener
	buffer := bufferSize
	BufconnLis = bufconn.Listen(buffer)
}

// Helper function to create a southbound gRPC server for host manager.
func createHostManagerServer() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		close(termChan)
	}()
	go func() {
		hostmgr.StartGrpcSrv(BufconnLis, nil, termChan, &wg,
			hostmgr.EnableAuth(true),
			hostmgr.EnableMetrics(true),
			hostmgr.WithMetricsAddress(":9081"),
			hostmgr.EnableTracing(true),
			hostmgr.WithRbacRulesPath(rbacRules),
		)
	}()
	zlog.Info().Msgf("Started Host Manager server...\n")
}

// Helper function to start all requirements to test southbound host manager client API.
func StartHostManagerTestingEnvironment() {
	invClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	// Boostrap c/s connectivity using bufconn
	createBufConn()
	// Sets gcli (inventory client) in hostmgr
	hostmgr.SetInvGrpcCli(invClient)
	// Not starting NB Handler, this is used in SB tests
	// Bootstrap server
	createHostManagerServer()
	// Bootstrap the clients
	cli, conn, err := test_utils.CreateHostManagerClient("", BufconnLis)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Host Manager client")
	}
	HostManagerTestClient = cli
	HostManagerTestClientConn = conn
}

// This function is used to stop the host manager test environment.
func StopHostManagerTestingEnvironment() {
	if err := HostManagerTestClientConn.Close(); err != nil { // close host manager test client
		zlog.Warn().Err(err).Msg("Failed to close host manager test client")
	}
	close(termChan) // stop the host manager server after tests
	wg.Wait()       // wait until servers terminate
}

// Starts all Inventory and Host Manager requirements to test host manager southbound client.
func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd))

	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	StartHostManagerTestingEnvironment()

	run := m.Run() // run all tests

	StopHostManagerTestingEnvironment()
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

// ####################################
// ######### Helper Functions #########
// ####################################

func CleanHost(tb testing.TB, hostUUID string) {
	tb.Helper()

	host := GetHostbyUUID(tb, hostUUID)
	inv_testing.HardDeleteHost(tb, host.GetResourceId())
}

func GetHostbyUUID(tb testing.TB, hostUUID string) *computev1.HostResource {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb)
	defer cancel()

	hostFilter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Host{}},
		Filter:   fmt.Sprintf("%s = %q", computev1.HostResourceFieldUuid, hostUUID),
	}

	listres, err := inv_testing.TestClients[inv_testing.RMClient].List(ctx, hostFilter)
	require.NoError(tb, err, "Get Host failed")

	resources := make([]*computev1.HostResource, 0, len(listres.Resources))
	for _, r := range listres.Resources {
		resources = append(resources, r.GetResource().GetHost())
	}

	host := resources[0]
	return host
}

func GetInstanceByHostUUID(tb testing.TB, hostUUID string) *computev1.InstanceResource {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb)
	defer cancel()

	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Instance{}},
		Filter:   fmt.Sprintf("%s.%s = %q", computev1.InstanceResourceEdgeHost, computev1.HostResourceFieldUuid, hostUUID),
	}

	listres, err := inv_testing.TestClients[inv_testing.RMClient].List(ctx, filter)
	require.NoError(tb, err)

	resources := make([]*computev1.InstanceResource, 0, len(listres.Resources))
	for _, r := range listres.Resources {
		resources = append(resources, r.GetResource().GetInstance())
	}

	inst := resources[0]
	return inst
}

type hasDeviceName interface {
	GetDeviceName() string
}

func OrderByDeviceName[T hasDeviceName](slice []T) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].GetDeviceName() > slice[j].GetDeviceName()
	})
}

func ConvertSystemNetworkIntoHostNics(tb testing.TB, networks []*pb.SystemNetwork,
	host *computev1.HostResource,
) []*computev1.HostnicResource {
	tb.Helper()

	hostNics := make([]*computev1.HostnicResource, 0, len(networks))
	for _, network := range networks {
		hostNic, err := hutils.PopulateHostnicWithNetworkInfo(network, host)
		require.NoError(tb, err, "Unable to convert hostNic")
		hostNics = append(hostNics, hostNic)
	}
	return hostNics
}

// Hard deletes the network by removing all SystemNetwork objects from the HwInfo of the SystemInfo.
// The SB does not support incremental updates - original systemInfo is requested to avoid side effects.
func HardDeleteHostnicResourcesWithUpdateHostSystemInfo(
	tb testing.TB, tenantID string, systemInfo *pb.UpdateHostSystemInfoByGUIDRequest,
) {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb, tenantID)
	defer cancel()

	systemInfo.SystemInfo.HwInfo.Network = []*pb.SystemNetwork{}
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo)
	require.NoError(tb, err, "UpdateHostSystemInfoByGuid() failed")
}

func GetIPbyNicID(tb testing.TB, nicID string) []*network_v1.IPAddressResource {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Ipaddress{}},
		Filter: fmt.Sprintf("%s.%s = %q", network_v1.IPAddressResourceEdgeNic,
			computev1.HostnicResourceFieldResourceId, nicID),
	}

	listres, err := inv_testing.TestClients[inv_testing.RMClient].ListAll(ctx, filter)
	if err != nil && errors.IsNotFound(err) {
		return []*network_v1.IPAddressResource{}
	}
	require.NoError(tb, err, "ListAll failed")

	ips, err := util.GetSpecificResourceList[*network_v1.IPAddressResource](listres)
	require.NoError(tb, err, "GetSpecificResourceList failed")

	return ips
}

type hasIPAddress interface {
	GetAddress() string
}

func OrderByIPAddress[T hasIPAddress](slice []T) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].GetAddress() > slice[j].GetAddress()
	})
}

func ConvertSystemNetworkIntoHostIPs(
	tb testing.TB,
	networks []*pb.IPAddress,
	hostNic *computev1.HostnicResource,
) []*network_v1.IPAddressResource {
	tb.Helper()

	hostIPs := make([]*network_v1.IPAddressResource, 0, len(networks))
	for _, network := range networks {
		hostIP, err := hutils.PopulateIPAddressWithIPAddressInfo(network, hostNic)
		require.NoError(tb, err, "Unable to convert hostIP")
		hostIPs = append(hostIPs, hostIP)
	}
	return hostIPs
}

// ConvertSystemDiskIntoHostStorages is an helper function to convert disks in host storage resources.
func ConvertSystemDiskIntoHostStorages(tb testing.TB, storage *pb.Storage,
	host *computev1.HostResource,
) []*computev1.HoststorageResource {
	tb.Helper()

	hostStorages := make([]*computev1.HoststorageResource, 0, len(storage.Disk))
	for _, disk := range storage.Disk {
		hostStorage, err := hutils.PopulateHoststorageWithDiskInfo(disk, host)
		require.NoError(tb, err, "Unable to convert hostStorage")
		hostStorages = append(hostStorages, hostStorage)
	}
	return hostStorages
}

// HardDeleteHoststoragesWithUpdateHostSystemInfo deletes the storage by removing all SystemDisk objects
// from the HwInfo of the SystemInfo. The SB does not support incremental updates - original systemInfo
// is requested to avoid side effects.
func HardDeleteHoststoragesWithUpdateHostSystemInfo(
	tb testing.TB, tenantID string, systemInfo *pb.UpdateHostSystemInfoByGUIDRequest,
) {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb, tenantID)
	defer cancel()

	systemInfo.SystemInfo.HwInfo.Storage = &pb.Storage{}
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo)
	require.NoError(tb, err, "UpdateHostSystemInfoByGuid() failed")
}

type hasBus interface {
	GetBus() uint32
}

func OrderByBus[T hasBus](slice []T) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].GetBus() > slice[j].GetBus()
	})
}

// ConvertSystemUSBIntoHostUsbs is an helper function to convert usbs in host usb resources.
func ConvertSystemUSBIntoHostUsbs(tb testing.TB, usbs []*pb.SystemUSB,
	host *computev1.HostResource,
) []*computev1.HostusbResource {
	tb.Helper()

	hostUsbs := make([]*computev1.HostusbResource, 0, len(usbs))
	for _, usb := range usbs {
		hostUsb, err := hutils.PopulateHostusbWithUsbInfo(usb, host)
		require.NoError(tb, err, "Unable to convert hostUsb")
		hostUsbs = append(hostUsbs, hostUsb)
	}
	return hostUsbs
}

func ConvertSystemGPUIntoHostGpus(tb testing.TB, gpus []*pb.SystemGPU,
	host *computev1.HostResource,
) []*computev1.HostgpuResource {
	tb.Helper()

	hostGpus := make([]*computev1.HostgpuResource, 0, len(gpus))
	for _, gpu := range gpus {
		hostGpu, err := hutils.PopulateHostgpuWithGpuInfo(gpu, host)
		require.NoError(tb, err, "Unable to convert hostGpu")
		hostGpus = append(hostGpus, hostGpu)
	}
	return hostGpus
}

// HardDeleteHoststoragesWithUpdateHostSystemInfo deletes the storage by removing all SystemDisk objects
// from the HwInfo of the SystemInfo. The SB does not support incremental updates - original systemInfo
// is requested to avoid side effects.
func HardDeleteHostgpusWithUpdateHostSystemInfo(
	tb testing.TB, tenantID string, systemInfo *pb.UpdateHostSystemInfoByGUIDRequest,
) {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb, tenantID)
	defer cancel()

	systemInfo.SystemInfo.HwInfo.Gpu = []*pb.SystemGPU{}
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo)
	require.NoError(tb, err, "UpdateHostSystemInfoByGuid() failed")
}

// HardDeleteHostnicResourcesWithUpdateHostSystemInfo deletes the usbs by removing all SystemUSB objects
// from the HwInfo of the SystemInfo. The SB does not support incremental updates - original systemInfo
// is requested to avoid side effects.
func HardDeleteHostusbResourcesWithUpdateHostSystemInfo(
	tb testing.TB, tenantID string, systemInfo *pb.UpdateHostSystemInfoByGUIDRequest,
) {
	tb.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(tb, tenantID)
	defer cancel()

	systemInfo.SystemInfo.HwInfo.Usb = []*pb.SystemUSB{}
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo)
	require.NoError(tb, err, "UpdateHostSystemInfoByGuid() failed")
}

func assertSameResources[T proto.Message](
	t *testing.T,
	expectedResources, actualResources []T,
	orderFunc func([]T),
	compareFunc func(T, T) (bool, string),
) {
	t.Helper()
	require.Equal(t, len(expectedResources), len(actualResources))

	orderFunc(expectedResources)
	orderFunc(actualResources)
	for i, expected := range expectedResources {
		actual := actualResources[i]
		if eq, diff := compareFunc(expected, actual); !eq {
			t.Errorf("Resource data not equal: %v", diff)
		}
	}
}

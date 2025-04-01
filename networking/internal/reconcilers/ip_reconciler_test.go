// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	net_testing "github.com/open-edge-platform/infra-managers/networking/internal/testing"
)

// From HRM inv_client UpdateIPAddress function.
var fmHrmIPAddrUpdate = &fieldmaskpb.FieldMask{Paths: []string{
	network_v1.IPAddressResourceFieldAddress,
	network_v1.IPAddressResourceFieldConfigMethod,
	network_v1.IPAddressResourceEdgeNic,
	network_v1.IPAddressResourceFieldStatus,
	network_v1.IPAddressResourceFieldStatusDetail,
	network_v1.IPAddressResourceFieldCurrentState,
}}

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd))
	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	run := m.Run() // run all tests
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

// Verify IPAddress reconciliation.
func TestIPAllocation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateIPController(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	hostT1 := dao.CreateHost(t, net_testing.Tenant1)
	hostNicT1 := dao.CreateHostNic(t, net_testing.Tenant1, hostT1)
	resID1T1 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNicT1, true).GetResourceId()

	hostT2 := dao.CreateHost(t, net_testing.Tenant2)
	hostNicT2 := dao.CreateHostNic(t, net_testing.Tenant2, hostT2)
	resID1T2 := dao.CreateIPAddress(t, net_testing.Tenant2, hostNicT2, true).GetResourceId()

	// Let's craft an unsupported address
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: &network_v1.IPAddressResource{
				TenantId:     net_testing.Tenant1,
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				Nic:          hostNicT1,
			},
		},
	}
	crespT1, err := rmClient.Create(ctx, net_testing.Tenant1, res)
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(crespT1)
	require.NoError(t, err)
	t.Cleanup(func() {
		dao.HardDeleteIPAddress(t, net_testing.Tenant1, rID)
	})
	resID2T1 := rID

	// Reconcile the addresses as they come
	net_testing.AssertReconcile(t, 3)

	// Verify reconciliation
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1T1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)
	net_testing.AssertIPAddress(t, net_testing.Tenant2, resID1T2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2T1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_ASSIGNMENT_ERROR)
}

// Verify IP addresses in Site.
func TestIPDuplication1(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	// Take into account the whole process and the sleeps.
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateIPController(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	site := dao.CreateSite(t, net_testing.Tenant1)
	host1 := dao.CreateHost(t, net_testing.Tenant1, inv_testing.HostSite(site))
	hostNic1 := dao.CreateHostNic(t, net_testing.Tenant1, host1)
	resID1 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic1, true).GetResourceId()
	// Create T2 objects to ensure isolation is guaranteed.
	siteT2 := dao.CreateSite(t, net_testing.Tenant2)
	host1T2 := dao.CreateHost(t, net_testing.Tenant2, inv_testing.HostSite(siteT2))
	hostNic1T2 := dao.CreateHostNic(t, net_testing.Tenant2, host1T2)
	dao.CreateIPAddress(t, net_testing.Tenant2, hostNic1T2, true).GetResourceId()

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: &network_v1.IPAddressResource{
				TenantId:     net_testing.Tenant1,
				Address:      "192.168.0.1/24",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Nic:          hostNic1,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
		},
	}
	cresp, err := rmClient.Create(ctx, net_testing.Tenant1, res)
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(cresp)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.Error(t, dao.HardDeleteIPAddressAndReturnError(t, net_testing.Tenant1, rID))
	})
	resID2 := rID

	host2 := dao.CreateHost(t, net_testing.Tenant1)
	hostNic2 := dao.CreateHostNic(t, net_testing.Tenant1, host2)
	resID3 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic2, true).GetResourceId()

	// Reconcile the addresses as they come
	net_testing.AssertReconcile(t, 4)

	// Verify reconciliation
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	// Emulate HRM periodic update
	require.NoErrorf(t, err, "Failed to create FieldMask from message")
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	net_testing.AssertReconcile(t, 1)

	// No changes
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	// Fix the duplication
	dao.HardDeleteIPAddress(t, net_testing.Tenant1, resID2)
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	net_testing.AssertReconcile(t, 1)

	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	_, err = rmClient.Get(ctx, net_testing.Tenant1, resID2)
	require.Error(t, err)
}

// Verify IP addresses without Site.
func TestIPDuplication2(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	// Take into account the whole process and the sleeps.
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateIPController(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	host1 := dao.CreateHost(t, net_testing.Tenant1)
	hostNic1 := dao.CreateHostNic(t, net_testing.Tenant1, host1)
	resID1 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic1, true).GetResourceId()
	// Create resource for net_testing.Tenant2 to ensure isolation.
	host1T2 := dao.CreateHost(t, net_testing.Tenant2)
	hostNic1T2 := dao.CreateHostNic(t, net_testing.Tenant2, host1T2)
	dao.CreateIPAddress(t, net_testing.Tenant2, hostNic1T2, true).GetResourceId()

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: &network_v1.IPAddressResource{
				TenantId:     net_testing.Tenant1,
				Address:      "192.168.0.1/24",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Nic:          hostNic1,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
		},
	}
	cresp, err := rmClient.Create(ctx, net_testing.Tenant1, res)
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(cresp)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.Error(t, dao.HardDeleteIPAddressAndReturnError(t, net_testing.Tenant1, rID))
	})
	resID2 := rID

	site := dao.CreateSite(t, net_testing.Tenant1)
	host2 := dao.CreateHost(t, net_testing.Tenant1, inv_testing.HostSite(site))
	hostNic2 := dao.CreateHostNic(t, net_testing.Tenant1, host2)
	resID3 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic2, true).GetResourceId()

	// Reconcile the addresses as they come
	net_testing.AssertReconcile(t, 4)

	// Verify reconciliation
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	// Emulate HRM periodic update
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	net_testing.AssertReconcile(t, 1)

	// No changes
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	// Fix the duplication
	dao.HardDeleteIPAddress(t, net_testing.Tenant1, resID2)
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	net_testing.AssertReconcile(t, 1)

	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID3, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	_, err = rmClient.Get(ctx, net_testing.Tenant1, resID2)
	require.Error(t, err)
}

// Verify race condition scenario.
func TestIPDuplicationNotFound(t *testing.T) {
	// Take into account the whole process and the sleeps.
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateIPController(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	site := inv_testing.CreateSite(t, nil, nil)
	host1 := inv_testing.CreateHost(t, site, nil)
	hostNic1 := inv_testing.CreateHostNic(t, host1)
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: &network_v1.IPAddressResource{
				Address:      "192.168.0.1/24",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Nic:          hostNic1,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
		},
	}
	cresp, err := inv_testing.TestClients[inv_testing.RMClient].Create(ctx, res)
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(cresp)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.Error(t, inv_testing.HardDeleteIPAddressAndReturnError(t, rID))
	})
	resID1 := rID

	// Emulate an immediate delete
	inv_testing.HardDeleteIPAddress(t, resID1)

	// Reconcile the addresses as they come
	net_testing.AssertReconcile(t, 1)

	// Give the time for the scheduling to happen
	time.Sleep(1 * time.Second)

	// No friends
	_, err = inv_testing.TestClients[inv_testing.RMClient].Get(ctx, resID1)
	require.Error(t, err)
}

// Verify update scenarios.
func TestIPDuplicationUpdates(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	// Take into account the whole process and the sleeps.
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateIPController(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	site := dao.CreateSite(t, net_testing.Tenant1)
	host1 := dao.CreateHost(t, net_testing.Tenant1, inv_testing.HostSite(site))
	hostNic1 := dao.CreateHostNic(t, net_testing.Tenant1, host1)
	resID1 := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic1, true).GetResourceId()
	siteT2 := dao.CreateSite(t, net_testing.Tenant2)
	host1T2 := dao.CreateHost(t, net_testing.Tenant2, inv_testing.HostSite(siteT2))
	hostNic1T2 := dao.CreateHostNic(t, net_testing.Tenant2, host1T2)
	resID1T2 := dao.CreateIPAddress(t, net_testing.Tenant2, hostNic1T2, true).GetResourceId()
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: &network_v1.IPAddressResource{
				TenantId:     net_testing.Tenant1,
				Address:      "192.168.0.1/24",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Nic:          hostNic1,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
		},
	}
	cresp, err := rmClient.Create(ctx, net_testing.Tenant1, res)
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(cresp)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.Error(t, dao.HardDeleteIPAddressAndReturnError(t, net_testing.Tenant1, rID))
	})
	resID2 := rID

	// Reconcile the addresses as they come
	net_testing.AssertReconcile(t, 3)

	// Verify reconciliation
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)

	// Emulate another update
	res.GetIpaddress().CurrentState = network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR
	res.GetIpaddress().Status = network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR
	res.GetIpaddress().StatusDetail = "IPAddress duplication is unsupported"
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	res.GetIpaddress().TenantId = ""
	res.GetIpaddress().CurrentState = network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR
	res.GetIpaddress().Status = network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR
	res.GetIpaddress().StatusDetail = "IPAddress duplication is unsupported"
	res.GetIpaddress().Nic = hostNic1T2
	_, err = rmClient.Update(ctx, net_testing.Tenant2, resID1T2, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)
	res.GetIpaddress().Nic = hostNic1

	net_testing.AssertReconcile(t, 2)

	// No changes
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID2, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR)

	// Fix the duplication
	dao.HardDeleteIPAddress(t, net_testing.Tenant1, resID2)
	// This update will not fix the resource
	_, err = rmClient.Update(ctx, net_testing.Tenant1, resID1, fmHrmIPAddrUpdate, res)
	require.NoError(t, err)

	net_testing.AssertReconcile(t, 1)
	// This update will fix the resource
	time.Sleep(1 * time.Second)
	net_testing.AssertIPAddress(t, net_testing.Tenant1, resID1, network_v1.IPAddressState_IP_ADDRESS_STATE_UNSPECIFIED,
		network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED, network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED)

	_, err = rmClient.Get(ctx, net_testing.Tenant1, resID2)
	require.Error(t, err)
}

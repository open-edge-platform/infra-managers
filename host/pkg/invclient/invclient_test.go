// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package invclient_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inventoryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	networkv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/host/pkg/invclient"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
	util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

const (
	tenant1 = "11111111-1111-1111-1111-111111111111"
	tenant2 = "22222222-2222-2222-2222-222222222222"
)

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

func TestInvClient_GetHostResourceByGuid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error - empty GUID
	h, err := invclient.GetHostResourceByGUID(ctx, client, tenant1, "")
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, grpc_status.Convert(err).Code())
	require.Nil(t, h)

	// Error - no host
	noHost, err := invclient.GetHostResourceByGUID(ctx, client, tenant1, "foobar")
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpc_status.Code(err))
	assert.Nil(t, noHost)

	// OK - gets host
	hostT1 := dao.CreateHost(t, tenant1)
	hostT2 := dao.CreateHost(t, tenant2)
	getHost, err := invclient.GetHostResourceByGUID(ctx, client, tenant1, hostT1.GetUuid())
	require.NoError(t, err)
	require.NotNil(t, getHost)
	assert.Equal(t, hostT1.GetUuid(), getHost.GetUuid())
	getHost, err = invclient.GetHostResourceByGUID(ctx, client, tenant2, hostT2.GetUuid())
	require.NoError(t, err)
	require.NotNil(t, getHost)
	assert.Equal(t, hostT2.GetUuid(), getHost.GetUuid())
	// Tenant Isolation
	_, err = invclient.GetHostResourceByGUID(ctx, client, tenant1, hostT2.GetUuid())
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, grpc_status.Code(err))

	// Error - not found host
	emptyHost, err2 := invclient.GetHostResourceByGUID(ctx, client, tenant1, "30b7deca-72c9-4cab-93d7-69956064ea15")
	require.Error(t, err2)
	assert.Equal(t, codes.NotFound, grpc_status.Code(err2))
	require.Nil(t, emptyHost)
}

func TestInvClient_GetHostResourceByResourceId(t *testing.T) {
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error - no host
	noHost, err := invclient.GetHostResourceByResourceID(context.Background(), client, tenant1, "host-12345678")
	require.Error(t, err)
	require.Nil(t, noHost)

	// OK - gets host
	host := dao.CreateHost(t, tenant1)
	getHost, err := invclient.GetHostResourceByResourceID(context.Background(), client, tenant1, host.GetResourceId())
	require.NoError(t, err)
	require.NotNil(t, getHost)
	assert.Equal(t, host.GetResourceId(), getHost.GetResourceId())
}

func TestInvClient_CreateHostusb(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	id, err := invclient.CreateHostusb(ctx, client, tenant1, &computev1.HostusbResource{})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpc_status.Code(err))
	assert.Empty(t, id)

	// Create with resourceId is not allowed
	host := dao.CreateHost(t, tenant1)
	usb := &computev1.HostusbResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
		ResourceId: "hostusb-12345678",
	}
	id, err = invclient.CreateHostusb(ctx, client, tenant1, usb)
	require.Error(t, err)
	require.Equal(t, "", id)

	// OK
	usb = &computev1.HostusbResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
	}
	id, err = invclient.CreateHostusb(ctx, client, tenant1, usb)
	require.NoError(t, err)
	require.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })
}

func TestInvClient_UpdateHostUsbResource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	err := invclient.UpdateHostusb(ctx, client, tenant1, &computev1.HostusbResource{})
	require.Error(t, err)

	// OK - create usb and updates description.
	host := dao.CreateHost(t, tenant1)
	usb := &computev1.HostusbResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
	}
	id, err := invclient.CreateHostusb(ctx, client, tenant1, usb)
	require.NoError(t, err)
	require.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })

	usb.DeviceName = "foobar"
	usb.ResourceId = id
	err = invclient.UpdateHostusb(ctx, client, tenant1, usb)
	require.NoError(t, err)
}

func TestInvClient_DeleteHostUsbResource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Not found, does not return error
	err := invclient.DeleteHostusb(ctx, client, tenant1, "hostusb-12345678")
	require.NoError(t, err)

	// OK - create usb and delete.
	host := dao.CreateHost(t, tenant1)
	usb := &computev1.HostusbResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
	}
	id, err := invclient.CreateHostusb(ctx, client, tenant1, usb)
	require.NoError(t, err)
	require.NotEqual(t, "", id)

	err = invclient.DeleteHostusb(ctx, client, tenant1, id)
	require.NoError(t, err)
}

func TestInvClient_CreateHoststorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	id, err := invclient.CreateHoststorage(ctx, client, tenant1, &computev1.HoststorageResource{})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpc_status.Code(err))
	assert.Empty(t, id)

	host := dao.CreateHost(t, tenant1)

	// Create with resourceId is not allowed
	storage := &computev1.HoststorageResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
		ResourceId: "hoststorage-12345678",
	}
	id, err = invclient.CreateHoststorage(ctx, client, tenant1, storage)
	require.Error(t, err)
	require.Equal(t, "", id)

	// OK
	storage = &computev1.HoststorageResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
		Wwid:       "0x50026b7684e50ff8",
	}
	id, err = invclient.CreateHoststorage(ctx, client, tenant1, storage)
	require.NoError(t, err)
	require.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })
}

func TestInvClient_UpdateHoststorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	err := invclient.UpdateHoststorage(ctx, client, tenant1, &computev1.HoststorageResource{})
	require.Error(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	storage := &computev1.HoststorageResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
		Wwid:       "eui.01000000010000005cd2e43cf16e5451",
	}
	id, err := invclient.CreateHoststorage(ctx, client, tenant1, storage)
	require.NoError(t, err)
	require.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })

	storage.DeviceName = "foobar"
	storage.Wwid = "0x50026b7684e50ff8" // update wwid should be allowed
	storage.ResourceId = id
	err = invclient.UpdateHoststorage(ctx, client, tenant1, storage)
	require.NoError(t, err)
}

func TestInvClient_DeleteHoststorage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Not found, does not return error
	err := invclient.DeleteHoststorage(ctx, client, tenant1, "hoststorage-12345678")
	require.NoError(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	storage := &computev1.HoststorageResource{
		TenantId:   tenant1,
		Kind:       "",
		DeviceName: "for unit testing purposes",
		Host:       host,
	}
	id, err := invclient.CreateHoststorage(ctx, client, tenant1, storage)
	require.NoError(t, err)
	require.NotEqual(t, "", id)
	err = invclient.DeleteHoststorage(ctx, client, tenant1, id)
	require.NoError(t, err)
}

func TestInvClient_SetHostAsConnectionLost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	assertHostStatus := func(
		resourceID string,
		expectedModernStatus inv_status.ResourceStatus,
	) {
		getHost, err := invclient.GetHostResourceByResourceID(ctx, client, tenant1, resourceID)
		require.NoError(t, err)
		require.NotNil(t, getHost)
		assert.Equal(t, resourceID, getHost.GetResourceId())
		assert.Equal(t, expectedModernStatus.Status, getHost.GetHostStatus())
		assert.Equal(t, expectedModernStatus.StatusIndicator, getHost.GetHostStatusIndicator())
	}

	err := invclient.SetHostAsConnectionLost(ctx, client, tenant1, "host-12345678",
		uint64(time.Now().Unix()))
	require.Error(t, err)
	require.Equal(t, codes.NotFound, grpc_status.Convert(err).Code())

	host := dao.CreateHost(t, tenant1)
	hostUp := &computev1.HostResource{
		ResourceId: host.GetResourceId(),
	}

	// Should not update, only in RUNNING, BOOTING or ERROR state
	err = invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	require.NoError(t, err)

	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusEmpty)

	// OK, in RUNNING state
	hostUp.HostStatus = hrm_status.HostStatusRunning.Status
	err = invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	require.NoError(t, err)

	err = invclient.SetHostAsConnectionLost(ctx, client, tenant1, host.GetResourceId(),
		uint64(time.Now().Unix()))
	require.NoError(t, err)

	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusNoConnection)

	// set to BOOTING status
	hostUp.HostStatus = hrm_status.HostStatusBooting.Status
	err = invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	require.NoError(t, err)

	err = invclient.SetHostAsConnectionLost(ctx, client, tenant1, host.GetResourceId(),
		uint64(time.Now().Unix()))
	require.NoError(t, err)

	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusNoConnection)

	// set to ERROR status
	hostUp.HostStatus = hrm_status.HostStatusError.Status
	err = invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	require.NoError(t, err)

	err = invclient.SetHostAsConnectionLost(ctx, client, tenant1, host.GetResourceId(),
		uint64(time.Now().Unix()))
	require.NoError(t, err)

	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusNoConnection)

	// No set due to newer timestamp in the host res
	hostUp.HostStatus = hrm_status.HostStatusRunning.Status
	hostUp.HostStatusIndicator = hrm_status.HostStatusRunning.StatusIndicator
	hostUp.HostStatusTimestamp = uint64(time.Now().Unix())
	err = invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusRunning)

	require.NoError(t, err)
	// This won't change the host status because timestamp is in the past
	err = invclient.SetHostAsConnectionLost(ctx, client, tenant1, host.GetResourceId(), 0)
	require.NoError(t, err)
	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusRunning)

	// Change status given timestamp in the future
	err = invclient.SetHostAsConnectionLost(
		ctx, client, tenant1, host.GetResourceId(),
		uint64(time.Now().Add(time.Second).Unix()))
	require.NoError(t, err)
	assertHostStatus(host.GetResourceId(), hrm_status.HostStatusNoConnection)
}

func TestInvClient_SetHostStatus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	err := invclient.SetHostStatus(
		ctx,
		client,
		tenant1,
		"foobar",
		hrm_status.HostStatusRunning,
	)
	require.Error(t, err)

	host := dao.CreateHost(t, tenant1)
	timeBeforeUpdate := time.Now().Unix()
	err = invclient.SetHostStatus(
		ctx,
		client,
		tenant1,
		host.GetResourceId(),
		hrm_status.HostStatusRunning,
	)
	require.NoError(t, err)

	getHost, err := invclient.GetHostResourceByGUID(ctx, client, tenant1, host.GetUuid())
	require.NoError(t, err)
	require.NotNil(t, getHost)
	assert.Equal(t, host.GetUuid(), getHost.GetUuid())
	assert.Equal(t, hrm_status.HostStatusRunning.Status, getHost.GetHostStatus())
	assert.Equal(t, hrm_status.HostStatusRunning.StatusIndicator, getHost.GetHostStatusIndicator())
	assert.LessOrEqual(t, uint64(timeBeforeUpdate), getHost.GetHostStatusTimestamp())
}

func TestInvClient_GetHostResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error - no hosts
	hosts, err := invclient.GetHostResources(ctx, client)
	require.NoError(t, err)
	require.Empty(t, hosts)

	// OK
	host := dao.CreateHost(t, tenant1)
	hosts, err = invclient.GetHostResources(ctx, client)
	require.NoError(t, err)
	require.NotNil(t, hosts)
	gotHost := hosts[0]
	assert.Equal(t, host.GetResourceId(), gotHost.GetResourceId())
}

func TestInvClient_FindAllTrustedHosts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// creating 4 hosts
	host1 := dao.CreateHost(t, tenant1)
	host2 := dao.CreateHost(t, tenant1)
	host3 := dao.CreateHost(t, tenant1)
	host4 := dao.CreateHostNoCleanup(t, tenant1)

	allHosts, err := invclient.FindAllTrustedHosts(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host1.GetResourceId()})
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host2.GetResourceId()})
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host3.GetResourceId()})
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host4.GetResourceId()})

	// delete one host
	dao.HardDeleteHost(t, tenant1, host4.GetResourceId())

	allHosts, err = invclient.FindAllTrustedHosts(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host1.GetResourceId()})
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host2.GetResourceId()})
	assert.Contains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host3.GetResourceId()})
	assert.NotContains(t, allHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host4.GetResourceId()})

	// deauthorize one host
	// set UNTRUSTED desired state to generate event
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// we set both current and desired states to simulate untrusting behavior
	res := &inventoryv1.Resource{
		Resource: &inventoryv1.Resource_Host{
			Host: &computev1.HostResource{
				TenantId:     tenant1,
				ResourceId:   host3.GetResourceId(),
				DesiredState: computev1.HostState_HOST_STATE_UNTRUSTED,
			},
		},
	}
	fmk := fieldmaskpb.FieldMask{Paths: []string{computev1.HostResourceFieldDesiredState}}
	_, err = inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, host3.GetResourceId(), &fmk, res)
	require.NoError(t, err)

	_, err = inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, host3.GetResourceId(),
			&fieldmaskpb.FieldMask{Paths: []string{computev1.HostResourceFieldCurrentState}},
			&inventoryv1.Resource{
				Resource: &inventoryv1.Resource_Host{
					Host: &computev1.HostResource{
						TenantId:     tenant1,
						ResourceId:   host3.GetResourceId(),
						CurrentState: computev1.HostState_HOST_STATE_UNTRUSTED,
					},
				},
			})
	require.NoError(t, err)

	trustedHosts, err := invclient.FindAllTrustedHosts(ctx, client)
	require.NoError(t, err)
	assert.Contains(t, trustedHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host1.GetResourceId()})
	assert.Contains(t, trustedHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host2.GetResourceId()})
	assert.NotContains(t, trustedHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host3.GetResourceId()})
	assert.NotContains(t, trustedHosts, util.TenantIDResourceIDTuple{TenantID: tenant1, ResourceID: host4.GetResourceId()})
}

//nolint:funlen // it is a table-driven test
func Test_updateInvResourceFields(t *testing.T) { //nolint:cyclop // it is a test, which doesn't go to production
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	hostInv := dao.CreateHost(t, tenant1)
	hostUsbInv := dao.CreateHostUsb(t, tenant1, hostInv)
	hostNicInv := dao.CreateHostNic(t, tenant1, hostInv)
	hostStorageInv := dao.CreateHostStorage(t, tenant1, hostInv)
	hostNicIPAddrInv := dao.CreateIPAddress(t, tenant1, hostNicInv, true)

	type args struct {
		resource protoreflect.ProtoMessage
		fields   []string
	}
	tests := []struct {
		name          string
		args          args
		valid         bool
		assertionFunc func(t *testing.T, newObj interface{}, oldObj interface{})
	}{
		{
			name: "Success_HostResource",
			args: args{
				resource: &computev1.HostResource{
					ResourceId:   hostInv.GetResourceId(),
					CurrentState: computev1.HostState_HOST_STATE_ONBOARDED,
					BiosVendor:   "some vendor",
				},
				fields: []string{
					computev1.HostResourceFieldCurrentState,
				},
			},
			valid: true,
			assertionFunc: func(t *testing.T, newObj interface{}, oldObj interface{}) {
				t.Helper()

				newHost, ok1 := newObj.(*computev1.HostResource)
				require.Truef(t, ok1, "casting of a newObj to HostResource failed")

				oldHost, ok2 := oldObj.(*computev1.HostResource)
				require.Truef(t, ok2, "casting of an oldObj to HostResource failed")

				assert.Equal(t, newHost.GetCurrentState(), computev1.HostState_HOST_STATE_ONBOARDED)
				assert.Equal(t, newHost.GetBiosVendor(), oldHost.GetBiosVendor())
			},
		},
		{
			name: "Success_HostUSBResource",
			args: args{
				resource: &computev1.HostusbResource{
					ResourceId: hostUsbInv.GetResourceId(),
					Serial:     "XXYZ",
					Bus:        4,
				},
				fields: []string{
					computev1.HostusbResourceFieldBus,
					computev1.HostusbResourceFieldSerial,
				},
			},
			valid: true,
			assertionFunc: func(t *testing.T, newObj interface{}, _ interface{}) {
				t.Helper()

				newUsb, ok1 := newObj.(*computev1.HostusbResource)
				require.Truef(t, ok1, "casting of a newObj to HostusbResource failed")

				assert.Equal(t, newUsb.GetSerial(), "XXYZ")
				assert.Equal(t, newUsb.GetBus(), uint32(4))
			},
		},
		{
			name: "Success_HostUSBResource",
			args: args{
				resource: &computev1.HostusbResource{
					ResourceId: hostUsbInv.GetResourceId(),
					Serial:     "XXYZ",
					Bus:        4,
				},
				fields: []string{
					computev1.HostusbResourceFieldBus,
					computev1.HostusbResourceFieldSerial,
				},
			},
			valid: true,
			assertionFunc: func(t *testing.T, newObj interface{}, oldObj interface{}) {
				t.Helper()

				newUsb, ok1 := newObj.(*computev1.HostusbResource)
				require.Truef(t, ok1, "casting of a newObj to HostusbResource failed")

				_, ok2 := oldObj.(*computev1.HostusbResource)
				require.Truef(t, ok2, "casting of a newObj to HostusbResource failed")

				assert.Equal(t, newUsb.GetSerial(), "XXYZ")
				assert.Equal(t, newUsb.GetBus(), uint32(4))
			},
		},
		{
			name: "Success_HostNICResource",
			args: args{
				resource: &computev1.HostnicResource{
					ResourceId: hostNicInv.GetResourceId(),
					Mtu:        1500,
					DeviceName: "ens3",
				},
				fields: []string{
					computev1.HostnicResourceFieldMtu,
					computev1.HostnicResourceFieldDeviceName,
				},
			},
			valid: true,
			assertionFunc: func(t *testing.T, newObj interface{}, _ interface{}) {
				t.Helper()

				newNic, ok1 := newObj.(*computev1.HostnicResource)
				require.Truef(t, ok1, "casting of a newObj to HostnicResource failed")

				assert.Equal(t, newNic.GetMtu(), uint32(1500))
				assert.Equal(t, newNic.GetDeviceName(), "ens3")
			},
		},
		{
			name: "Success_HostStorageResource",
			args: args{
				resource: &computev1.HoststorageResource{
					ResourceId: hostStorageInv.GetResourceId(),
					DeviceName: "s1",
					Vendor:     "some vendor",
				},
				fields: []string{
					computev1.HoststorageResourceFieldVendor,
					computev1.HoststorageResourceFieldDeviceName,
				},
			},
			valid: true,
			assertionFunc: func(t *testing.T, newObj interface{}, _ interface{}) {
				t.Helper()

				newStorage, ok1 := newObj.(*computev1.HoststorageResource)
				require.Truef(t, ok1, "casting of a newObj to HoststorageResource failed")

				assert.Equal(t, newStorage.GetVendor(), "some vendor")
				assert.Equal(t, newStorage.GetDeviceName(), "s1")
			},
		},
		{
			name: "Success_IPAddress",
			args: args{
				resource: &networkv1.IPAddressResource{
					ResourceId: hostNicIPAddrInv.GetResourceId(),
					Address:    "17.0.0.1/24",
				},
				fields: []string{networkv1.IPAddressResourceFieldAddress},
			},
			valid:         true,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
		{
			name: "Success_NoFields",
			args: args{
				resource: &computev1.HostnicResource{
					ResourceId: hostNicInv.GetResourceId(),
					Mtu:        1500,
					DeviceName: "ens3",
				},
				fields: []string{},
			},
			valid:         true,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
		{
			name: "Failed_NoResourceID",
			args: args{
				resource: &computev1.HostnicResource{
					DeviceName: "ens3",
				},
				fields: []string{computev1.HostnicResourceFieldDeviceName},
			},
			valid:         false,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
		{
			name: "Failed_UnsupportedResourceType",
			args: args{
				resource: &networkv1.NetlinkResource{
					ResourceId: hostInv.GetResourceId(),
					Kind:       "kind",
				},
				fields: []string{networkv1.NetlinkResourceFieldKind},
			},
			valid:         false,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
		{
			name: "Failed_InvalidFieldMask",
			args: args{
				resource: &computev1.HostnicResource{
					ResourceId: hostNicInv.GetResourceId(),
					Mtu:        1500,
					DeviceName: "ens3",
				},
				fields: []string{networkv1.IPAddressResourceFieldAddress},
			},
			valid:         false,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
		{
			name: "Failed_NilResource",
			args: args{
				resource: nil,
				fields:   []string{},
			},
			valid:         false,
			assertionFunc: func(*testing.T, interface{}, interface{}) {},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := invclient.UpdateInvResourceFields(context.Background(), client, tenant1, tt.args.resource, tt.args.fields)
			if err != nil {
				if tt.valid {
					t.Errorf("UpdateInvResourceFields() failed: %s", err)
					t.FailNow()
				}
			} else {
				if !tt.valid {
					t.Errorf("UpdateInvResourceFields() succeeded but should have failed")
					t.FailNow()
				}
			}

			if !t.Failed() && tt.valid {
				switch tt.args.resource.(type) {
				case *computev1.HostResource:
					host, err := invclient.GetHostResourceByGUID(
						context.Background(), client, tenant1, hostInv.GetUuid())
					require.NoError(t, err)
					require.NotNil(t, host)

					tt.assertionFunc(t, host, hostInv)
				case *computev1.HostusbResource:
					host, err := invclient.GetHostResourceByGUID(
						context.Background(), client, tenant1, hostInv.GetUuid())
					require.NoError(t, err)
					require.NotNil(t, host)
					require.NotNil(t, host.HostUsbs)

					tt.assertionFunc(t, host.HostUsbs[0], hostUsbInv)
				case *computev1.HostnicResource:
					host, err := invclient.GetHostResourceByGUID(
						context.Background(), client, tenant1, hostInv.GetUuid())
					require.NoError(t, err)
					require.NotNil(t, host)
					require.NotNil(t, host.HostNics)
					require.Len(t, host.HostNics, 1)

					tt.assertionFunc(t, host.HostNics[0], hostNicInv)
				case *computev1.HoststorageResource:
					host, err := invclient.GetHostResourceByGUID(
						context.Background(), client, tenant1, hostInv.GetUuid())
					require.NoError(t, err)
					require.NotNil(t, host)
					require.NotNil(t, host.HostStorages)
					require.Len(t, host.HostStorages, 1)

					tt.assertionFunc(t, host.HostStorages[0], hostStorageInv)
				}
			}
		})
	}
}

func TestUpdateHostStatus(t *testing.T) {
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	hostInv := dao.CreateHost(t, tenant1)

	type args struct {
		host       *computev1.HostResource
		hostStatus inv_status.ResourceStatus
	}
	tests := []struct {
		name  string
		args  args
		valid bool
	}{
		{
			name: "Success",
			args: args{
				host:       hostInv,
				hostStatus: hrm_status.HostStatusRunning,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeBeforeUpdate := time.Now().Unix()

			tt.args.host.HostStatus = tt.args.hostStatus.Status
			tt.args.host.HostStatusIndicator = tt.args.hostStatus.StatusIndicator
			tt.args.host.HostStatusTimestamp = uint64(time.Now().Unix())

			err := invclient.UpdateHostStatus(context.Background(), client, tenant1, tt.args.host)
			if err != nil {
				if tt.valid {
					t.Errorf("UpdateHostStatus() failed: %s", err)
					t.FailNow()
				}
			} else {
				if !tt.valid {
					t.Errorf("UpdateHostStatus() succeeded but should have failed")
					t.FailNow()
				}
			}

			if !t.Failed() && tt.valid {
				h, err := invclient.GetHostResourceByResourceID(context.Background(), client, tenant1, hostInv.GetResourceId())
				require.NoError(t, err)
				require.NotNil(t, h)

				assert.Equal(t, tt.args.hostStatus.Status, h.GetHostStatus())
				assert.Equal(t, tt.args.hostStatus.StatusIndicator, h.GetHostStatusIndicator())
				assert.LessOrEqual(t, uint64(timeBeforeUpdate), h.GetHostStatusTimestamp())
			}
		})
	}
}

func TestInvClient_CreateHostnic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Should be error
	id, err := invclient.CreateHostnic(ctx, client, tenant1, &computev1.HostnicResource{})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpc_status.Code(err))
	assert.Empty(t, id)

	host := dao.CreateHost(t, tenant1)

	// Create with resourceId is not allowed
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
		ResourceId:    "hostnic-12345678",
	}
	id, err = invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.Error(t, err)
	assert.Equal(t, "", id)

	// OK
	nic = &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	id, err = invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })
}

func TestInvClient_UpdateHostnic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	err := invclient.UpdateHostnic(ctx, client, tenant1, &computev1.HostnicResource{})
	require.Error(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	id, err := invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)

	nic.SriovEnabled = false
	nic.ResourceId = id
	err = invclient.UpdateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })
}

func TestInvClient_DeleteHostnic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Not found, does not return error
	err := invclient.DeleteHostnic(ctx, client, tenant1, "hostnic-12345678")
	require.NoError(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	id, err := invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)

	err = invclient.DeleteHostnic(ctx, client, tenant1, id)
	require.NoError(t, err)
}

func TestInvClient_CreateHostgpu(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Should be error
	id, err := invclient.CreateHostgpu(ctx, client, tenant1, &computev1.HostgpuResource{})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, grpc_status.Code(err))
	assert.Empty(t, id)

	host := dao.CreateHost(t, tenant1)

	// Create with resourceId is not allowed
	gpu := &computev1.HostgpuResource{
		TenantId:   tenant1,
		Host:       host,
		DeviceName: "eth1",
		PciId:      "0000:b1:00.0",
		ResourceId: "hostnic-12345678",
	}
	id, err = invclient.CreateHostgpu(ctx, client, tenant1, gpu)
	require.Error(t, err)
	assert.Equal(t, "", id)

	// OK
	gpu = &computev1.HostgpuResource{
		TenantId:    tenant1,
		Host:        host,
		DeviceName:  "eth1",
		PciId:       "0000:b1:00.0",
		Vendor:      "Intel",
		Product:     "XYZ",
		Description: "some desc",
	}
	id, err = invclient.CreateHostgpu(ctx, client, tenant1, gpu)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, id) })
}

func TestInvClient_UpdateHostgpu(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	err := invclient.UpdateHostgpu(ctx, client, tenant1, &computev1.HostgpuResource{})
	require.Error(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	gpu := dao.CreateHostGPU(t, tenant1, host)

	gpu.Vendor = ""
	gpu.PciId = "00:01"
	gpu.Features = "abc,xyz"
	// dao.CreateHostGPU sets Host to nil, we need to restore the mandatory field
	gpu.Host = host

	err = invclient.UpdateHostgpu(ctx, client, tenant1, gpu)
	require.NoError(t, err)

	resp, err := client.Get(ctx, tenant1, gpu.ResourceId)
	require.NoError(t, err)
	invGpu := resp.GetResource().GetHostgpu()
	assert.Equal(t, gpu.Vendor, invGpu.Vendor)
	assert.Equal(t, gpu.PciId, invGpu.PciId)
	assert.Equal(t, gpu.Features, invGpu.Features)
}

func TestInvClient_DeleteHostgpu(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Not found, does not return error
	err := invclient.DeleteHostgpu(ctx, client, tenant1, "hostgpu-12345678")
	require.NoError(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	gpu := dao.CreateHostGPUNoCleanup(t, tenant1, host)

	err = invclient.DeleteHostgpu(ctx, client, tenant1, gpu.ResourceId)
	require.NoError(t, err)
}

func TestInvClient_CreateIPAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// IPAddress cannot exist without a valid nic
	ip := &networkv1.IPAddressResource{
		TenantId:     tenant1,
		Address:      "10.0.0.1/24",
		CurrentState: networkv1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
		ConfigMethod: networkv1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
	}
	idEmpty, err := invclient.CreateIPAddress(ctx, client, tenant1, ip)
	require.Error(t, err)
	assert.Equal(t, "", idEmpty)

	// OK
	host := dao.CreateHost(t, tenant1)
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	idHostNic, err := invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", idHostNic)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, idHostNic) })

	nic.ResourceId = idHostNic
	ip.Nic = nic
	idIPAddress, err := invclient.CreateIPAddress(ctx, client, tenant1, ip)
	require.NoError(t, err)
	assert.NotEqual(t, "", idIPAddress)
	t.Cleanup(func() { dao.HardDeleteIPAddress(t, tenant1, idIPAddress) })
}

func TestInvClient_UpdateIPAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Error
	err := invclient.UpdateIPAddress(ctx, client, tenant1, &networkv1.IPAddressResource{})
	require.Error(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	idHostNic, err := invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", idHostNic)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, idHostNic) })

	nic.ResourceId = idHostNic
	ip := &networkv1.IPAddressResource{
		TenantId:     tenant1,
		Address:      "10.0.0.1/24",
		CurrentState: networkv1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
		ConfigMethod: networkv1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
		Nic:          nic,
	}
	idIPAddress, err := invclient.CreateIPAddress(ctx, client, tenant1, ip)
	require.NoError(t, err)
	assert.NotEqual(t, "", idIPAddress)
	t.Cleanup(func() { dao.HardDeleteIPAddress(t, tenant1, idIPAddress) })

	ip.ConfigMethod = networkv1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_STATIC
	ip.ResourceId = idIPAddress
	err = invclient.UpdateIPAddress(ctx, client, tenant1, ip)
	require.NoError(t, err)
}

func TestInvClient_DeleteIPAddress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Not found, does not return error
	err := invclient.DeleteIPAddress(ctx, client, tenant1, "ipaddr-12345678")
	require.NoError(t, err)

	// OK
	host := dao.CreateHost(t, tenant1)
	nic := &computev1.HostnicResource{
		TenantId:      tenant1,
		Host:          host,
		DeviceName:    "eth1",
		MacAddr:       "ee:ee:ee:ee:ee:ee",
		PciIdentifier: "0000:b1:00.0",
		SriovEnabled:  true,
		SriovVfsNum:   8,
		SriovVfsTotal: 128,
		Mtu:           1500,
		LinkState:     computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:  false,
	}
	idHostNic, err := invclient.CreateHostnic(ctx, client, tenant1, nic)
	require.NoError(t, err)
	assert.NotEqual(t, "", idHostNic)
	t.Cleanup(func() { dao.DeleteResource(t, tenant1, idHostNic) })

	nic.ResourceId = idHostNic
	ip := &networkv1.IPAddressResource{
		TenantId:     tenant1,
		Address:      "10.0.0.1/24",
		CurrentState: networkv1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
		ConfigMethod: networkv1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
		Nic:          nic,
	}
	id, err := invclient.CreateIPAddress(ctx, client, tenant1, ip)
	require.NoError(t, err)
	assert.NotEqual(t, "", id)

	err = invclient.DeleteIPAddress(ctx, client, tenant1, id)
	require.NoError(t, err)
}

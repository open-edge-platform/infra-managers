// SPDX-FileCopyrightText: (C) 2026 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	om_status "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/status"
)

// Verify Add/Remove of DeviceInfo resources.
//
//nolint:funlen // it's a test
func TestHostManagerClient_AddRemovDevice(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})

	testcases := map[string]struct {
		in    *pb.DeviceInfo
		valid bool
	}{
		"GoodDevice1": {
			in: &pb.DeviceInfo{
				Version:          "1.2.34",
				Hostname:         "testhost",
				OperationalState: "enabled",
				BuildNumber:      "1234",
				Sku:              "5467",
				Features:         "features string for test",
				DeviceGuid:       "1234abcd-ef56-7890-12ab-34567890cdef",
				ControlMode:      "client",
				DnsSuffix:        "testhost.com",
				RasInfo: &pb.RASInfo{
					NetworkStatus: "direct",
					RemoteStatus:  "not connected",
					RemoteTrigger: "user initiated",
					MpsHostname:   "",
				},
			},
			valid: true,
		},
		"GoodDevice2": {
			in: &pb.DeviceInfo{
				RasInfo: &pb.RASInfo{},
			},
			valid: true,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the storage resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.DeviceInfo = tc.in
			_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
			if err != nil {
				if tc.valid {
					t.Errorf("UpdateHostSystemInfoByGUID() failed: %s", err)
					t.FailNow()
				}
			} else {
				if !tc.valid {
					t.Errorf("UpdateHostSystemInfoByGUID() succeeded but should have failed")
					t.FailNow()
				}
			}

			// only get/delete if valid test and hasn't failed otherwise may segfault
			if !t.Failed() && tc.valid {
				// validate with get
				host := GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				invDevice := host.HostDevice
				hostDevice := ConvertDeviceInfoIntoHostDevice(t, tc.in, host)
				assertSameHostDevice(t, hostDevice, invDevice)

				systemInfo1.SystemInfo.DeviceInfo = &pb.DeviceInfo{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostDevice)
			}
		})
	}
}

// Verify update of the DeviceInfo resources.
//
//nolint:funlen // it's a test
func TestHostManagerClient_UpdateDevice(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHoststoragesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    *pb.DeviceInfo
		valid bool
	}{
		"FirstDevice": {
			in: &pb.DeviceInfo{
				Version:          "1.2.34",
				Hostname:         "testhost",
				OperationalState: "enabled",
				BuildNumber:      "1234",
				Sku:              "5467",
				Features:         "features string for test",
				DeviceGuid:       "1234abcd-ef56-7890-12ab-34567890cdef",
				ControlMode:      "client",
				DnsSuffix:        "testhost.com",
				RasInfo: &pb.RASInfo{
					NetworkStatus: "direct",
					RemoteStatus:  "not connected",
					RemoteTrigger: "user initiated",
					MpsHostname:   "",
				},
			},
			valid: true,
		},
		"UpdatedDevice": {
			in: &pb.DeviceInfo{
				Version:          "1.2.56",
				Hostname:         "testhost",
				OperationalState: "enabled",
				BuildNumber:      "7890",
				Sku:              "5467",
				Features:         "features string for test",
				DeviceGuid:       "1234abcd-ef56-7890-12ab-34567890cdef",
				ControlMode:      "client",
				DnsSuffix:        "testhost.com",
				RasInfo: &pb.RASInfo{
					NetworkStatus: "direct",
					RemoteStatus:  "not connected",
					RemoteTrigger: "user initiated",
					MpsHostname:   "",
				},
			},
			valid: true,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the storage resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.DeviceInfo = tc.in
			_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
			if err != nil {
				if tc.valid {
					t.Errorf("UpdateHostSystemInfoByGUID() failed: %s", err)
					t.FailNow()
				}
			} else {
				if !tc.valid {
					t.Errorf("UpdateHostSystemInfoByGUID() succeeded but should have failed")
					t.FailNow()
				}
			}

			// only get if valid test and hasn't failed otherwise may segfault
			if !t.Failed() && tc.valid {
				// validate with get
				host := GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				invDevice := host.HostDevice
				hostDevice := ConvertDeviceInfoIntoHostDevice(t, tc.in, host)
				assertSameHostDevice(t, hostDevice, invDevice)
			}
		})
	}
}

// Verify no changes are applied.
func TestHostManagerClient_UpdateDeviceNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostdeviceWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	device := &pb.DeviceInfo{
		Version:          "1.2.34",
		Hostname:         "testhost",
		OperationalState: "enabled",
		BuildNumber:      "1234",
		Sku:              "5467",
		Features:         "features string for test",
		DeviceGuid:       "1234abcd-ef56-7890-12ab-34567890cdef",
		ControlMode:      "client",
		DnsSuffix:        "testhost.com",
		RasInfo: &pb.RASInfo{
			NetworkStatus: "direct",
			RemoteStatus:  "not connected",
			RemoteTrigger: "user initiated",
			MpsHostname:   "",
		},
	}

	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.DeviceInfo = device
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invDevice := host.HostDevice
	hostDevice := ConvertDeviceInfoIntoHostDevice(t, device, host)
	assertSameHostDevice(t, hostDevice, invDevice)
}

func assertSameHostDevice(t *testing.T, expectedDevice, actualDevice *computev1.HostdeviceResource) {
	t.Helper()

	// make the devices comparable
	expectedDevice.ResourceId = ""
	expectedDevice.Host = nil
	expectedDevice.CreatedAt = ""
	expectedDevice.UpdatedAt = ""
	actualDevice.ResourceId = ""
	actualDevice.Host = nil
	actualDevice.CreatedAt = ""
	actualDevice.UpdatedAt = ""
	if eq, diff := inv_testing.ProtoEqualOrDiff(expectedDevice, actualDevice); !eq {
		t.Errorf("HostDevice data not equal: %v", diff)
	}
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	om_status "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/status"
	"testing"

	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
)

// Verify Add/Remove of USB resources.
func TestHostManagerClient_AddRemoveUsb(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})

	testcases := map[string]struct {
		in    []*pb.SystemUSB
		valid bool
	}{
		"GoodUsb1": {
			in: []*pb.SystemUSB{
				{
					Bus:      1,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo",
				},
			},
			valid: true,
		},
		"GoodUsb2": {
			in: []*pb.SystemUSB{
				{
					Bus:      1,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo",
				},
				{
					Bus:      10,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo2",
				},
			},
			valid: true,
		},
		"GoodUsb3": {
			in:    []*pb.SystemUSB{},
			valid: true,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the usb resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Usb = tc.in
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
				invUsbs := host.HostUsbs
				hostUsbs := ConvertSystemUSBIntoHostUsbs(t, tc.in, host)
				assertSameHostUsb(t, hostUsbs, invUsbs)

				systemInfo1.SystemInfo.HwInfo.Usb = []*pb.SystemUSB{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostUsbs)
			}
		})
	}
}

// Verify update of the USB resources.
func TestHostManagerClient_UpdateUsb(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostusbResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    []*pb.SystemUSB
		valid bool
	}{
		"OneUsb": {
			in: []*pb.SystemUSB{
				{
					Bus:      1,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo",
				},
			},
			valid: true,
		},
		"TwoUsbs": {
			in: []*pb.SystemUSB{
				{
					Bus:      1,
					Addr:     1,
					Class:    "HghFooBar",
					Idvendor: "Fo",
				},
				{
					Bus:      10,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo",
				},
			},
			valid: true,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the usb resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Usb = tc.in
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
				invUsbs := host.HostUsbs
				hostUsbs := ConvertSystemUSBIntoHostUsbs(t, tc.in, host)
				assertSameHostUsb(t, hostUsbs, invUsbs)
			}
		})
	}
}

// Verify no changes are applied.
func TestHostManagerClient_UpdateUsbNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostusbResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	usb := []*pb.SystemUSB{
		{
			Bus:      1,
			Addr:     1,
			Class:    "HighFooBar",
			Idvendor: "Foo",
		},
	}
	req := &pb.UpdateHostSystemInfoByGUIDRequest{
		HostGuid:   hostInv.GetUuid(),
		SystemInfo: &pb.SystemInfo{},
	}
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, req)
	require.NoError(t, err)

	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.HwInfo.Usb = usb
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invUsbs := host.HostUsbs
	hostUsbs := ConvertSystemUSBIntoHostUsbs(t, usb, host)
	assertSameHostUsb(t, hostUsbs, invUsbs)
}

func assertSameHostUsb(t *testing.T, expectdUsbs, actualUsbs []*computev1.HostusbResource) {
	t.Helper()
	require.Equal(t, len(expectdUsbs), len(actualUsbs))

	OrderByBus(expectdUsbs)
	OrderByBus(actualUsbs)
	for i, expected := range expectdUsbs {
		// make the usbs comparable
		expected.ResourceId = ""
		expected.Host = nil
		expected.CreatedAt = ""
		expected.UpdatedAt = ""
		actual := actualUsbs[i]
		actual.ResourceId = ""
		actual.Host = nil
		actual.CreatedAt = ""
		actual.UpdatedAt = ""
		if eq, diff := inv_testing.ProtoEqualOrDiff(expected, actual); !eq {
			t.Errorf("HostUsb data not equal: %v", diff)
		}
	}
}

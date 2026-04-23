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

// Verify Add/Remove of AmtConfigInfo resources.
func TestHostManagerClient_AddRemoveAmt(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})

	testcases := map[string]struct {
		in    *pb.AmtConfigInfo
		valid bool
	}{
		"GoodAmt1": {
			in: &pb.AmtConfigInfo{
				Version:          "1.2.34",
				DeviceName:       "testhost",
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
		"GoodAmt2": {
			in: &pb.AmtConfigInfo{
				RasInfo: &pb.RASInfo{},
			},
			valid: true,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the Amtconfig resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.AmtInfo = tc.in
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
				invAmtconfig := host.HostAmtconfig
				hostAmtconfig := ConvertAmtConfigInfoIntoHostAmtconfig(t, tc.in, host)
				assertSameHostAmtconfig(t, hostAmtconfig, invAmtconfig)

				systemInfo1.SystemInfo.AmtInfo = &pb.AmtConfigInfo{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostAmtconfig)
			}
		})
	}
}

// Verify update of the AmtConfigInfo resources.
func TestHostManagerClient_UpdateAmt(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostamtconfigWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    *pb.AmtConfigInfo
		valid bool
	}{
		"FirstAmt": {
			in: &pb.AmtConfigInfo{
				Version:          "1.2.34",
				DeviceName:       "testhost",
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
		"UpdatedAmt": {
			in: &pb.AmtConfigInfo{
				Version:          "1.2.56",
				DeviceName:       "testhost",
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
			systemInfo1.SystemInfo.AmtInfo = tc.in
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
				invAmtconfig := host.HostAmtconfig
				hostAmtconfig := ConvertAmtConfigInfoIntoHostAmtconfig(t, tc.in, host)
				assertSameHostAmtconfig(t, hostAmtconfig, invAmtconfig)
			}
		})
	}
}

// Verify no changes are applied.
func TestHostManagerClient_UpdateAmtNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostamtconfigWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	amtconfig := &pb.AmtConfigInfo{
		Version:          "1.2.34",
		DeviceName:       "testhost",
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
	systemInfo1.SystemInfo.AmtInfo = amtconfig
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invAmtconfig := host.HostAmtconfig
	hostAmtconfig := ConvertAmtConfigInfoIntoHostAmtconfig(t, amtconfig, host)
	assertSameHostAmtconfig(t, hostAmtconfig, invAmtconfig)
}

func assertSameHostAmtconfig(t *testing.T, expectedAmtconfig, actualAmtconfig *computev1.HostamtconfigResource) {
	t.Helper()

	// make the amtconfigs comparable
	expectedAmtconfig.ResourceId = ""
	expectedAmtconfig.Host = nil
	expectedAmtconfig.CreatedAt = ""
	expectedAmtconfig.UpdatedAt = ""
	actualAmtconfig.ResourceId = ""
	actualAmtconfig.Host = nil
	actualAmtconfig.CreatedAt = ""
	actualAmtconfig.UpdatedAt = ""
	if eq, diff := inv_testing.ProtoEqualOrDiff(expectedAmtconfig, actualAmtconfig); !eq {
		t.Errorf("HostAmtconfig data not equal: %v", diff)
	}
}

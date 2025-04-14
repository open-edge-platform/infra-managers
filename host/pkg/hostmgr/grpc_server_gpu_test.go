// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
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

// TestUpdateGPUInfoDeprecated validates that the deprecated SystemGPU field is still handled
// to ensure backwards compatibility with older HD agents. As HDA always sent an empty SystemGPU,
// the expected behavior is that HRM will accept the proto message, but ignore deprecated SystemGPU.
func TestUpdateGPUInfoDeprecated(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	os := dao.CreateOs(t, tenant1)
	hostInv := dao.CreateHost(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	systemInfo1.HostGuid = hostInv.GetUuid()
	//nolint:staticcheck // deprecated field will be removed in future
	systemInfo1.SystemInfo.HwInfo.GpuDeprecated = &pb.SystemGPU{}

	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	require.Len(t, host.HostGpus, 0)
}

func TestHostManagerClient_AddUpdateRemoveGPU(t *testing.T) { //nolint:funlen // it is a table-driven test
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})

	testcases := map[string]struct {
		in    []*pb.SystemGPU
		valid bool
	}{
		"GoodGPU_One": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     "some product",
					Vendor:      "Intel",
					Name:        "gpu0",
					Description: "some desc",
				},
			},
			valid: true,
		},
		"GoodGPU_Two": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     "some product",
					Vendor:      "Intel",
					Name:        "gpu0",
					Description: "some desc",
				},
				{
					PciId:       "0000:00:1f.7",
					Product:     "some product",
					Vendor:      "Intel",
					Name:        "gpu0",
					Description: "some desc",
				},
			},
			valid: true,
		},
		"GoodGPU_Empty": { // this means delete all existing GPUs
			in:    []*pb.SystemGPU{},
			valid: true,
		},
		"Invalid_EmptyPciID": {
			in: []*pb.SystemGPU{
				{
					Product:     "some product",
					Vendor:      "Intel",
					Name:        "gpu0",
					Description: "some desc",
				},
			},
			valid: false,
		},
		"Invalid_ProductNameTooLong": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     TooLongString,
					Vendor:      "Intel",
					Name:        "gpu0",
					Description: "some desc",
				},
			},
			valid: false,
		},
		"Invalid_VendorTooLong": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     "some product",
					Vendor:      TooLongString,
					Name:        "gpu0",
					Description: "some desc",
				},
			},
			valid: false,
		},
		"Invalid_NameTooLong": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     "some product",
					Vendor:      "Intel",
					Name:        TooLongString,
					Description: "some desc",
				},
			},
			valid: false,
		},
		"Invalid_DescTooLong": {
			in: []*pb.SystemGPU{
				{
					PciId:       "0000:00:1f.6",
					Product:     "some product",
					Vendor:      "Intel",
					Name:        "gpu name",
					Description: TooLongString,
				},
			},
			valid: false,
		},
	}
	//nolint:dupl // keep the test cases separate.
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Gpu = tc.in
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
				invGpus := host.HostGpus
				hostGpus := ConvertSystemGPUIntoHostGpus(t, tc.in, host)
				assertSameGpus(t, hostGpus, invGpus)

				systemInfo1.SystemInfo.HwInfo.Gpu = []*pb.SystemGPU{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostGpus)
			}
		})
	}
}

func TestUpdateGPUNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	os := dao.CreateOs(t, tenant1)
	dao.CreateInstanceWithOpts(t, tenant1, hostInv, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
	})
	t.Cleanup(func() { HardDeleteHostgpusWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	gpu := []*pb.SystemGPU{
		{
			PciId:       "0000:00:1f.6",
			Product:     "some product",
			Vendor:      "Intel",
			Name:        "gpu name",
			Description: "desc",
		},
	}

	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.HwInfo.Gpu = gpu
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	// validate with get
	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invGpus := host.HostGpus
	hostGpus := ConvertSystemGPUIntoHostGpus(t, gpu, host)
	assertSameGpus(t, hostGpus, invGpus)
}

func assertSameGpus(t *testing.T, expectedGpus, actualGpus []*computev1.HostgpuResource) {
	t.Helper()
	assertSameResources(t, expectedGpus, actualGpus, OrderByDeviceName,
		func(expected, actual *computev1.HostgpuResource) (bool, string) {
			expected.ResourceId = ""
			expected.Host = nil
			expected.CreatedAt = ""
			expected.UpdatedAt = ""
			actual.ResourceId = ""
			actual.Host = nil
			actual.CreatedAt = ""
			actual.UpdatedAt = ""
			return inv_testing.ProtoEqualOrDiff(expected, actual)
		})
}

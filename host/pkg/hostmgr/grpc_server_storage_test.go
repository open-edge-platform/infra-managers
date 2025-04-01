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
)

// Verify Add/Remove of Storage resources.
func TestHostManagerClient_AddRemoveStorage(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)

	testcases := map[string]struct {
		in    *pb.Storage
		valid bool
	}{
		"GoodStorage1": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name:         "sda1",
						SerialNumber: "1234W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         1000000,
						Wwid:         "0x50026b7684e50ff8",
					},
				},
			},
			valid: true,
		},
		"GoodStorage2": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name:         "sda1",
						SerialNumber: "1234W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         1000000,
						Wwid:         "0x55cd2e415346252e",
					},
					{
						Name:         "sda2",
						SerialNumber: "1434W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         2000000,
						Wwid:         "eui.01000000010000005cd2e43cf16e5451",
					},
				},
			},
			valid: true,
		},
		"GoodStorage3": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{},
			},
			valid: true,
		},
		"Fail_InvalidName": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name: "",
					},
				},
			},
			valid: false,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the storage resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Storage = tc.in
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
				invStorages := host.HostStorages
				hostStorages := ConvertSystemDiskIntoHostStorages(t, tc.in, host)
				assertSameHostStorage(t, hostStorages, invStorages)

				systemInfo1.SystemInfo.HwInfo.Storage = &pb.Storage{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostStorages)
			}
		})
	}
}

// Verify update of the Storage resources.
func TestHostManagerClient_UpdateStorage(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	t.Cleanup(func() { HardDeleteHoststoragesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    *pb.Storage
		valid bool
	}{
		"OneStorage": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name:         "sda1",
						SerialNumber: "1234W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         1000000,
						Wwid:         "0x50026b7684e50ff8",
					},
				},
			},
			valid: true,
		},
		"TwoStorages": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name:         "sda1",
						SerialNumber: "1234W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         5000000,
						Wwid:         "0x50026b7684e50ff8",
					},
					{
						Name:         "sda2",
						SerialNumber: "1434W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         2000000,
						Wwid:         "0x55cd2e415346252e",
					},
				},
			},
			valid: true,
		},
		"Fail_InvalidName": {
			in: &pb.Storage{
				Disk: []*pb.SystemDisk{
					{
						Name:         "",
						SerialNumber: "1234W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         1000000,
						Wwid:         "0x50026b7684e50ff8",
					},
					{
						Name:         "sda2",
						SerialNumber: "1434W45678U",
						Vendor:       "Foobar Corp.",
						Model:        "SUN",
						Size:         2000000,
						Wwid:         "0x55cd2e415346252e",
					},
				},
			},
			valid: false,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the storage resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Storage = tc.in
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
				invStorages := host.HostStorages
				hostStorages := ConvertSystemDiskIntoHostStorages(t, tc.in, host)
				assertSameHostStorage(t, hostStorages, invStorages)
			}
		})
	}
}

// Verify no changes are applied.
func TestHostManagerClient_UpdateStorageNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	t.Cleanup(func() { HardDeleteHoststoragesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	storage := &pb.Storage{
		Disk: []*pb.SystemDisk{
			{
				Name:         "sda1",
				SerialNumber: "1234W45678U",
				Vendor:       "Foobar Corp.",
				Model:        "SUN",
				Size:         1000000,
				Wwid:         "0x50026b7684e50ff8",
			},
		},
	}

	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.HwInfo.Storage = storage
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invStorages := host.HostStorages
	hostStorages := ConvertSystemDiskIntoHostStorages(t, storage, host)
	assertSameHostStorage(t, hostStorages, invStorages)
}

func assertSameHostStorage(t *testing.T, expectedStorages, actualStorages []*computev1.HoststorageResource) {
	t.Helper()
	require.Equal(t, len(expectedStorages), len(actualStorages))

	OrderByDeviceName(expectedStorages)
	OrderByDeviceName(actualStorages)
	for i, expected := range expectedStorages {
		// make the storages comparable
		expected.ResourceId = ""
		expected.Host = nil
		expected.CreatedAt = ""
		expected.UpdatedAt = ""
		actual := actualStorages[i]
		actual.ResourceId = ""
		actual.Host = nil
		actual.CreatedAt = ""
		actual.UpdatedAt = ""
		if eq, diff := inv_testing.ProtoEqualOrDiff(expected, actual); !eq {
			t.Errorf("HostStorage data not equal: %v", diff)
		}
	}
}

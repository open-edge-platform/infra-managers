// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	networkv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
)

// Verify Add/Remove of SystemNetwork resources.
func TestHostManagerClient_AddRemoveNetwork(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)

	testcases := map[string]struct {
		in    []*pb.SystemNetwork
		valid bool
	}{
		"GoodNic1": {
			in: []*pb.SystemNetwork{
				{
					Name:         "ens3",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fd",
					PciId:        "0000:00:1f.6",
					Sriovenabled: false,
					Mtu:          1500,
					BmcNet:       false,
				},
			},
			valid: true,
		},
		"GoodNic2": {
			in: []*pb.SystemNetwork{
				{
					Name:         "ens3",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fd",
					PciId:        "0000:00:1f.6",
					Sriovenabled: true,
					Mtu:          1500,
				},
				{
					Name:         "ens8",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fe",
					PciId:        "0000:00:1f.7",
					Sriovenabled: false,
					BmcNet:       false,
				},
			},
			valid: true,
		},
		"GoodNic3": {
			in:    []*pb.SystemNetwork{},
			valid: true,
		},
		"Fail_InvalidName": {
			in: []*pb.SystemNetwork{
				{
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:7c:fd",
					Sriovenabled: false,
					Mtu:          150,
					BmcNet:       false,
				},
			},
			valid: false,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the network resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Network = tc.in
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
				invNics := host.HostNics
				hostNics := ConvertSystemNetworkIntoHostNics(t, tc.in, host)
				assertSameHostNics(t, hostNics, invNics)

				systemInfo1.SystemInfo.HwInfo.Network = []*pb.SystemNetwork{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")

				// validate again with get
				host = GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, host)
				require.Empty(t, host.HostNics)
			}
		})
	}
}

// Verify update of the SystemNetwork resources.
func TestHostManagerClient_UpdateNetwork1(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	t.Cleanup(func() { HardDeleteHostnicResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    []*pb.SystemNetwork
		valid bool
	}{
		"OneNic": {
			in: []*pb.SystemNetwork{
				{
					Name:         "ens4",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fa",
					PciId:        "0000:00:1f.6",
					Sriovenabled: false,
					Mtu:          1500,
					BmcNet:       false,
				},
			},
			valid: true,
		},
		"TwoNics": {
			in: []*pb.SystemNetwork{
				{
					Name:         "ens4",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fa",
					PciId:        "0000:00:1f.6",
					Sriovenabled: true,
					Mtu:          1500,
					BmcNet:       false,
				},
				{
					Name:         "eth0",
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:6c:fe",
					PciId:        "0000:00:1f.7",
					Sriovenabled: false,
					Mtu:          1500,
					BmcNet:       false,
				},
			},
			valid: true,
		},
		"Fail_InvalidMtu": {
			in: []*pb.SystemNetwork{
				{
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:7c:fd",
					Sriovenabled: false,
					Mtu:          0,
					BmcNet:       false,
				},
			},
			valid: false,
		},
		"Fail_InvalidName": {
			in: []*pb.SystemNetwork{
				{
					Sriovnumvfs:  0,
					Mac:          "90:49:fa:07:7c:fd",
					Sriovenabled: false,
					Mtu:          150,
					BmcNet:       false,
				},
			},
			valid: false,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
			defer cancel()

			// Modify the network resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Network = tc.in
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
				invNics := host.HostNics
				hostNics := ConvertSystemNetworkIntoHostNics(t, tc.in, host)
				assertSameHostNics(t, hostNics, invNics)
			}
		})
	}
}

// Verify no changes are applied.
func TestHostManagerClient_UpdateNetworkNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	t.Cleanup(func() { HardDeleteHostnicResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	network := []*pb.SystemNetwork{
		{
			Name:         "ens3",
			Sriovnumvfs:  0,
			Mac:          "90:49:fa:07:6c:fa",
			PciId:        "0000:00:1f.6",
			Sriovenabled: false,
			Mtu:          1500,
			BmcNet:       false,
		},
	}
	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.HwInfo.Network = network
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	// validate with get
	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	invNics := host.HostNics
	hostNics := ConvertSystemNetworkIntoHostNics(t, network, host)
	assertSameHostNics(t, hostNics, invNics)
}

// Verify Add/Remove of IP resources.
func TestHostManagerClient_AddRemoveIP(t *testing.T) { //nolint:funlen // it is a table-driven test
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)

	testcases := map[string]struct {
		in    []*pb.SystemNetwork
		valid bool
	}{
		"GoodIP1": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
					},
				},
			},
			valid: true,
		},
		"GoodIP2": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
						{
							IpAddress:         "fe80::61ff:fe9d:f156",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
						},
					},
				},
			},
			valid: true,
		},
		"GoodIP3": {
			in: []*pb.SystemNetwork{
				{
					Name:        "ens3",
					PciId:       "0000:00:1f.6",
					Mtu:         1500,
					IpAddresses: []*pb.IPAddress{},
				},
			},
			valid: true,
		},
		"GoodWithAndWithoutIP": {
			in: []*pb.SystemNetwork{
				{
					Name:        "ens2",
					PciId:       "0000:00:1f.6",
					Mtu:         1500,
					IpAddresses: []*pb.IPAddress{},
				},
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
						},
					},
				},
			},
			valid: true,
		},
		"Fail_InvalidPrefix": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 0,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
					},
				},
			},
			valid: false,
		},
		"Fail_InvalidAddress": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "fe80::61ff:fe9d:g156",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
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

			// Modify the network resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Network = tc.in
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
				require.NotEmpty(t, host.HostNics)

				// may be multiple NICs
				for _, hnic := range host.HostNics {
					invIPs := GetIPbyNicID(t, hnic.GetResourceId())

					tcindex := 0
					for tci, tcase := range tc.in {
						// go through list of interfaces, break when we get to the right one
						if tcase.Name == hnic.DeviceName {
							tcindex = tci
							break
						}
					}

					tcIPs := ConvertSystemNetworkIntoHostIPs(t, tc.in[tcindex].IpAddresses, hnic)
					assertSameIPAddress(t, tcIPs, invIPs)
				}

				// Note that NICs cannot be removed if IPs are not removed first
				systemInfo1.SystemInfo.HwInfo.Network = []*pb.SystemNetwork{}
				_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
				require.NoError(t, err, "UpdateHostSystemInfoByGUID() failed")
			}
		})
	}
}

// Verify update of the IP resources.
func TestHostManagerClient_UpdateIP(t *testing.T) { //nolint:funlen // it is a table-driven test
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	// Purge the entire network
	t.Cleanup(func() { HardDeleteHostnicResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	testcases := map[string]struct {
		in    []*pb.SystemNetwork
		valid bool
	}{
		"OneIP": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
					},
				},
			},
			valid: true,
		},
		"TwoIPs": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
						},
						{
							IpAddress:         "fe80::61ff:fe9d:f156",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
						},
					},
				},
			},
			valid: true,
		},
		"Fail_InvalidPrefix": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "192.168.0.11",
							NetworkPrefixBits: 0,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
					},
				},
			},
			valid: false,
		},
		"Fail_InvalidAddress": {
			in: []*pb.SystemNetwork{
				{
					Name:  "ens3",
					PciId: "0000:00:1f.6",
					Mtu:   1500,
					IpAddresses: []*pb.IPAddress{
						{
							IpAddress:         "fe80:0000:0000:0000:0204:61ff:fe9d:g156",
							NetworkPrefixBits: 24,
							ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
						},
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

			// Modify the network resources
			systemInfo1.HostGuid = hostInv.GetUuid()
			systemInfo1.SystemInfo.HwInfo.Network = tc.in
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
				require.NotEmpty(t, host.HostNics)
				// Assumption there is one nic
				invIPs := GetIPbyNicID(t, host.HostNics[0].GetResourceId())
				hostIPs := ConvertSystemNetworkIntoHostIPs(t, tc.in[0].IpAddresses, host.HostNics[0])
				assertSameIPAddress(t, hostIPs, invIPs)
			}
		})
	}
}

func TestHostManagerClient_UpdateIPNoChanges(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	hostInv := dao.CreateHost(t, tenant1)
	// Purge the entire network
	t.Cleanup(func() { HardDeleteHostnicResourcesWithUpdateHostSystemInfo(t, tenant1, systemInfo1) })

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	network := []*pb.SystemNetwork{
		{
			Name:  "ens3",
			PciId: "0000:00:1f.6",
			Mtu:   1500,
			IpAddresses: []*pb.IPAddress{
				{
					IpAddress:         "192.168.0.11",
					NetworkPrefixBits: 24,
					ConfigMode:        pb.ConfigMode_CONFIG_MODE_STATIC,
				},
			},
		},
	}
	systemInfo1.HostGuid = hostInv.GetUuid()
	systemInfo1.SystemInfo.HwInfo.Network = network
	_, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.NoError(t, err)

	host := GetHostbyUUID(t, hostInv.GetUuid())
	require.NotNil(t, host)
	require.NotEmpty(t, host.HostNics)
	// Assumption there is one nic
	invIPs := GetIPbyNicID(t, host.HostNics[0].GetResourceId())
	hostIPs := ConvertSystemNetworkIntoHostIPs(t, network[0].IpAddresses, host.HostNics[0])
	assertSameIPAddress(t, hostIPs, invIPs)
}

func assertSameHostNics(t *testing.T, expectedNics, actualNics []*computev1.HostnicResource) {
	t.Helper()
	assertSameResources(t, expectedNics, actualNics, OrderByDeviceName,
		func(expected, actual *computev1.HostnicResource) (bool, string) {
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

func assertSameIPAddress(t *testing.T, expectedIPs, actualIPs []*networkv1.IPAddressResource) {
	t.Helper()
	assertSameResources(t, expectedIPs, actualIPs, OrderByIPAddress,
		func(expected, actual *networkv1.IPAddressResource) (bool, string) {
			expected.ResourceId = ""
			expected.Nic = nil
			expected.CreatedAt = ""
			expected.UpdatedAt = ""
			actual.ResourceId = ""
			actual.Nic = nil
			actual.CreatedAt = ""
			actual.UpdatedAt = ""
			return inv_testing.ProtoEqualOrDiff(expected, actual)
		})
}

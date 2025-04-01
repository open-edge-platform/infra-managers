// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
	hmgr_util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

// ####################################
// ######### Helper Messages ##########
// ####################################

var systemInfo1 = &pb.UpdateHostSystemInfoByGUIDRequest{
	HostGuid: hostGUID,
	SystemInfo: &pb.SystemInfo{
		HwInfo: &pb.HWInfo{
			SerialNum: hostSN,
			Cpu: &pb.SystemCPU{
				Cores:   hostCPUCores,
				Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
				Sockets: 2,
				Threads: 20,
				Vendor:  "GenuineIntel",
				Arch:    "x86",
				CpuTopology: &pb.CPUTopology{
					Sockets: []*pb.Socket{
						{
							SocketId: 0,
							CoreGroups: []*pb.CoreGroup{
								{
									CoreType: "P-Cores",
									CoreList: []uint32{0, 1},
								},
								{
									CoreType: "E-Cores",
									CoreList: []uint32{2, 3, 4, 5},
								},
							},
						},
						{
							SocketId: 1,
							CoreGroups: []*pb.CoreGroup{
								{
									CoreType: "P-Cores",
									CoreList: []uint32{0, 1, 2},
								},
								{
									CoreType: "E-Cores",
									CoreList: []uint32{3, 4, 5},
								},
							},
						},
					},
				},
			},
			Memory: &pb.SystemMemory{
				Size: 64,
			},
			Storage: &pb.Storage{},
			Usb:     []*pb.SystemUSB{},
			Network: []*pb.SystemNetwork{},
		},
		BiosInfo: &pb.BiosInfo{
			Version:     "1.0.18",
			ReleaseDate: "09/30/2022",
			Vendor:      "Dell Inc.",
		},
	},
}

// ####################################
// ############ Test Cases ############
// ####################################

func TestHostManagerClient_UpdateHostStatusByHostGuid(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	// Error
	inUp := &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid: hostGUID,
		HostStatus: &pb.HostStatus{
			HostStatus:          pb.HostStatus_RUNNING,
			Details:             hostStatusDetails,
			HumanReadableStatus: hostStatusHumanReadable,
		},
	}
	respGet, err := HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.Error(t, err)
	require.Nil(t, respGet)

	// Error - no JWT
	inUp = &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid: hostGUID,
		HostStatus: &pb.HostStatus{
			HostStatus:          pb.HostStatus_RUNNING,
			Details:             hostStatusDetails,
			HumanReadableStatus: hostStatusHumanReadable,
		},
	}
	respGet, err = HostManagerTestClient.UpdateHostStatusByHostGuid(context.Background(), inUp)
	require.Error(t, err)
	require.Nil(t, respGet)

	// OK
	hostInv := dao.CreateHost(t, tenant1)

	inUp = &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid: hostInv.GetUuid(),
		HostStatus: &pb.HostStatus{
			HostStatus:          pb.HostStatus_RUNNING,
			Details:             hostStatusDetails,
			HumanReadableStatus: hostStatusHumanReadable,
		},
	}
	respGet, err = HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.NoError(t, err)
	require.NotNil(t, respGet)

	// Validate
	host := GetHostbyUUID(t, hostInv.GetUuid())
	assert.Equal(t, hostInv.GetSerialNumber(), host.GetSerialNumber())
	assert.Equal(t, hrm_status.HostStatusRunning.Status, host.GetHostStatus())
	assert.Equal(t, hrm_status.HostStatusRunning.StatusIndicator, host.GetHostStatusIndicator())

	// OK - with Instance created
	osInv := dao.CreateOs(t, tenant1)
	_ = dao.CreateInstance(t, tenant1, hostInv, osInv)

	inUp = &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid: hostInv.GetUuid(),
		HostStatus: &pb.HostStatus{
			HostStatus:          pb.HostStatus_RUNNING,
			Details:             hostStatusDetails,
			HumanReadableStatus: hostStatusHumanReadable,
		},
	}
	respGet, err = HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.NoError(t, err)
	require.NotNil(t, respGet)

	// Validate
	host = GetHostbyUUID(t, hostInv.GetUuid())
	assert.Equal(t, hostInv.GetSerialNumber(), host.GetSerialNumber())
	assert.Equal(t, hrm_status.HostStatusRunning.Status, host.GetHostStatus())
	assert.Equal(t, hrm_status.HostStatusRunning.StatusIndicator, host.GetHostStatusIndicator())

	// Write again, we expect no second write, thus no events for hosts.
	// Use a different client, in order to catch events if any.
	testClient := inv_testing.ClientType("tempTestClient")
	err = inv_testing.CreateClient(
		testClient, inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		[]inv_v1.ResourceKind{inv_v1.ResourceKind_RESOURCE_KIND_HOST}, "")
	require.NoError(t, err)

	eventsChannel := inv_testing.TestClientsEvents[testClient]
	// Drain the channel to ensure no events are there prior the test.
	for len(eventsChannel) > 0 {
		<-eventsChannel
	}

	respGet, err = HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.NoError(t, err)
	require.NotNil(t, respGet)

	// Wait for a few seconds and verify that no events are received in the eventsChannel
	select {
	case event := <-eventsChannel:
		t.Errorf("Unexpected event received: %v", event)
	case <-time.After(3 * time.Second):
		// No events received within the timeout period
	}
}

// General test cases for UpdateHostSystemInfoByGUID RPC.
// In these TCs storage, network and usbs are not considered.
func TestHostManagerClient_UpdateHostSystemInfoByGUID(t *testing.T) { //nolint:funlen // it is a table-driven test
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	// Error
	respGet, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, systemInfo1)
	require.Error(t, err)
	require.Nil(t, respGet)

	// Error - no JWT
	respGet, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(context.Background(), systemInfo1)
	require.Error(t, err)
	require.Nil(t, respGet)

	// OK
	hostInv := dao.CreateHost(t, tenant1)
	systemInfo1.HostGuid = hostInv.GetUuid()

	testcases := map[string]struct {
		in    *pb.UpdateHostSystemInfoByGUIDRequest
		valid bool
	}{
		"Success": {
			in:    systemInfo1,
			valid: true,
		},
		"SuccessBIOSInfoVendorWithComma": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Vendor: "American Megatrends International, LLC.",
					},
				},
			},
			valid: true,
		},
		"FailedInvalidSystemCPU": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   0,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Threads: 0,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
						},
					},
				},
			},
			valid: false,
		},
		"FailedInvalidSystemMemory": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum: "test",
						Memory: &pb.SystemMemory{
							Size: 0,
						},
					},
				},
			},
			valid: false,
		},
		"FailedInvalidBIOSInfoInvalidVersion": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     TooLongString, // not allowed, see .proto rules
						ReleaseDate: "01/31/2022",
						Vendor:      "Any Vendor",
					},
				},
			},
			valid: false,
		},
		"FailedInvalidBIOSInfoInvalidReleaseDate1": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "22/01/2022", // not allowed, see .proto rules
						Vendor:      "Any Vendor",
					},
				},
			},
			valid: false,
		},
		"FailedInvalidBIOSInfoInvalidReleaseDate2": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "22.01.2022", // not allowed, see .proto rules
						Vendor:      "Any Vendor",
					},
				},
			},
			valid: false,
		},
		"FailedInvalidBIOSInfoInvalidReleaseDate3": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostGUID,
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "01/33/2022", // not allowed, see .proto rules
						Vendor:      "Any Vendor",
					},
				},
			},
			valid: false,
		},
		"FailedInvalidBIOSInfoInvalidReleaseDate4": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "13/31/2022", // not allowed, see .proto rules
						Vendor:      "Any Vendor",
					},
				},
			},
			valid: false,
		},
		"FailedBIOSInfoTooLong": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version: "1.0.0aaaaaaaaaaaaadsadasdsadasdadasdasdasdasdasdasdasdasdasdasdadasdasdasdas" +
							"sadasdasdasadfsavdasdasddasdasdasdsadaadasdasdasdasdvadyyyyyyyyyyfdssvadasdasdada",
					},
				},
			},
			valid: false,
		},
		"FailedCPUTopologyEmptySocketList": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   hostCPUCores,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Sockets: 2,
							Threads: 20,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
							CpuTopology: &pb.CPUTopology{
								Sockets: []*pb.Socket{},
							},
						},
					},
				},
			},
			valid: false,
		},
		"FailedCPUTopologyEmptyCoreGroup": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   hostCPUCores,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Sockets: 2,
							Threads: 20,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
							CpuTopology: &pb.CPUTopology{
								Sockets: []*pb.Socket{
									{
										SocketId:   0,
										CoreGroups: []*pb.CoreGroup{},
									},
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		"FailedCPUTopologyEmptyCoreType": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   hostCPUCores,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Sockets: 2,
							Threads: 20,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
							CpuTopology: &pb.CPUTopology{
								Sockets: []*pb.Socket{
									{
										SocketId: 0,
										CoreGroups: []*pb.CoreGroup{
											{
												CoreType: "",
												CoreList: []uint32{1, 2},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		"FailedCPUTopologyEmptyCoreList": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   hostCPUCores,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Sockets: 2,
							Threads: 20,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
							CpuTopology: &pb.CPUTopology{
								Sockets: []*pb.Socket{
									{
										SocketId: 0,
										CoreGroups: []*pb.CoreGroup{
											{
												CoreType: "Type A",
												CoreList: []uint32{},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			valid: false,
		},
		"FailedCPUTopologyDuplicatedCoresInCoreList": {
			in: &pb.UpdateHostSystemInfoByGUIDRequest{
				HostGuid: hostInv.GetUuid(),
				SystemInfo: &pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						Cpu: &pb.SystemCPU{
							Cores:   hostCPUCores,
							Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
							Sockets: 2,
							Threads: 20,
							Vendor:  "GenuineIntel",
							Arch:    "x86",
							CpuTopology: &pb.CPUTopology{
								Sockets: []*pb.Socket{
									{
										SocketId: 0,
										CoreGroups: []*pb.CoreGroup{
											{
												CoreType: "Type A",
												CoreList: []uint32{1, 1},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			valid: false,
		},
	}

	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			respGet, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, tc.in)
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

			if !t.Failed() && tc.valid {
				require.NotNil(t, respGet)

				// validate some fields
				host := GetHostbyUUID(t, tc.in.GetHostGuid())
				assert.Equal(t, tc.in.GetSystemInfo().GetHwInfo().GetSerialNum(), host.GetSerialNumber())
				assert.Equal(t, tc.in.GetSystemInfo().GetHwInfo().GetCpu().GetCores(), host.GetCpuCores())

				hostCPUTopology, marshalErr := hmgr_util.MarshalHostCPUTopology(
					tc.in.GetSystemInfo().GetHwInfo().GetCpu().GetCpuTopology(),
				)
				require.NoError(t, marshalErr)
				assert.Equal(t, hostCPUTopology, host.GetCpuTopology())

				respGet, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, tc.in)
				require.NotNil(t, respGet)
			}
		})
	}
}

//nolint:funlen // this function has a unit test cases in matrix nature, what makes function longer than usual
func TestHostManagerClient_UpdateInstanceStateStatusByHostGUID(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Setting up prerequisites
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()

	// Creating a Host first
	hostInv := dao.CreateHost(t, tenant1)
	require.NotNil(t, hostInv)

	// creating OS resource
	osInv := dao.CreateOs(t, tenant1)
	require.NotNil(t, osInv)

	// creating Instance resource
	instInv := dao.CreateInstance(t, tenant1, hostInv, osInv)
	require.NotNil(t, instInv)

	testcases := map[string]struct {
		in    *pb.UpdateInstanceStateStatusByHostGUIDRequest
		in2   *pb.HostStatus
		valid bool
	}{
		"EmptyHostUUID": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:             "",
				InstanceState:        pb.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus:       pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
				ProviderStatusDetail: "0 of 0 components are running",
			},
			valid: false,
		},
		"InvalidHostUUID": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:       "invalid_Host_UUID",
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
			},
			valid: false,
		},
		"GoodUpdateInstance": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:       hostInv.GetUuid(),
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_ERROR,
			},
			in2: &pb.HostStatus{
				HostStatus: pb.HostStatus_ERROR,
				Details:    "",
			},
			valid: true,
		},
		"GoodUpdateInstance2": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:       hostInv.GetUuid(),
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
			},
			in2: &pb.HostStatus{
				HostStatus: pb.HostStatus_RUNNING,
				Details:    "",
			},
			valid: true,
		},
		"GoodUpdateInstance3": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:       hostInv.GetUuid(),
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
			},
			in2: &pb.HostStatus{
				HostStatus: pb.HostStatus_RUNNING,
				Details:    "",
			},
			valid: true,
		},
		"GoodInstanceStatusDetail": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:             hostInv.GetUuid(),
				ProviderStatusDetail: "5 of 5 components are running",
			},
			in2: &pb.HostStatus{
				HumanReadableStatus: "5 of 5 components are running",
			},
			valid: true,
		},
		"InvalidInstanceStatusDetail": {
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				HostGuid:             hostInv.GetUuid(),
				ProviderStatusDetail: "5 % 5 components are running", // % not allowed, see .proto rules
			},
			in2: &pb.HostStatus{
				HumanReadableStatus: "5 % 5 components are running",
			},
			valid: false,
		},
	}
	for tcname, tc := range testcases {
		t.Run(tcname, func(t *testing.T) {
			timeBeforeUpdate := time.Now().Unix()
			_, err := HostManagerTestClient.UpdateInstanceStateStatusByHostGUID(ctx, tc.in)
			if err != nil {
				if tc.valid {
					t.Errorf("UpdateInstanceStateStatusByHostGUID() failed: %s", err)
					t.FailNow()
				}
			} else {
				if !tc.valid {
					t.Errorf("UpdateInstanceStateStatusByHostGUID() succeeded but should have failed")
					t.FailNow()
				}
			}
			// only get/delete if valid test and hasn't failed otherwise may segfault
			if !t.Failed() && tc.valid {
				// obtaining Instance back to compare
				instInv = GetInstanceByHostUUID(t, hostInv.GetUuid())
				require.NotNil(t, instInv)

				// performing comparison
				assert.Equal(t, tc.in.GetHostGuid(), instInv.GetHost().GetUuid())
				assert.Equal(t, hmgr_util.GetInstanceStatus(tc.in.GetInstanceStatus()).Status, instInv.GetInstanceStatus())
				assert.Equal(t, hmgr_util.GetInstanceStatus(tc.in.GetInstanceStatus()).StatusIndicator,
					instInv.GetInstanceStatusIndicator())
				assert.LessOrEqual(t, uint64(timeBeforeUpdate), instInv.GetInstanceStatusTimestamp())
				assert.Equal(t, tc.in.GetProviderStatusDetail(), instInv.GetInstanceStatusDetail())

				// obtaining Host back to compare
				updHostInv := GetHostbyUUID(t, hostInv.GetUuid())
				require.NotNil(t, updHostInv)

				// performing comparison
				assert.Equal(t, hmgr_util.GetHostStatus(tc.in2.GetHostStatus()).Status, updHostInv.GetHostStatus())
				assert.Equal(t, hmgr_util.GetHostStatus(tc.in2.GetHostStatus()).StatusIndicator,
					updHostInv.GetHostStatusIndicator())
				assert.LessOrEqual(t, uint64(timeBeforeUpdate), updHostInv.GetHostStatusTimestamp())
			}
		})
	}
}

// TestHostManagerClient_TenantIsolation e2e test validating tenant isolation from SBI.
func TestHostManagerClient_TenantIsolation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	// Setting up prerequisites
	ctxT1, cancel := inv_testing.CreateContextWithENJWT(t, tenant1)
	defer cancel()
	ctxT2, cancel := inv_testing.CreateContextWithENJWT(t, tenant2)
	defer cancel()

	hostInvT1 := dao.CreateHost(t, tenant1)
	osInvT1 := dao.CreateOs(t, tenant1)
	_ = dao.CreateInstance(t, tenant1, hostInvT1, osInvT1)
	hostInvT2 := dao.CreateHost(t, tenant2)
	osInvT2 := dao.CreateOs(t, tenant2)
	_ = dao.CreateInstance(t, tenant2, hostInvT2, osInvT2)

	// UpdateHostStatusbyHostGuid
	t.Run("IsolationUpdateHostStatusByHostGUID", func(t *testing.T) {
		inUp := &pb.UpdateHostStatusByHostGuidRequest{
			HostGuid: hostInvT1.GetUuid(),
			HostStatus: &pb.HostStatus{
				HostStatus:          pb.HostStatus_RUNNING,
				Details:             hostStatusDetails,
				HumanReadableStatus: hostStatusHumanReadable,
			},
		}
		resp, err := HostManagerTestClient.UpdateHostStatusByHostGuid(ctxT1, inUp)
		require.NoError(t, err)
		require.NotNil(t, resp)
		resp, err = HostManagerTestClient.UpdateHostStatusByHostGuid(ctxT2, inUp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, resp)
	})

	// UpdateHostSystemInfoByGUID
	t.Run("IsolationUpdateHostSystemInfoByGUID", func(t *testing.T) {
		systemInfo1.HostGuid = hostInvT1.GetUuid()
		resp, err := HostManagerTestClient.UpdateHostSystemInfoByGUID(ctxT1, systemInfo1)
		require.NoError(t, err)
		require.NotNil(t, resp)
		resp, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctxT2, systemInfo1)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, resp)
		systemInfo1.HostGuid = hostGUID
	})

	// UpdateHostStatusByHostGuid
	t.Run("IsolationUpdateHostStatusByHostGuid", func(t *testing.T) {
		inUp := &pb.UpdateInstanceStateStatusByHostGUIDRequest{
			HostGuid:       hostInvT1.GetUuid(),
			InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
			InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
		}
		resp, err := HostManagerTestClient.UpdateInstanceStateStatusByHostGUID(ctxT1, inUp)
		require.NoError(t, err)
		require.NotNil(t, resp)
		resp, err = HostManagerTestClient.UpdateInstanceStateStatusByHostGUID(ctxT2, inUp)
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, resp)
	})

	// We don't enforce Tenancy in the JWT
	t.Run("MissingTenantIDInJWT", func(t *testing.T) {
		inUp := &pb.UpdateHostStatusByHostGuidRequest{
			HostGuid: hostInvT1.GetUuid(),
			HostStatus: &pb.HostStatus{
				HostStatus:          pb.HostStatus_RUNNING,
				Details:             hostStatusDetails,
				HumanReadableStatus: hostStatusHumanReadable,
			},
		}
		missingTenantCtx, cancel := inv_testing.CreateContextWithENJWT(t)
		defer cancel()
		respFail, err := HostManagerTestClient.UpdateHostStatusByHostGuid(missingTenantCtx, inUp)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
		require.Nil(t, respFail)
	})
}

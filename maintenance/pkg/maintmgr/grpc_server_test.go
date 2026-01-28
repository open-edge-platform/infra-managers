// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package maintmgr_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedule_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	mm_testing "github.com/open-edge-platform/infra-managers/maintenance/internal/testing"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/maintmgr"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	inv_utils "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
	om_status "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/status"
)

//nolint:funlen // long function due to matrix-based test
func TestServer_PlatformUpdateStatusErrors(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant1)
	defer cancel()

	os := dao.CreateOs(t, mm_testing.Tenant1)
	mutableOSUpdatePolicy := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant1,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages"),
	)
	immutableOsProfileName := "immutable OS profile name"
	osImageSha256 := inv_testing.GenerateRandomSha256()
	osImageSha2562 := inv_testing.GenerateRandomSha256()
	immutableOs := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = osImageSha256
		os.ProfileName = immutableOsProfileName
		os.ProfileVersion = "1.0.1"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})
	immutableOs2 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Name = "Immutable OS 2"
		os.Sha256 = osImageSha2562
		os.ProfileName = immutableOsProfileName
		os.ProfileVersion = "1.0.1"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})
	immmutableOSUpdatePolicy := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant1,
		inv_testing.OsUpdatePolicyName("Test Immutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyTargetOS(immutableOs2),
	)

	// Host1 has both single and repeated schedule associated to it
	h1 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1.Uuid = uuid.NewString()
	host1 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h1)
	dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host1, os, true,
		func(inst *computev1.InstanceResource) {
			inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
			inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		},
		inv_testing.InstanceOsUpdatePolicy(mutableOSUpdatePolicy),
	)
	sSched1 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched1, host1, nil)
	rSched1 := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched1, host1, nil)

	// Host2 has no schedule associated
	h2 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h2.Uuid = uuid.NewString()
	host2 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h2)
	dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host2, os, true,
		func(inst *computev1.InstanceResource) {
			inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
			inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		}, inv_testing.InstanceOsUpdatePolicy(mutableOSUpdatePolicy),
	)

	// Host3 has no instance associated
	h3 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h3.Uuid = uuid.NewString()
	host3 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h3)

	// Host4 has instance with immutable OS associated
	h4 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h4.Uuid = uuid.NewString()
	host4 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h4)
	dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host4, immutableOs, true,
		func(inst *computev1.InstanceResource) {
			inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
			inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		}, inv_testing.InstanceOsUpdatePolicy(mutableOSUpdatePolicy),
		inv_testing.InstanceOsUpdatePolicy(immmutableOSUpdatePolicy),
	)

	testCases := map[string]struct {
		in          *pb.PlatformUpdateStatusRequest
		expGRCPCode codes.Code
		valid       bool
		expReply    *pb.PlatformUpdateStatusResponse
	}{
		"EmptyGUID": {
			in: &pb.PlatformUpdateStatusRequest{
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			expGRCPCode: codes.InvalidArgument,
			valid:       false,
		},
		"InvalidGUID": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: "invalid GUID",
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			expGRCPCode: codes.InvalidArgument,
			valid:       false,
		},
		"MissingHost": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: uuid.NewString(),
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			expGRCPCode: codes.NotFound,
			valid:       false,
		},
		"MissingInstance": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: host3.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			expGRCPCode: codes.NotFound,
			valid:       false,
		},
		"UpdateStatusNil": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: host1.Uuid,
			},
			expGRCPCode: codes.InvalidArgument,
		},
		"MissingSchedules": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: host2.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			valid: true,
			expReply: &pb.PlatformUpdateStatusResponse{
				UpdateSource: &pb.UpdateSource{
					KernelCommand: mutableOSUpdatePolicy.GetUpdateKernelCommand(),
					CustomRepos:   mutableOSUpdatePolicy.GetUpdateSources(),
				},
				UpdateSchedule:        &pb.UpdateSchedule{},
				InstalledPackages:     mutableOSUpdatePolicy.GetUpdatePackages(),
				OsType:                pb.PlatformUpdateStatusResponse_OSType(os.GetOsType()),
				OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
			},
		},
		"MissingOsProfileUpdateSourceForMutableOsType": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: host1.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			valid: true,
			expReply: &pb.PlatformUpdateStatusResponse{
				UpdateSource: &pb.UpdateSource{
					KernelCommand: mutableOSUpdatePolicy.GetUpdateKernelCommand(),
					CustomRepos:   mutableOSUpdatePolicy.GetUpdateSources(),
				},
				UpdateSchedule:        &pb.UpdateSchedule{},
				InstalledPackages:     mutableOSUpdatePolicy.GetUpdatePackages(),
				OsType:                pb.PlatformUpdateStatusResponse_OSType(os.GetOsType()),
				OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
			},
		},

		"MissingUpdateSourceForImmutableOsType": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: host4.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			valid: true,
			expReply: &pb.PlatformUpdateStatusResponse{
				UpdateSource:      &pb.UpdateSource{},
				UpdateSchedule:    &pb.UpdateSchedule{},
				InstalledPackages: "",
				OsType:            pb.PlatformUpdateStatusResponse_OSType(immutableOs2.GetOsType()),
				OsProfileUpdateSource: &pb.OSProfileUpdateSource{
					ProfileName:    immutableOs2.GetProfileName(),
					ProfileVersion: immutableOs2.GetProfileVersion(),
					OsImageUrl:     immutableOs2.GetImageUrl(),
					OsImageId:      immutableOs2.GetImageId(),
					OsImageSha:     immutableOs2.GetSha256(),
				},
			},
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, tc.in)
			if !tc.valid {
				require.Error(t, err)
				sts, _ := status.FromError(err)
				assert.Equal(t, tc.expGRCPCode, sts.Code())
			} else {
				require.NoErrorf(t, err, errors.ErrorToStringWithDetails(err))
				if eq, diff := inv_testing.ProtoEqualOrDiff(tc.expReply, resp); !eq {
					t.Errorf("Wrong reply: %v", diff)
				}
			}
		})
	}
}

//nolint:funlen // long function due to matrix-based test
func TestServer_PlatformUpdateStatus_Isolation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctxT1, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant1)
	defer cancel()

	ctxT2, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant2)
	defer cancel()

	ctxNoTenantID, cancel := inv_testing.CreateContextWithENJWT(t)
	defer cancel()

	osT1 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true,
		inv_testing.ImageID("3.0.20240710.1000"),
		inv_testing.Sha256(inv_testing.GenerateRandomSha256()),
		inv_testing.ProfileName(inv_testing.GenerateRandomProfileName()),
		inv_testing.OsName(inv_testing.GenerateRandomOsResourceName()),
	)

	osUpdatePolicyT1 := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant1,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy T1"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command T1"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages T1"),
	)

	h1T1 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1T1.Uuid = uuid.NewString()
	host1T1 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h1T1)
	instT1 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host1T1, osT1, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.OsUpdatePolicy = osUpdatePolicyT1
	})
	assert.NotNil(t, instT1)
	sSched1T1 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched1T1, host1T1, nil)

	osT2 := dao.CreateOsWithOpts(t, mm_testing.Tenant2, true,
		inv_testing.ImageID("3.0.20240720.2000"),
		inv_testing.Sha256(inv_testing.GenerateRandomSha256()),
		inv_testing.ProfileName(inv_testing.GenerateRandomProfileName()),
		inv_testing.OsName(inv_testing.GenerateRandomOsResourceName()),
	)
	osUpdatePolicyT2 := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant2,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command T2"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages T2"),
	)
	h1T2 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1T2.TenantId = mm_testing.Tenant2
	h1T2.Uuid = uuid.NewString()
	host1T2 := mm_testing.CreateHost(t, mm_testing.Tenant2, &h1T2)
	instT2 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant2, host1T2, osT2, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.OsUpdatePolicy = osUpdatePolicyT2
	})
	assert.NotNil(t, instT2)
	sSched1T2 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	sSched1T2.TenantId = mm_testing.Tenant2
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant2, &sSched1T2, host1T2, nil)

	osT0 := dao.CreateOsWithOpts(t, mm_testing.DefaultTenantID, true,
		inv_testing.ImageID("3.0.20240721.3000"),
		inv_testing.Sha256(inv_testing.GenerateRandomSha256()),
		inv_testing.ProfileName(inv_testing.GenerateRandomProfileName()),
		inv_testing.OsName(inv_testing.GenerateRandomOsResourceName()),
	)
	osUpdatePolicyT0 := dao.CreateOSUpdatePolicy(
		t, mm_testing.DefaultTenantID,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command T2"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages T2"),
	)
	hT0 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	hT0.TenantId = mm_testing.DefaultTenantID
	hT0.Uuid = uuid.NewString()
	hostT0 := mm_testing.CreateHost(t, mm_testing.DefaultTenantID, &hT0)
	_ = dao.CreateInstanceWithOpts(t, mm_testing.DefaultTenantID, hostT0, osT0, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.OsUpdatePolicy = osUpdatePolicyT0
	})
	sSchedT0 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	sSchedT0.TenantId = mm_testing.DefaultTenantID
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.DefaultTenantID, &sSchedT0, hostT0, nil)

	t.Run("Isolation", func(t *testing.T) {
		resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctxT1, &pb.PlatformUpdateStatusRequest{
			HostGuid: host1T1.Uuid,
			UpdateStatus: &pb.UpdateStatus{
				StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, resp)

		require.NoError(t, err)
		require.NotNil(t, resp)

		resp, err = MaintManagerTestClient.PlatformUpdateStatus(ctxT1, &pb.PlatformUpdateStatusRequest{
			HostGuid: host1T2.Uuid,
			UpdateStatus: &pb.UpdateStatus{
				StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
			},
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.Nil(t, resp)

		resp, err = MaintManagerTestClient.PlatformUpdateStatus(ctxT2, &pb.PlatformUpdateStatusRequest{
			HostGuid: host1T1.Uuid,
			UpdateStatus: &pb.UpdateStatus{
				StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
			},
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
		assert.Nil(t, resp)
	})

	t.Run("DefaultTenant", func(t *testing.T) {
		resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctxNoTenantID, &pb.PlatformUpdateStatusRequest{
			HostGuid: hostT0.Uuid,
			UpdateStatus: &pb.UpdateStatus{
				StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
			},
		})
		require.Error(t, err)
		assert.Nil(t, resp)
	})
}

func Test_DenyRBAC(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	os := dao.CreateOs(t, mm_testing.Tenant1)
	// Host2 has no schedule associated

	h2 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h2.TenantId = mm_testing.Tenant1
	h2.Uuid = uuid.NewString()
	host2 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h2)
	_ = dao.CreateInstance(t, mm_testing.Tenant1, host2, os)

	req := &pb.PlatformUpdateStatusRequest{
		HostGuid: host2.Uuid,
		UpdateStatus: &pb.UpdateStatus{
			StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
		},
	}
	resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, req)
	require.Error(t, err)
	assert.Equal(t, "Request unauthenticated with Bearer", errors.ErrorToStringWithDetails(err))
	require.Nil(t, resp)

	ctx1, cancel1 := inv_testing.CreateContextWithJWT(t, "")
	defer cancel1()

	resp, err = MaintManagerTestClient.PlatformUpdateStatus(ctx1, req)
	require.Error(t, err)
	assert.Equal(t, "Rejected because missing projectID in JWT roles: "+
		"rejected=/maintmgr.v1.MaintmgrService/PlatformUpdateStatus",
		errors.ErrorToStringWithDetails(err))
	require.Nil(t, resp)

	// Use the default Tenant ID
	ctxEn, cancel := inv_testing.CreateContextWithENJWT(t)
	defer cancel()
	resp, err = MaintManagerTestClient.PlatformUpdateStatus(ctxEn, req)
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Nil(t, resp)
}

//nolint:funlen // Test functions are long but necessary to test all the cases.
func TestServer_UpdateEdgeNode(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// This test case emulates the behavior of PUA while doing an update on a Node

	invCli := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	scheduleCache := schedule_cache.NewScheduleCacheClient(invCli)
	hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
	require.NoError(t, err)
	client := invclient.NewInvGrpcClient(invCli, hScheduleCache)
	maintmgr.SetInvGrpcCli(client)

	// Host1 has both single and repeated schedule associated to
	h := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h.TenantId = mm_testing.Tenant1
	h.Uuid = uuid.NewString()
	host := mm_testing.CreateHost(t, mm_testing.Tenant1, &h)
	sSched := schedule_v1.SingleScheduleResource{
		TenantId:       mm_testing.Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		//nolint:gosec // no overflow for some time.
		StartSeconds: uint64(time.Now().Unix()) + 1, // 1 second from now.
	}

	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched, host, nil)
	rSched := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	rSched.TenantId = mm_testing.Tenant1
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched, host, nil)
	time.Sleep(2 * time.Second) // wait for the single schedule to be active

	scheduleCache.LoadAllSchedulesFromInv()

	os := dao.CreateOs(t, mm_testing.Tenant1)
	osUpdatePolicy := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant1,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages"),
	)
	inst := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.RuntimePackages = "packages"
		inst.OsUpdatePolicy = &computev1.OSUpdatePolicyResource{ResourceId: osUpdatePolicy.GetResourceId()}
	})

	// Host and instance start with RUNNING status
	updateStatusTime, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
	require.NoError(t, err)
	require.NotNil(t, inst.GetOsUpdatePolicy())

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant1)
	defer cancel()

	_, err = client.InvClient.Update(ctx, mm_testing.Tenant1, inst.ResourceId, &fieldmaskpb.FieldMask{Paths: []string{
		computev1.InstanceResourceFieldUpdateStatus,
		computev1.InstanceResourceFieldUpdateStatusIndicator,
		computev1.InstanceResourceFieldUpdateStatusTimestamp,
	}}, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Instance{
			Instance: &computev1.InstanceResource{
				UpdateStatus:          mm_status.UpdateStatusUpToDate.Status,
				UpdateStatusIndicator: mm_status.UpdateStatusUpToDate.StatusIndicator,
				UpdateStatusTimestamp: updateStatusTime,
			},
		},
	})

	require.NoError(t, err)

	// TODO validate with list of repeated schedule from inventory
	sSchedNow := &pb.SingleSchedule{
		StartSeconds: sSched.StartSeconds,
	}
	expUpdateResponse := &pb.PlatformUpdateStatusResponse{
		UpdateSchedule: &pb.UpdateSchedule{
			SingleSchedule:    sSchedNow,
			RepeatedSchedule:  mm_testing.MmRepeatedSchedule1[0],
			RepeatedSchedules: mm_testing.MmRepeatedSchedule1,
		},
		UpdateSource: &pb.UpdateSource{
			KernelCommand: osUpdatePolicy.GetUpdateKernelCommand(),
			CustomRepos:   osUpdatePolicy.GetUpdateSources(),
		},
		InstalledPackages:     osUpdatePolicy.GetUpdatePackages(),
		OsType:                pb.PlatformUpdateStatusResponse_OS_TYPE_MUTABLE,
		OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
	}

	// First request with UP_TO_DATE to gather schedules
	// init host and instance statuses in the inventory but leave update status unspecified
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_UNSPECIFIED,
		mm_status.UpdateStatusUnknown,
		expUpdateResponse,
	)

	// The repeated request results in MM updating the inventory with only with the instance's update status
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
		mm_status.UpdateStatusUpToDate,
		expUpdateResponse,
	)

	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADING,
		mm_status.UpdateStatusDownloading,
		expUpdateResponse,
	)

	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADED,
		mm_status.UpdateStatusDownloaded,
		expUpdateResponse,
	)

	// STARTED
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_STARTED,
		mm_status.UpdateStatusInProgress,
		expUpdateResponse,
	)

	// UPDATED
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_UPDATED,
		mm_status.UpdateStatusDone,
		expUpdateResponse,
	)

	// FAILED
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_FAILED,
		mm_status.UpdateStatusFailed,
		expUpdateResponse,
	)

	// FAILED - should neither update the existing OsUpdateRun nor create a new one
	// because the previous status is already FAILED.
	RunPUAUpdateAndAssert(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_FAILED,
		mm_status.UpdateStatusFailed,
		expUpdateResponse,
	)

	// Delete the OsUpdateRun resource created in the previous FAILED status
	// Note: The second FAILED status above does not create a new OsUpdateRun
	require.NoError(t, OSUpdateRunDeleteLatest(t, mm_testing.Tenant1, inst))

	// should not handle untrusted
	// Host and instance start with RUNNING status
	_, err = client.InvClient.Update(ctx, mm_testing.Tenant1, host.ResourceId, &fieldmaskpb.FieldMask{Paths: []string{
		computev1.HostResourceFieldCurrentState,
	}}, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Host{
			Host: &computev1.HostResource{
				CurrentState: computev1.HostState_HOST_STATE_UNTRUSTED,
			},
		},
	})
	require.NoError(t, err)

	_, err = MaintManagerTestClient.PlatformUpdateStatus(ctx, &pb.PlatformUpdateStatusRequest{
		HostGuid: host.Uuid,
		UpdateStatus: &pb.UpdateStatus{
			StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
		},
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

//nolint:funlen // Test functions are long but necessary to test all the cases.
func TestServer_HandleUpdateRunDuringEdgeNodeUpdate(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// This test case emulates the behavior of PUA while doing an update on a Node
	// to test creating and updating OsUpdateRun resource on statuses
	// received from PUA: DOWNLOADING, DOWNLOADED, STARTED, DONE, FAILED

	h := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h.TenantId = mm_testing.Tenant1
	h.Uuid = uuid.NewString()
	host := mm_testing.CreateHost(t, mm_testing.Tenant1, &h)

	os := dao.CreateOs(t, mm_testing.Tenant1)
	osUpdatePolicy := dao.CreateOSUpdatePolicy(
		t, mm_testing.Tenant1,
		inv_testing.OsUpdatePolicyName("Test Mutable OS Update Policy"),
		inv_testing.OSUpdatePolicyTarget(),
		inv_testing.OSUpdatePolicyUpdateSources([]string{}),
		inv_testing.OSUpdatePolicyUpdateKernelCommand("test command"),
		inv_testing.OSUpdatePolicyUpdatePackages("test packages"),
	)
	inst := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host, os, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.RuntimePackages = "runtimepackages"
		inst.OsUpdatePolicy = &computev1.OSUpdatePolicyResource{ResourceId: osUpdatePolicy.GetResourceId()}
	})

	require.NotNil(t, inst)
	require.NotNil(t, inst.GetOsUpdatePolicy())

	expUpdateResponse := &pb.PlatformUpdateStatusResponse{
		UpdateSchedule: &pb.UpdateSchedule{},
		UpdateSource: &pb.UpdateSource{
			KernelCommand: osUpdatePolicy.GetUpdateKernelCommand(),
			CustomRepos:   osUpdatePolicy.GetUpdateSources(),
		},
		InstalledPackages:     osUpdatePolicy.GetUpdatePackages(),
		OsType:                pb.PlatformUpdateStatusResponse_OS_TYPE_MUTABLE,
		OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
	}

	// Status DOWNLOADING creates new OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADING,
		mm_status.UpdateStatusDownloading,
		expUpdateResponse,
	)

	// Status DOWNLOADED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADED,
		mm_status.UpdateStatusDownloaded,
		expUpdateResponse,
	)

	// Status STARTED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_STARTED,
		mm_status.UpdateStatusInProgress,
		expUpdateResponse,
	)

	// Status UPDATED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_UPDATED,
		mm_status.UpdateStatusDone,
		expUpdateResponse,
	)

	// Status DOWNLOADING created new OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADING,
		mm_status.UpdateStatusDownloading,
		expUpdateResponse,
	)

	// Status DOWNLOADED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADED,
		mm_status.UpdateStatusDownloaded,
		expUpdateResponse,
	)

	// Status STARTED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_STARTED,
		mm_status.UpdateStatusInProgress,
		expUpdateResponse,
	)

	// Status FAILED updates the latest OsUpdateRun
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_FAILED,
		mm_status.UpdateStatusFailed,
		expUpdateResponse,
	)

	// FAILED - should neither update the existing OsUpdateRun nor create a new one
	// because the previous status is already FAILED.
	RunPUAUpdateAndTestOsUpRun(
		t,
		mm_testing.Tenant1,
		host, inst,
		pb.UpdateStatus_STATUS_TYPE_FAILED,
		mm_status.UpdateStatusFailed,
		expUpdateResponse,
	)

	// Delete the OsUpdateRun resources created in previous step
	require.NoError(t, OSUpdateRunDeleteLatest(t, mm_testing.Tenant1, inst))
	require.NoError(t, OSUpdateRunDeleteLatest(t, mm_testing.Tenant1, inst))

	// Check that all OsUpdateRun resources for this instance are deleted
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant1)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	runs, err := invclient.GetLatestOSUpdateRunByInstanceID(
		ctx, client, mm_testing.Tenant1, inst.GetResourceId(), invclient.OSUpdateRunAll)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Nil(t, runs)
}

//nolint:funlen // Test functions are long but necessary to test all the cases.
func TestServer_OSUpdateAvailableImmutableOS(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	immutableOsProfileName := "immutable OS profile name"
	osImageSha256 := inv_testing.GenerateRandomSha256()
	osImageSha2562 := inv_testing.GenerateRandomSha256()
	tenantID := mm_testing.Tenant1
	h1 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1.TenantId = tenantID
	h1.Uuid = uuid.NewString()
	host1 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h1)

	h2 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h2.TenantId = tenantID
	h2.Uuid = uuid.NewString()
	host2 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h2)

	runtimePackages := "packages"
	immutableOs := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = osImageSha256
		os.ProfileName = immutableOsProfileName
		os.ImageId = "3.0.20240720.2000"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	// Instance has no Update Status
	inst1 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host1, immutableOs, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.RuntimePackages = runtimePackages
		inst.OsUpdatePolicy = nil
	})

	// Instance has update status already set to UpToDate
	inst2 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host2, immutableOs, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.RuntimePackages = runtimePackages
		inst.UpdateStatus = mm_status.UpdateStatusUpToDate.Status
		inst.UpdateStatusIndicator = mm_status.UpdateStatusUpToDate.StatusIndicator
		timestamp, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)
		inst.UpdateStatusTimestamp = timestamp
		inst.OsUpdatePolicy = nil
	})

	immutableOs2 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Name = "Immutable OS 2"
		os.Sha256 = osImageSha2562
		os.ProfileName = immutableOsProfileName
		os.ImageId = "3.0.20240820.2000"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	expUpdateResponse := &pb.PlatformUpdateStatusResponse{
		UpdateSchedule:        &pb.UpdateSchedule{},
		OsType:                pb.PlatformUpdateStatusResponse_OS_TYPE_IMMUTABLE,
		OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
		UpdateSource:          &pb.UpdateSource{},
	}

	expectedUpdateStatus := mm_status.UpdateStatusUpToDate

	instances := []struct {
		name string
		host *computev1.HostResource
		inst *computev1.InstanceResource
	}{
		{
			name: "Set OsUpdateAvailable together with UpdateStatus ",
			host: host1,
			inst: inst1,
		},
		{
			name: "Set OsUpdateAvailable when UpdateStatus does not require update",
			host: host2,
			inst: inst2,
		},
	}
	for _, tc := range instances {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenantID)
			defer cancel()
			client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

			resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, &pb.PlatformUpdateStatusRequest{
				HostGuid: host1.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType:        pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
					ProfileName:       "immutable OS profile name",
					OsImageId:         "3.0.20250909",
					OsUpdateAvailable: "Edge Microvisor Toolkit 3.0.20250909",
				},
			})
			require.NoErrorf(t, err, errors.ErrorToStringWithDetails(err))
			if eq, diff := inv_testing.ProtoEqualOrDiff(expUpdateResponse, resp); !eq {
				t.Errorf("Wrong response: %v", diff)
			}

			gResp, err := client.Get(ctx, tenantID, inst1.ResourceId)
			require.NoError(t, err)

			instGet := gResp.GetResource().GetInstance()
			require.NotNil(t, instGet)

			require.NotEmpty(t, instGet.GetOsUpdateAvailable())
			require.Equal(t, immutableOs2.GetName(), instGet.GetOsUpdateAvailable())
			assert.Equal(t, expectedUpdateStatus.StatusIndicator, instGet.UpdateStatusIndicator)
			assert.Equal(t, expectedUpdateStatus.Status, instGet.UpdateStatus)
		})
	}
}

func TestServer_OSUpdateAvailableMutableOS(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	mutableOsProfileName := "mutable OS profile name"
	osImageSha256 := inv_testing.GenerateRandomSha256()
	tenantID := mm_testing.Tenant1
	h1 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1.TenantId = tenantID
	h1.Uuid = uuid.NewString()
	host1 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h1)

	h2 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h2.TenantId = tenantID
	h2.Uuid = uuid.NewString()
	host2 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h2)

	mutableOs := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = osImageSha256
		os.ProfileName = mutableOsProfileName
		os.ImageId = "22.04.5"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION
		os.OsType = os_v1.OsType_OS_TYPE_MUTABLE
	})

	// Instance has no Update Status
	inst1 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host1, mutableOs, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.OsUpdatePolicy = nil
	})

	// Instance has update status already set to UpToDate
	inst2 := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, host2, mutableOs, true, func(inst *computev1.InstanceResource) {
		inst.ProvisioningStatus = om_status.ProvisioningStatusDone.Status
		inst.ProvisioningStatusIndicator = om_status.ProvisioningStatusDone.StatusIndicator
		inst.UpdateStatus = mm_status.UpdateStatusUpToDate.Status
		inst.UpdateStatusIndicator = mm_status.UpdateStatusUpToDate.StatusIndicator
		timestamp, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)
		inst.UpdateStatusTimestamp = timestamp
		inst.OsUpdatePolicy = nil
	})

	expUpdateResponse := &pb.PlatformUpdateStatusResponse{
		UpdateSchedule:        &pb.UpdateSchedule{},
		OsType:                pb.PlatformUpdateStatusResponse_OS_TYPE_MUTABLE,
		OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
		UpdateSource:          &pb.UpdateSource{},
	}

	expectedUpdateStatus := mm_status.UpdateStatusUpToDate
	updatePackages := "wget\ncurl"

	instances := []struct {
		name string
		host *computev1.HostResource
		inst *computev1.InstanceResource
		pkgs string
	}{
		{
			name: "Set OsUpdateAvailable together with UpdateStatus",
			host: host1,
			inst: inst1,
		},
		{
			name: "Set OsUpdateAvailable when UpdateStatus does not require update",
			host: host2,
			inst: inst2,
		},
	}
	for _, tc := range instances {
		t.Run(tc.name, func(t *testing.T) {
			t.Helper()

			ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenantID)
			defer cancel()
			client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

			resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, &pb.PlatformUpdateStatusRequest{
				HostGuid: host1.Uuid,
				UpdateStatus: &pb.UpdateStatus{
					StatusType:        pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
					ProfileName:       mutableOsProfileName,
					OsImageId:         "22.04.5",
					OsUpdateAvailable: updatePackages,
				},
			})
			require.NoErrorf(t, err, errors.ErrorToStringWithDetails(err))
			if eq, diff := inv_testing.ProtoEqualOrDiff(expUpdateResponse, resp); !eq {
				t.Errorf("Wrong response: %v", diff)
			}

			gResp, err := client.Get(ctx, tenantID, inst1.ResourceId)
			require.NoError(t, err)

			instGet := gResp.GetResource().GetInstance()
			require.NotNil(t, instGet)

			require.NotEmpty(t, instGet.GetOsUpdateAvailable())
			require.Equal(t, updatePackages, instGet.GetOsUpdateAvailable())
			assert.Equal(t, expectedUpdateStatus.StatusIndicator, instGet.UpdateStatusIndicator)
			assert.Equal(t, expectedUpdateStatus.Status, instGet.UpdateStatus)
		})
	}
}

func OSUpdateRunDeleteLatest(
	t *testing.T,
	tenantID string,
	inst *computev1.InstanceResource,
) error {
	t.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenantID)
	defer cancel()

	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	runGet, err := invclient.GetLatestOSUpdateRunByInstanceID(
		ctx, client, tenantID, inst.GetResourceId(), invclient.OSUpdateRunAll)
	if err != nil {
		return err
	}

	if runGet != nil {
		err = invclient.DeleteOSUpdateRun(ctx, client, tenantID, runGet)
		if err != nil {
			return err
		}

		require.Eventually(t, func() bool {
			_, err := client.Get(ctx, tenantID, runGet.GetResourceId())
			return errors.IsNotFound(err)
		}, 5*time.Second, 50*time.Millisecond)
	}
	return nil
}

func RunPUAUpdateAndAssert(
	t *testing.T,
	tenantID string,
	host *computev1.HostResource,
	inst *computev1.InstanceResource,
	upStatus pb.UpdateStatus_StatusType,
	expUpdateStatus inv_status.ResourceStatus,
	expResponse *pb.PlatformUpdateStatusResponse,
) {
	t.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenantID)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	// TODO validate with list of repeated schedule from inventory
	resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, &pb.PlatformUpdateStatusRequest{
		HostGuid: host.Uuid,
		UpdateStatus: &pb.UpdateStatus{
			StatusType:        upStatus,
			ProfileName:       "immutable OS profile name",
			OsImageId:         "3.0.20250909",
			OsUpdateAvailable: "Edge Microvisor Toolkit 3.0.20250909",
		},
	})
	require.NoErrorf(t, err, errors.ErrorToStringWithDetails(err))
	if eq, diff := inv_testing.ProtoEqualOrDiff(expResponse, resp); !eq {
		t.Errorf("Wrong response: %v", diff)
	}

	gResp, err := client.Get(ctx, tenantID, inst.ResourceId)
	require.NoError(t, err)
	instGet := gResp.GetResource().GetInstance()
	require.NotNil(t, instGet)
	assert.Equal(t, expUpdateStatus.StatusIndicator, instGet.UpdateStatusIndicator)
	assert.Equal(t, expUpdateStatus.Status, instGet.UpdateStatus)
}

func RunPUAUpdateAndTestOsUpRun(
	t *testing.T,
	tenantID string,
	host *computev1.HostResource,
	inst *computev1.InstanceResource,
	upStatus pb.UpdateStatus_StatusType,
	expUpdateStatus inv_status.ResourceStatus,
	expResponse *pb.PlatformUpdateStatusResponse,
) {
	t.Helper()

	ctx, cancel := inv_testing.CreateContextWithENJWT(t, tenantID)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	resp, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, &pb.PlatformUpdateStatusRequest{
		HostGuid: host.Uuid,
		UpdateStatus: &pb.UpdateStatus{
			StatusType:  upStatus,
			ProfileName: "immutable OS profile name",
			OsImageId:   "2.0.0",
		},
	})
	require.NoErrorf(t, err, errors.ErrorToStringWithDetails(err))
	if eq, diff := inv_testing.ProtoEqualOrDiff(expResponse, resp); !eq {
		t.Errorf("Wrong response: %v", diff)
	}

	gResp, err := client.Get(ctx, tenantID, inst.ResourceId)
	require.NoError(t, err)
	instGet := gResp.GetResource().GetInstance()
	require.NotNil(t, instGet)

	tID := instGet.GetTenantId()
	instID := instGet.GetResourceId()

	// Wait briefly to allow OsUpdateRun resource to be created/updated
	time.Sleep(200 * time.Millisecond)
	runGet, err := invclient.GetLatestOSUpdateRunByInstanceID(ctx, client, tID, instID, invclient.OSUpdateRunAll)
	require.NoError(t, err)
	require.NotNil(t, runGet)
	assert.Equal(t, expUpdateStatus.StatusIndicator, runGet.StatusIndicator)
	assert.Equal(t, expUpdateStatus.Status, runGet.Status)
}

func TestGetSanitizeErrorGrpcInterceptor(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := inv_testing.CreateContextWithENJWT(t, mm_testing.Tenant1)
	defer cancel()

	os := dao.CreateOs(t, mm_testing.Tenant1)
	// Host1 has both single and repeated schedule associated to it
	h1 := mm_testing.HostResource1 //nolint:govet // ok to copy locks in test
	h1.TenantId = mm_testing.Tenant1
	h1.Uuid = uuid.NewString()
	host1 := mm_testing.CreateHost(t, mm_testing.Tenant1, &h1)
	dao.CreateInstance(t, mm_testing.Tenant1, host1, os)
	sSched1 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	sSched1.TenantId = mm_testing.Tenant1
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched1, host1, nil)
	rSched1 := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	rSched1.TenantId = mm_testing.Tenant1
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched1, host1, nil)

	testCases := map[string]struct {
		in    *pb.PlatformUpdateStatusRequest
		valid bool
	}{
		"EmptyGUID": {
			in: &pb.PlatformUpdateStatusRequest{
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			valid: false,
		},
		"InvalidGUID": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: "invalid GUID",
				UpdateStatus: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
			},
			valid: false,
		},
		"InvalidGUIDWithAddedProfileDetails": {
			in: &pb.PlatformUpdateStatusRequest{
				HostGuid: "invalid GUID",
				UpdateStatus: &pb.UpdateStatus{
					StatusType:     pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
					ProfileName:    "TestProfileName",
					ProfileVersion: "TestProfileVersion",
					StatusDetail:   "TestStatusDetail",
				},
			},
			valid: false,
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			_, err := MaintManagerTestClient.PlatformUpdateStatus(ctx, tc.in)
			if !tc.valid {
				if err != nil {
					// The format of the actual GPC error
					// rpc error: code = xxx desc = xxx
					desc := strings.Split(err.Error(), "desc = ")
					if strings.Contains(errors.ErrorToStringWithDetails(err), desc[1]) {
						t.Logf("Errors are sanitized properly without datils: %v", err)
					}
				}
			}
		})
	}
}

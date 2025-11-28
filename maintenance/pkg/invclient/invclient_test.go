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
	"google.golang.org/grpc/status"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedule_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	mm_testing "github.com/open-edge-platform/infra-managers/maintenance/internal/testing"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	inv_utils "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

const (
	semverVersion100 = "1.0.0"
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

func TestInvClient_GetRepeatedScheduleByHostResource(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	invCli := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	scheduleCache := schedule_cache.NewScheduleCacheClient(invCli)
	hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
	require.NoError(t, err)
	client := invclient.NewInvGrpcClient(invCli, hScheduleCache)

	// Host with no repeated schedules but single schedules
	site1 := dao.CreateSite(t, mm_testing.Tenant1)
	host1 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site1))
	host1.Site = site1
	sSched1 := mm_testing.SingleSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched1, host1, nil)

	// Host with repeated schedule set for the site
	site2 := dao.CreateSite(t, mm_testing.Tenant1)
	host2 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site2))
	host2.Site = site2
	rSched2 := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched2, nil, site2)
	rSched2b := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched2b, nil, site2)

	// Host with repeated schedule set directly for the host and idirectly for the site
	site3 := dao.CreateSite(t, mm_testing.Tenant1)
	host3 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site3))
	host3.Site = site3
	rSched3 := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched3, host3, nil)
	rSched3b := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched3b, host3, nil)

	host4 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site2))
	host4.Site = site2
	rSched4 := mm_testing.RepeatedSchedule1 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched4, host4, nil)

	rSched5 := mm_testing.RepeatedSchedule3 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched5, host4, nil)

	rSched6 := mm_testing.RepeatedSchedule4 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched6, host4, nil)

	// Create repeated schedules assigned a site that has hosts assigned to it
	rSched7 := mm_testing.RepeatedSchedule5 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched7, nil, site2)

	// Schedule not assigned to a host nor a site
	rSched8 := mm_testing.RepeatedSchedule6 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, &rSched8, nil, nil)

	scheduleCache.LoadAllSchedulesFromInv()

	testCases := map[string]struct {
		in         *computev1.HostResource
		expSScheds []*schedule_v1.RepeatedScheduleResource
	}{
		"MissingHost": {
			in: &computev1.HostResource{ResourceId: "host-12345678"},
		},
		"HostNoSched": {
			in: host1,
		},
		"RepeatedSchedsForSite": {
			in:         host2,
			expSScheds: []*schedule_v1.RepeatedScheduleResource{&rSched2, &rSched2b, &rSched7},
		},
		"RepeatedSchedsForHost": {
			in:         host3,
			expSScheds: []*schedule_v1.RepeatedScheduleResource{&rSched3, &rSched3b},
		},
		"RepeatedSchedsForSiteHost": {
			in:         host4,
			expSScheds: []*schedule_v1.RepeatedScheduleResource{&rSched2, &rSched2b, &rSched4, &rSched5, &rSched6, &rSched7},
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			scheds, err := invclient.ListRepeatedSchedules(ctx, client, mm_testing.Tenant1, tc.in)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expSScheds), len(scheds))
			inv_testing.OrderByResourceID(scheds)
			inv_testing.OrderByResourceID(tc.expSScheds)
			for i, expSsched := range tc.expSScheds {
				assert.Equal(t, expSsched.ResourceId, scheds[i].ResourceId)
			}
		})
	}
}

func TestInvClient_GetSingleScheduleByHostResource(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	invCli := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	scheduleCache := schedule_cache.NewScheduleCacheClient(invCli)
	hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
	client := invclient.NewInvGrpcClient(invCli, hScheduleCache)

	// Host with no single schedules but repeated schedules
	site1 := dao.CreateSite(t, mm_testing.Tenant1)
	host1 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site1))
	host1.Site = site1
	rSched1 := &mm_testing.RepeatedSchedule1
	mm_testing.CreateAndBindRepeatedSchedule(t, mm_testing.Tenant1, rSched1, host1, nil)

	// Host with single schedule set for the site
	site2 := dao.CreateSite(t, mm_testing.Tenant1)
	host2 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site2))
	host2.Site = site2
	sSched2 := mm_testing.SingleSchedule0 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched2, nil, site2)
	sSched2b := mm_testing.SingleSchedule3 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched2b, nil, site2)

	// Host with single schedule set directly for the host and indirectly for the site
	site3 := dao.CreateSite(t, mm_testing.Tenant1)
	host3 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site3))
	host3.Site = site3
	sSched3 := mm_testing.SingleSchedule0 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched3, host3, nil)
	sSched3b := mm_testing.SingleSchedule3 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched3b, host3, nil)

	host4 := dao.CreateHost(t, mm_testing.Tenant1, inv_testing.HostSite(site2))
	host4.Site = site2
	sSched4 := mm_testing.SingleSchedule0 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched4, host4, nil)

	// Create single schedules assigned host that is assigned a site
	sSched5 := mm_testing.SingleSchedule4 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched5, host4, nil)

	sSched6 := mm_testing.SingleSchedule5 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched6, host4, nil)

	// Create single schedules assigned a site that has hosts assigned to it
	sSched7 := mm_testing.SingleSchedule6 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched7, nil, site2)

	// Schedule not assigned to a host nor a site
	sSched8 := mm_testing.SingleSchedule7 //nolint:govet // ok to copy locks in test
	mm_testing.CreateAndBindSingleSchedule(t, mm_testing.Tenant1, &sSched8, nil, nil)

	require.NoError(t, err)
	scheduleCache.LoadAllSchedulesFromInv()

	testCases := map[string]struct {
		in         *computev1.HostResource
		expSScheds []*schedule_v1.SingleScheduleResource
	}{
		"MissingHost": {
			in:         &computev1.HostResource{ResourceId: "host-12345678"},
			expSScheds: []*schedule_v1.SingleScheduleResource{},
		},
		"HostNoSched": {
			in:         host1,
			expSScheds: []*schedule_v1.SingleScheduleResource{},
		},
		"SingleSchedsForSite": {
			in:         host2,
			expSScheds: []*schedule_v1.SingleScheduleResource{&sSched2, &sSched2b, &sSched7},
		},
		"SingleSchedsForHost": {
			in:         host3,
			expSScheds: []*schedule_v1.SingleScheduleResource{&sSched3, &sSched3b},
		},
		"SingleSchedsForSiteHost": {
			in:         host4,
			expSScheds: []*schedule_v1.SingleScheduleResource{&sSched2, &sSched2b, &sSched4, &sSched5, &sSched6, &sSched7},
		},
	}

	for tcName, tc := range testCases {
		t.Run(tcName, func(t *testing.T) {
			scheds, err := invclient.ListSingleSchedules(ctx, client, mm_testing.Tenant1, tc.in)
			require.NoError(t, err)
			assert.Equal(t, len(tc.expSScheds), len(scheds))
			inv_testing.OrderByResourceID(scheds)
			inv_testing.OrderByResourceID(tc.expSScheds)
			for i, expSsched := range tc.expSScheds {
				assert.Equal(t, expSsched.ResourceId, scheds[i].ResourceId)
			}
		})
	}
}

func TestInvClient_GetInstanceResourceByHostGUID(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	// Error - empty HostGUID
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, err := invclient.GetInstanceResourceByHostGUID(ctx, client, mm_testing.Tenant1, "")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// Error - invalid HostGUID
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, err := invclient.GetInstanceResourceByHostGUID(ctx, client, mm_testing.Tenant1, "foobar")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// Error - missing host
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, err := invclient.GetInstanceResourceByHostGUID(ctx, client, mm_testing.Tenant1, "57ed598c-4b94-11ee-806c-3a7c7693aac3")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// OK - gets host
	t.Run("GetHost", func(t *testing.T) {
		osRes := dao.CreateOs(t, mm_testing.Tenant1)
		host := dao.CreateHost(t, mm_testing.Tenant1)
		inst := dao.CreateInstance(t, mm_testing.Tenant1, host, osRes)
		inst.Os = osRes
		inst.Host = host

		getInst, err := invclient.GetInstanceResourceByHostGUID(ctx, client, mm_testing.Tenant1, host.Uuid)
		require.NoError(t, err)
		require.NotNil(t, getInst)
		if eq, diff := inv_testing.ProtoEqualOrDiff(inst, getInst); !eq {
			t.Errorf("Wrong reply: %v", diff)
		}
	})
}

func TestInvClient_UpdateInstance(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	osRes := dao.CreateOs(t, mm_testing.Tenant1)
	newOSRes := dao.CreateOs(t, mm_testing.Tenant1)
	host := dao.CreateHost(t, mm_testing.Tenant1)
	inst := dao.CreateInstance(t, mm_testing.Tenant1, host, osRes)

	// Error - non-existent Instance
	t.Run("ErrorNoInst", func(t *testing.T) {
		plan := invclient.InstanceUpdatePlan{
			Status:  &mm_status.UpdateStatusUpToDate,
			OsResID: newOSRes.GetResourceId(),
		}
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, "inst-12345678", plan)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	t.Run("UpdateInstStatusNotCurrentOS", func(t *testing.T) {
		timeBeforeUpdate := time.Now().Unix()
		plan := invclient.InstanceUpdatePlan{
			Status: &mm_status.UpdateStatusInProgress,
		}
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId, plan)
		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusInProgress.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusInProgress.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
		timeBefore, err := inv_utils.SafeInt64ToUint64(timeBeforeUpdate)
		require.NoError(t, err)
		assert.LessOrEqual(t, timeBefore, updatedInst.GetResource().GetInstance().GetUpdateStatusTimestamp())
	})

	t.Run("UpdateInstStatusAndCurrentOS", func(t *testing.T) {
		beforeUpdateInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.NotEqual(t, newOSRes.GetSha256(), beforeUpdateInst.GetResource().GetInstance().GetOs().GetSha256())
		plan := invclient.InstanceUpdatePlan{
			Status:  &mm_status.UpdateStatusDone,
			OsResID: newOSRes.GetResourceId(),
		}
		err = invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId, plan)
		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusDone.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusDone.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
		assert.Equal(t, newOSRes.GetSha256(), updatedInst.GetResource().GetInstance().GetOs().GetSha256())
	})

	t.Run("UpdateUpdateStatusToRunning", func(t *testing.T) {
		// initial setup of instance status to running and update status to unknown
		plan := invclient.InstanceUpdatePlan{
			Status:  &mm_status.UpdateStatusUnknown,
			OsResID: newOSRes.GetResourceId(),
		}
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId, plan)
		require.NoError(t, err)
		// update only the update status (instance status already set)
		plan.Status = &mm_status.UpdateStatusDone
		err = invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId, plan)
		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusDone.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusDone.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
	})
}

func TestInvClient_GetOSResourceIDByProfileInfo(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	profileName := "profile name"
	osImageID := "some image ID"
	osImageSha256 := inv_testing.GenerateRandomSha256()
	wrongProfileName := "wrong profile name"
	wrongOSImageID := "wrong image ID"

	// Error - empty profileName and osImageId
	t.Run("ErrorNoOSResourceID", func(t *testing.T) {
		_, err := invclient.GetOSResourceIDByProfileInfo(ctx, client, mm_testing.Tenant1, "", "")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// OK - gets OS Resource ID
	t.Run("GetOSResourceID", func(t *testing.T) {
		osRes := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
			os.Sha256 = osImageSha256
			os.ProfileName = profileName
			os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
			os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		})

		getOSResID, err := invclient.GetOSResourceIDByProfileInfo(ctx, client, mm_testing.Tenant1, profileName, osImageID)
		require.NoError(t, err)
		require.NotEmpty(t, getOSResID)
		require.Equal(t, osRes.ResourceId, getOSResID)
	})

	// Error - nonexisting os resource
	tests := []struct {
		name              string
		wantedProfileName string
		wantedOSImageID   string
		errWant           bool
	}{
		{
			name:              "ErrorNoOSResourceWithProfileName",
			wantedProfileName: wrongProfileName,
			wantedOSImageID:   osImageID,
		},
		{
			name:              "ErrorNoOSResourceWithOsImageID",
			wantedProfileName: profileName,
			wantedOSImageID:   wrongOSImageID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
				os.Sha256 = osImageSha256
				os.ProfileName = profileName
				os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
				os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
			})

			_, err := invclient.GetOSResourceIDByProfileInfo(ctx, client, mm_testing.Tenant1,
				tt.wantedProfileName, tt.wantedOSImageID)
			require.Error(t, err)
			sts, _ := status.FromError(err)
			assert.Equal(t, codes.NotFound, sts.Code())
		})
	}
}

//nolint:funlen // long test function due to table driven tests
/* Tests versioning with ImageID. ProfileVersion is currently not used in the OS Resource
 * but is expected to be used in the future.
 * The test checks that the latest OS Resource with a given profile name is returned.
 */
func TestInvClient_GetLatestImmutableOSByProfile(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := t.Context()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	osRes1 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource 1"
		os.ProfileName = "profile name 1"
		os.ImageId = semverVersion100
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	osRes2 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource 2"
		os.ProfileName = "profile name 2"
		os.ImageId = semverVersion100
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource no version"
		os.ProfileName = "profile name 2"
		os.ImageId = ""
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	osRes3 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource 3"
		os.ProfileName = "profile name 3"
		os.ImageId = semverVersion100
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource 4"
		os.ProfileName = "profile name 3"
		os.ImageId = "0.0.1"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "OS Resource with no profile with version"
		os.ProfileName = "profile name 4"
		os.ImageId = ""
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	// Create two immutable OS resources with different versions
	dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Name = "Old OS Resource"
		os.ProfileName = "profile name 6"
		os.ImageId = "1.0.20240101.0000"
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		os.Sha256 = inv_testing.GenerateRandomSha256()
	})

	osRes4 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Name = "New OS Resource "
		os.ProfileName = "profile name 6"
		os.ImageId = "2.0.20240601.0000"
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		os.Sha256 = inv_testing.GenerateRandomSha256()
	})

	tests := []struct {
		name          string
		profileName   string
		expErr        bool
		expErrCode    codes.Code
		expResourceID string
	}{
		{
			name:          "FindOSResWithProfileName1",
			profileName:   "profile name 1",
			expErr:        false,
			expErrCode:    codes.OK,
			expResourceID: osRes1.GetResourceId(),
		},
		{
			name:          "FindOSResWithProfileName2",
			profileName:   "profile name 2",
			expErr:        false,
			expErrCode:    codes.OK,
			expResourceID: osRes2.GetResourceId(),
		},
		{
			name:          "FindOSResWithProfileName3",
			profileName:   "profile name 3",
			expErr:        false,
			expResourceID: osRes3.GetResourceId(),
		},
		{
			name:        "ErrorFindOsResWithoutVersion",
			profileName: "profile name 4",
			expErr:      true,
			expErrCode:  codes.NotFound,
		},
		{
			name:        "ErrorFindNonexistientOSRes",
			profileName: "profile name 5",
			expErr:      true,
			expErrCode:  codes.NotFound,
		},
		{
			name:        "ErrorNoProfileName",
			profileName: "",
			expErr:      true,
			expErrCode:  codes.NotFound,
		},
		{
			name:          "FindNewOSResWithProfileName5",
			profileName:   "profile name 6",
			expErr:        false,
			expResourceID: osRes4.GetResourceId(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getOSRes, err := invclient.GetLatestImmutableOSByProfile(ctx, client, mm_testing.Tenant1, tt.profileName)
			if tt.expErr {
				require.Error(t, err)
				assert.Equal(t, tt.expErrCode, status.Code(err))
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, getOSRes)
				require.Equal(t, tt.expResourceID, getOSRes.GetResourceId())
			}
		})
	}
}

// createOSResourceValidator creates a validation function for OS resource comparison.
func createOSResourceValidator(expected *os_v1.OperatingSystemResource) func(*testing.T, *os_v1.OperatingSystemResource) {
	return func(t *testing.T, osRes *os_v1.OperatingSystemResource) {
		t.Helper()
		assert.Equal(t, expected.GetResourceId(), osRes.GetResourceId())
		assert.Equal(t, expected.GetName(), osRes.GetName())
		assert.Equal(t, expected.GetProfileName(), osRes.GetProfileName())
		assert.Equal(t, expected.GetImageId(), osRes.GetImageId())
		assert.Equal(t, expected.GetProfileVersion(), osRes.GetProfileVersion())
		assert.Equal(t, expected.GetSha256(), osRes.GetSha256())
		assert.Equal(t, expected.GetSecurityFeature(), osRes.GetSecurityFeature())
		assert.Equal(t, expected.GetOsType(), osRes.GetOsType())
	}
}

func TestInvClient_GetOSResourceByID(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	// Create test OS resources
	osRes1 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "Test OS Resource 1"
		os.ProfileName = "test-profile-1"
		os.ImageId = "test-image-1.0.0"
		os.ProfileVersion = "1.0.0"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	osRes2 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "Test OS Resource 2"
		os.ProfileName = "test-profile-2"
		os.ImageId = "test-image-2.0.0"
		os.ProfileVersion = "2.0.0"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
		os.OsType = os_v1.OsType_OS_TYPE_MUTABLE
	})

	tests := []struct {
		name         string
		osResourceID string
		expectError  bool
		expectedCode codes.Code
		validateFunc func(*testing.T, *os_v1.OperatingSystemResource)
	}{
		{
			name:         "SuccessGetImmutableOS",
			osResourceID: osRes1.GetResourceId(),
			expectError:  false,
			validateFunc: createOSResourceValidator(osRes1),
		},
		{
			name:         "SuccessGetMutableOS",
			osResourceID: osRes2.GetResourceId(),
			expectError:  false,
			validateFunc: createOSResourceValidator(osRes2),
		},
		{
			name:         "ErrorNonExistentOSResource",
			osResourceID: "os-nonexistent-12345678",
			expectError:  true,
			expectedCode: codes.NotFound,
		},
		{
			name:         "ErrorEmptyOSResourceID",
			osResourceID: "",
			expectError:  true,
			expectedCode: codes.InvalidArgument,
		},
		{
			name:         "ErrorInvalidOSResourceID",
			osResourceID: "invalid-resource-id",
			expectError:  true,
			expectedCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osRes, err := invclient.GetOSResourceByID(ctx, client, mm_testing.Tenant1, tt.osResourceID)

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, tt.expectedCode, status.Code(err))
				assert.Nil(t, osRes)
			} else {
				require.NoError(t, err)
				require.NotNil(t, osRes)
				if tt.validateFunc != nil {
					tt.validateFunc(t, osRes)
				}
			}
		})
	}
}

func TestInvClient_GetOSResourceByID_ErrorCases(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	// Create a test OS resource for some tests
	osRes1 := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
		os.Sha256 = inv_testing.GenerateRandomSha256()
		os.Name = "Test OS Resource 1"
		os.ProfileName = "test-profile-1"
		os.ImageId = "test-image-1.0.0"
		os.ProfileVersion = "1.0.0"
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
	})

	// Test with different tenant
	t.Run("ErrorWrongTenant", func(t *testing.T) {
		_, err := invclient.GetOSResourceByID(ctx, client, "wrong-tenant-id", osRes1.GetResourceId())
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// Test context timeout
	t.Run("ContextTimeout", func(t *testing.T) {
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		time.Sleep(1 * time.Millisecond) // Ensure context is expired

		_, err := invclient.GetOSResourceByID(timeoutCtx, client, mm_testing.Tenant1, osRes1.GetResourceId())
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.DeadlineExceeded, sts.Code())
	})
}

//nolint:funlen // long test function due to table driven tests
func TestInvClient_CreateOSUpdateRun(t *testing.T) {
	// Helper function to generate OSUpdateRun name with truncated host name
	// to ensure the total name length doesn't exceed 40 bytes
	generateOSUpdateRunName := func(hostName string, timestamp time.Time) string {
		const maxLenHostName = 13
		if len(hostName) > maxLenHostName {
			hostName = hostName[:maxLenHostName] // Truncate to max 13 characters
		}
		return "update-" + hostName + "-" + timestamp.Format("20060102-150405")
	}

	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	// Create resources needed for OSUpdateRun
	osRes := dao.CreateOs(t, mm_testing.Tenant1)
	host := dao.CreateHost(t, mm_testing.Tenant1)
	inst := dao.CreateInstance(t, mm_testing.Tenant1, host, osRes)
	policy := dao.CreateOSUpdatePolicy(t, mm_testing.Tenant1)

	// Keep track of created OSUpdateRun resources for cleanup
	var createdRuns []*computev1.OSUpdateRunResource

	// Cleanup: Delete OSUpdateRun resources before policy/instance/host/os
	// This cleanup is registered after all resource creation, so it runs first (LIFO order)
	t.Cleanup(func() {
		for _, run := range createdRuns {
			if run != nil && run.GetResourceId() != "" {
				_, err := client.Delete(context.Background(), mm_testing.Tenant1, run.GetResourceId())
				require.NoError(t, err)
			}
		}
	})

	t.Run("SuccessCreateOSUpdateRun", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		timestamp := time.Now()
		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:            generateOSUpdateRunName(host.GetName(), timestamp),
			Description:     "Test OS Update Run",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusDownloading,
			StatusDetails:   "Downloading OS update",
			StatusIndicator: mm_status.UpdateStatusDownloading.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		createdRun, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun)
		require.NoError(t, err)
		require.NotNil(t, createdRun)
		createdRuns = append(createdRuns, createdRun)
		assert.NotEmpty(t, createdRun.GetResourceId())
		assert.Equal(t, osUpdateRun.Name, createdRun.GetName())
		assert.Equal(t, osUpdateRun.Description, createdRun.GetDescription())
		assert.Equal(t, osUpdateRun.Status, createdRun.GetStatus())
		assert.Equal(t, osUpdateRun.StatusDetails, createdRun.GetStatusDetails())
		assert.Equal(t, inst.GetResourceId(), createdRun.GetInstance().GetResourceId())
		assert.Equal(t, policy.GetResourceId(), createdRun.GetAppliedPolicy().GetResourceId())
		assert.Equal(t, invclient.SentinelEndTimeUnset, createdRun.GetEndTime())
	})

	t.Run("SuccessCreateOSUpdateRunWithDifferentPolicy", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		// Create a different policy for this test
		policy2 := dao.CreateOSUpdatePolicy(t, mm_testing.Tenant1)

		timestamp := time.Now()
		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:            generateOSUpdateRunName(host.GetName(), timestamp),
			Description:     "Test OS Update Run with Different Policy",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy2.GetResourceId()},
			Status:          mm_status.StatusUpdating,
			StatusIndicator: mm_status.UpdateStatusInProgress.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		createdRun, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun)
		require.NoError(t, err)
		require.NotNil(t, createdRun)
		createdRuns = append(createdRuns, createdRun)
		time.Sleep(10 * time.Millisecond) // Allow event queue to process
		assert.NotEmpty(t, createdRun.GetResourceId())
		assert.Equal(t, policy2.GetResourceId(), createdRun.GetAppliedPolicy().GetResourceId())

		// Clean up this run immediately to avoid foreign key constraint with policy2
		_, err = client.Delete(context.Background(), mm_testing.Tenant1, createdRun.GetResourceId())
		require.NoError(t, err)
		// Remove from createdRuns since we already deleted it
		createdRuns = createdRuns[:len(createdRuns)-1]
	})

	t.Run("SuccessCreateCompletedOSUpdateRun", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		timestamp := time.Now()
		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:            generateOSUpdateRunName(host.GetName(), timestamp),
			Description:     "Completed OS Update Run",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusCompleted,
			StatusIndicator: mm_status.UpdateStatusDone.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         timeNow,
			TenantId:        mm_testing.Tenant1,
		}

		createdRun, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun)
		require.NoError(t, err)
		require.NotNil(t, createdRun)
		createdRuns = append(createdRuns, createdRun)
		time.Sleep(10 * time.Millisecond) // Allow event queue to process
		assert.Equal(t, mm_status.StatusCompleted, createdRun.GetStatus())
		assert.Equal(t, policy.GetResourceId(), createdRun.GetAppliedPolicy().GetResourceId())
		assert.Equal(t, timeNow, createdRun.GetEndTime())
		assert.NotEqual(t, invclient.SentinelEndTimeUnset, createdRun.GetEndTime())
	})

	t.Run("SuccessCreateWithUnsetAppliedPolicy", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		// Test creating OSUpdateRun without applied_policy (verifies it's optional)
		timestamp := time.Now()
		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:        generateOSUpdateRunName(host.GetName(), timestamp),
			Description: "OS Update Run without Policy",
			Instance:    &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			// AppliedPolicy is intentionally not set to verify it's optional
			Status:          mm_status.StatusUpdating,
			StatusIndicator: mm_status.UpdateStatusInProgress.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		run, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun)
		require.NoError(t, err)
		require.NotNil(t, run)
		createdRuns = append(createdRuns, run)
		time.Sleep(10 * time.Millisecond) // Allow event queue to process

		// Verify the run was created without a policy
		assert.Equal(t, osUpdateRun.GetName(), run.GetName())
		assert.Equal(t, osUpdateRun.GetDescription(), run.GetDescription())
		assert.Equal(t, osUpdateRun.GetStatus(), run.GetStatus())
		assert.Nil(t, run.GetAppliedPolicy(), "AppliedPolicy should be nil when not set")
		assert.Equal(t, inst.GetResourceId(), run.GetInstance().GetResourceId())
	})

	t.Run("SuccessCreateMultipleRunsWithSamePolicy", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		// Create first OSUpdateRun with the shared policy
		timestamp1 := time.Now()
		osUpdateRun1 := &computev1.OSUpdateRunResource{
			Name:            generateOSUpdateRunName(host.GetName(), timestamp1) + "-1",
			Description:     "First OS Update Run with Shared Policy",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusDownloading,
			StatusIndicator: mm_status.UpdateStatusDownloading.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		run1, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun1)
		require.NoError(t, err)
		require.NotNil(t, run1)
		createdRuns = append(createdRuns, run1)
		time.Sleep(10 * time.Millisecond) // Allow event queue to process
		assert.Equal(t, policy.GetResourceId(), run1.GetAppliedPolicy().GetResourceId())

		// Create second OSUpdateRun with the same shared policy
		timestamp2 := time.Now()
		osUpdateRun2 := &computev1.OSUpdateRunResource{
			Name:            generateOSUpdateRunName(host.GetName(), timestamp2) + "-2",
			Description:     "Second OS Update Run with Shared Policy",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusUpdating,
			StatusIndicator: mm_status.UpdateStatusInProgress.StatusIndicator,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		run2, err := invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun2)
		require.NoError(t, err)
		require.NotNil(t, run2)
		createdRuns = append(createdRuns, run2)
		time.Sleep(10 * time.Millisecond) // Allow event queue to process

		// Verify both runs share the same policy
		assert.Equal(t, policy.GetResourceId(), run2.GetAppliedPolicy().GetResourceId())
		assert.Equal(t, run1.GetAppliedPolicy().GetResourceId(), run2.GetAppliedPolicy().GetResourceId())

		// Verify they have different resource IDs and names
		assert.NotEqual(t, run1.GetResourceId(), run2.GetResourceId())
		assert.NotEqual(t, run1.GetName(), run2.GetName())
	})

	t.Run("ErrorCreateWithInvalidInstance", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:            "update-invalid-" + time.Now().Format("20060102-150405"),
			Description:     "Test OS Update Run",
			Instance:        &computev1.InstanceResource{ResourceId: "inst-nonexistent"},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusDownloading,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		_, err = invclient.CreateOSUpdateRun(ctx, client, mm_testing.Tenant1, osUpdateRun)
		require.Error(t, err)
	})

	t.Run("ErrorCreateWithWrongTenant", func(t *testing.T) {
		timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
		require.NoError(t, err)

		osUpdateRun := &computev1.OSUpdateRunResource{
			Name:            "update-" + host.GetName() + "-" + time.Now().Format("20060102-150405"),
			Description:     "Test OS Update Run",
			Instance:        &computev1.InstanceResource{ResourceId: inst.GetResourceId()},
			AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
			Status:          mm_status.StatusDownloading,
			StatusTimestamp: timeNow,
			StartTime:       timeNow,
			EndTime:         invclient.SentinelEndTimeUnset,
			TenantId:        mm_testing.Tenant1,
		}

		_, err = invclient.CreateOSUpdateRun(ctx, client, "wrong-tenant-id", osUpdateRun)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.InvalidArgument, sts.Code())
	})
}

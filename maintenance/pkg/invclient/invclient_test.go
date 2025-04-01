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
		inst.DesiredOs = osRes
		inst.CurrentOs = osRes
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
	host := dao.CreateHost(t, mm_testing.Tenant1)
	inst := dao.CreateInstance(t, mm_testing.Tenant1, host, osRes)
	newOSRes := dao.CreateOs(t, mm_testing.Tenant1)

	// Error - non-existent Instance
	t.Run("ErrorNoInst", func(t *testing.T) {
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, "inst-12345678",
			mm_status.UpdateStatusUpToDate, "", newOSRes.GetResourceId())
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.Run("UpdateInstStatusNotCurrentOS", func(t *testing.T) {
		timeBeforeUpdate := time.Now().Unix()
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId,
			mm_status.UpdateStatusInProgress, "", "")

		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusInProgress.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusInProgress.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
		assert.Equal(t, "", updatedInst.GetResource().GetInstance().GetUpdateStatusDetail())

		timeBefore, err := inv_utils.SafeInt64ToUint64(timeBeforeUpdate)
		require.NoError(t, err)
		assert.LessOrEqual(t, timeBefore, updatedInst.GetResource().GetInstance().GetUpdateStatusTimestamp())
	})

	t.Run("UpdateInstStatusAndCurrentOS", func(t *testing.T) {
		beforeUpdateInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.NotEqual(t, newOSRes.GetSha256(), beforeUpdateInst.GetResource().GetInstance().GetCurrentOs().GetSha256())
		err = invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId,
			mm_status.UpdateStatusDone, "some update status detail", newOSRes.GetResourceId())
		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusDone.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusDone.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
		assert.Equal(t, "some update status detail", updatedInst.GetResource().GetInstance().GetUpdateStatusDetail())
		assert.Equal(t, newOSRes.GetSha256(), updatedInst.GetResource().GetInstance().GetCurrentOs().GetSha256())
		assert.NotEqual(t, newOSRes.GetSha256(), updatedInst.GetResource().GetInstance().GetDesiredOs().GetSha256())
	})

	t.Run("UpdateUpdateStatusToRunning", func(t *testing.T) {
		// initial setup of instance status to running and update status to unknown
		err := invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId,
			mm_status.UpdateStatusUnknown, "some update status detail", newOSRes.GetResourceId())
		require.NoError(t, err)
		// setup only the update status as instance status is already set to running
		err = invclient.UpdateInstance(ctx, client, mm_testing.Tenant1, inst.ResourceId,
			mm_status.UpdateStatusDone, "some update status detail", newOSRes.GetResourceId())
		require.NoError(t, err)
		updatedInst, err := client.Get(ctx, mm_testing.Tenant1, inst.ResourceId)
		require.NoError(t, err)
		assert.Equal(t, mm_status.UpdateStatusDone.Status, updatedInst.GetResource().GetInstance().GetUpdateStatus())
		assert.Equal(t, mm_status.UpdateStatusDone.StatusIndicator,
			updatedInst.GetResource().GetInstance().GetUpdateStatusIndicator())
		assert.Equal(t, "some update status detail", updatedInst.GetResource().GetInstance().GetUpdateStatusDetail())
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

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

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	telemetry_testing "github.com/open-edge-platform/infra-managers/telemetry/internal/testing"
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

func TestEventsWatcher(t *testing.T) {
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient

	group := inv_testing.CreateTelemetryGroupLogs(t, true)
	site := inv_testing.CreateSite(t, nil, nil)
	inv_testing.CreateTelemetryProfile(t, nil, site, nil, group, true)

	select {
	case ev, ok := <-telemetryInvClient.Watcher:
		require.True(t, ok, "No events received")
		assert.Equal(t, inv_v1.SubscribeEventsResponse_EVENT_KIND_CREATED, ev.Event.EventKind, "Wrong event kind")
		expectedKind, err := util.GetResourceKindFromResourceID(ev.Event.ResourceId)
		require.NoError(t, err, "resource manager did receive a strange event")
		require.Equal(t, expectedKind, inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE, "Wrong resource kind")
	case <-time.After(1 * time.Second):
		// Timeout to avoid waiting events indefinitely
		t.Fatalf("No events received within timeout")
	}
}

func TestListTelemetryProfilesByHostAndInstanceID(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	telemetry_testing.CreateTelemetryClientForTesting(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profilesInstWithSite := make([]*telemetryv1.TelemetryProfile, 0, 2)

	telClient := telemetry_testing.TelemetryClient
	group := dao.CreateTelemetryGroupLogs(t, tenant1, true)

	region := dao.CreateRegion(t, tenant1)
	profile0 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(region), group, true)
	profile0.Group = group
	profile0.Relation = &telemetryv1.TelemetryProfile_Region{
		Region: region,
	}
	profilesInstWithSite = append(profilesInstWithSite, profile0)

	site := dao.CreateSite(t, tenant1, inv_testing.SiteRegion(region))
	hostNoSite := dao.CreateHost(t, tenant1)
	hostWithSite := dao.CreateHost(t, tenant1, inv_testing.HostSite(site))
	hostWithSite.Site = site
	os1 := dao.CreateOs(t, tenant1)
	instNoSite := dao.CreateInstance(t, tenant1, hostNoSite, os1)
	instWithSite := dao.CreateInstance(t, tenant1, hostWithSite, os1)

	telClient.TelemetryCache.LoadAllTelemetryProfiles()

	// Instance without site has no Telemetry Profiles
	telClient.TelemetryCache.LoadAllTelemetryProfiles()
	profiles, err := telClient.ListTelemetryProfilesByHostAndInstanceID(
		ctx, tenant1, hostNoSite.GetResourceId(), instNoSite.GetResourceId())
	require.NoError(t, err)
	require.Empty(t, profiles)

	profile1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(instNoSite), group, true)
	profile1.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: instNoSite,
	}
	profile1.Group = group

	// some other profiles that shouldn't be returned
	profile2 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), group, true)
	profile2.Group = group
	profile2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: site,
	}
	profilesInstWithSite = append(profilesInstWithSite, profile2)

	telClient.TelemetryCache.LoadAllTelemetryProfiles()

	// Instance without site now has a single Telemetry Profile
	profiles, err = telClient.ListTelemetryProfilesByHostAndInstanceID(
		ctx, tenant1, hostNoSite.GetResourceId(), instNoSite.GetResourceId())
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	if eq, diff := inv_testing.ProtoEqualOrDiff(profile1, profiles[0]); !eq {
		t.Errorf("TestListTelemetryProfilesByInstance() data not equal: %v", diff)
	}

	// Instance with site has single Telemetry Profile inherited by Site and region
	profiles, err = telClient.ListTelemetryProfilesByHostAndInstanceID(
		ctx, tenant1, hostWithSite.GetResourceId(), instWithSite.GetResourceId())
	require.NoError(t, err)
	require.Len(t, profiles, 2)
	inv_testing.OrderByResourceID(profiles)
	inv_testing.OrderByResourceID(profilesInstWithSite)
	for i, expProfile := range profilesInstWithSite {
		if eq, diff := inv_testing.ProtoEqualOrDiff(expProfile, profiles[i]); !eq {
			t.Errorf("TestListTelemetryProfilesByInstance() data not equal: %v", diff)
		}
	}
}

func Test_GetInstanceResourceByHostUUID(t *testing.T) {
	telemetry_testing.CreateTelemetryClientForTesting(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	telClient := telemetry_testing.TelemetryClient

	// Error - empty HostUUID
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, _, err := telClient.GetHostAndInstanceIDResourceByHostUUID(ctx, tenant1, "")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// Error - invalid HostUUID
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, _, err := telClient.GetHostAndInstanceIDResourceByHostUUID(ctx, tenant1, "foobar")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// Error - missing host
	t.Run("ErrorNoInstance", func(t *testing.T) {
		_, _, err := telClient.GetHostAndInstanceIDResourceByHostUUID(ctx, tenant1, "57ed598c-4b94-11ee-806c-3a7c7693aac3")
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	// OK - gets host
	t.Run("GetHost", func(t *testing.T) {
		dao := inv_testing.NewInvResourceDAOOrFail(t)
		osRes := dao.CreateOs(t, tenant1)
		host := dao.CreateHost(t, tenant1)
		inst := dao.CreateInstance(t, tenant1, host, osRes)
		inst.Os = osRes
		inst.Host = host

		hostID, instID, err := telClient.GetHostAndInstanceIDResourceByHostUUID(ctx, tenant1, host.Uuid)
		require.NoError(t, err)
		require.NotNil(t, host)
		require.NotNil(t, instID)
		assert.Equal(t, host.GetResourceId(), hostID)
		assert.Equal(t, inst.GetResourceId(), instID)
	})
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package invclient_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/mennanov/fmutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

const (
	tenant1 = "11111111-1111-1111-1111-111111111111"
	tenant2 = "22222222-2222-2222-2222-222222222222"
)

func TestNewTelemetryCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("Fail to create invclient", func(t *testing.T) {
		scheduleCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
			invclient.WithInventoryAddress(""),
		)
		assert.Nil(t, scheduleCache)
		assert.Error(t, err)
	})

	t.Run("Success", func(t *testing.T) {
		telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
			invclient.WithInventoryAddress("bufconn"),
			invclient.WithDialOption(
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
			),
			invclient.WithEnableTracing(false),
		)
		assert.NotNil(t, telemetryCache)
		assert.NoError(t, err)
		telemetryCache.Stop()
	})

	t.Run("Success - load at start", func(t *testing.T) {
		dao := inv_testing.NewInvResourceDAOOrFail(t)
		os := dao.CreateOs(t, tenant1)
		inst := dao.CreateInstance(t, tenant1, nil, os)
		site := dao.CreateSite(t, tenant1)
		region := dao.CreateRegion(t, tenant1)
		tgLog := dao.CreateTelemetryGroupLogs(t, tenant1, true)
		tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
		dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst), tgLog, true)
		dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), tgMetric, true)
		dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(region), tgMetric, true)

		telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
			invclient.WithInventoryAddress("bufconn"),
			invclient.WithDialOption(
				grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
			),
		)
		assert.NotNil(t, telemetryCache)
		assert.NoError(t, err)
		defer telemetryCache.Stop()

		telProfiles := telemetryCache.ListTelemetryProfileByRelation(tenant1, inst.ResourceId)
		require.NotEmpty(t, telProfiles)
		assert.Len(t, telProfiles, 1)

		telProfiles = telemetryCache.ListTelemetryProfileByRelation(tenant1, site.ResourceId)
		require.NotEmpty(t, telProfiles)
		assert.Len(t, telProfiles, 1)

		telProfiles = telemetryCache.ListTelemetryProfileByRelation(tenant1, region.ResourceId)
		require.NotEmpty(t, telProfiles)
		assert.Len(t, telProfiles, 1)
	})
}

func TestTelemetryCache(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	// faster refresh rate than default to test it quickly
	invclient.PeriodicCacheRefresh = 5 * time.Second

	telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
		invclient.WithInventoryAddress("bufconn"),
		invclient.WithDialOption(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
		),
	)
	assert.NotNil(t, telemetryCache)
	assert.NoError(t, err)
	defer telemetryCache.Stop()

	siteT1 := dao.CreateSite(t, tenant1)
	siteT2 := dao.CreateSite(t, tenant2)
	tgMetricT1 := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tgMetricT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	tpSiteT1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(siteT1), tgMetricT1, false)
	tpSiteT1.Group = tgMetricT1
	tpSiteT1.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT1,
	}
	tpSiteT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgMetricT2, false)
	tpSiteT2.Group = tgMetricT2
	tpSiteT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}

	// Wait cache to properly load
	time.Sleep(100 * time.Millisecond)

	assertSameTpInCache(t, tenant1, telemetryCache, siteT1.ResourceId, tpSiteT1)
	// Assert isolation
	emptyCached := telemetryCache.ListTelemetryProfileByRelation(tenant1, siteT2.ResourceId)
	require.Len(t, emptyCached, 0)
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.ResourceId, tpSiteT2)
	// Assert isolation
	emptyCached = telemetryCache.ListTelemetryProfileByRelation(tenant2, siteT1.ResourceId)
	require.Len(t, emptyCached, 0)

	// wait for periodic refresh + a small delta just to ensure cache renewal is completed
	time.Sleep(invclient.PeriodicCacheRefresh + 100*time.Millisecond)

	// Validate that cache is still coherent after periodic refresh
	assertSameTpInCache(t, tenant1, telemetryCache, siteT1.ResourceId, tpSiteT1)
	// Assert isolation
	emptyCached = telemetryCache.ListTelemetryProfileByRelation(tenant1, siteT2.ResourceId)
	require.Len(t, emptyCached, 0)
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.ResourceId, tpSiteT2)
	// Assert isolation
	emptyCached = telemetryCache.ListTelemetryProfileByRelation(tenant2, siteT1.ResourceId)
	require.Len(t, emptyCached, 0)

	dao.DeleteResource(t, tenant1, tpSiteT1.GetResourceId())
	assert.EventuallyWithT(
		t,
		func(collect *assert.CollectT) {
			cachedTp := telemetryCache.ListTelemetryProfileByRelation(tenant1, siteT1.ResourceId)
			require.Empty(collect, cachedTp)
		},
		invclient.PeriodicCacheRefresh,
		time.Second)
	// Other tenant data still in cache
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.ResourceId, tpSiteT2)

	// Delete other tenant data
	dao.DeleteResource(t, tenant2, tpSiteT2.GetResourceId())
	assert.EventuallyWithT(
		t,
		func(collect *assert.CollectT) {
			cachedTp := telemetryCache.ListTelemetryProfileByRelation(tenant2, siteT2.ResourceId)
			require.Empty(collect, cachedTp)
		},
		invclient.PeriodicCacheRefresh,
		time.Second)
}

func Test_TelemetryCache_LoadAllFromInventory(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a telemetry cache with the testing client
	invclient.BatchSize = 20
	telemetryCache := invclient.NewTelemetryCacheClient(
		inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient(),
	)
	nTelProf := int(invclient.BatchSize + invclient.BatchSize/2)

	// Keep one telemetry profile to check content of the cache
	siteT1 := dao.CreateSite(t, tenant1)
	siteT2 := dao.CreateSite(t, tenant2)
	tgLogT1 := dao.CreateTelemetryGroupLogs(t, tenant1, true)
	tgLogT2 := dao.CreateTelemetryGroupLogs(t, tenant2, true)
	telProfT1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(siteT1), tgLogT1, true)
	telProfT1.Group = tgLogT1
	telProfT1.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT1,
	}
	telProfT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgLogT2, true)
	telProfT2.Group = tgLogT2
	telProfT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}
	for i := 1; i < nTelProf; i++ {
		dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(siteT1), tgLogT1, true)
		dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgLogT2, true)
	}

	telemetryCache.LoadAllTelemetryProfiles()
	allTPs := telemetryCache.TestGetAllTelemetryProfiles()
	assert.Equal(t, nTelProf*2, len(allTPs))
	foundTPs := 0
	for _, tp := range allTPs {
		if tp.GetResourceId() == telProfT1.GetResourceId() {
			if eq, diff := inv_testing.ProtoEqualOrDiff(telProfT1, tp); !eq {
				t.Errorf("wrong telemetry profile in cache: %v", diff)
			} else {
				foundTPs++
			}
		}
		if tp.GetResourceId() == telProfT2.GetResourceId() {
			if eq, diff := inv_testing.ProtoEqualOrDiff(telProfT2, tp); !eq {
				t.Errorf("wrong telemetry profile in cache: %v", diff)
			} else {
				foundTPs++
			}
		}
	}
	assert.Equalf(t, 2, foundTPs, "Expected telemetry profiles not found in cache")
}

func Test_TelemetryCache_ListTelemetryProfileByRelation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a schedule cache with the testing client
	telemetryCache := invclient.NewTelemetryCacheClient(
		inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient(),
	)
	os := dao.CreateOs(t, tenant1)
	inst1 := dao.CreateInstance(t, tenant1, nil, os)
	inst2 := dao.CreateInstance(t, tenant1, nil, os)
	site := dao.CreateSite(t, tenant1)
	region := dao.CreateRegion(t, tenant1)
	tgLog := dao.CreateTelemetryGroupLogs(t, tenant1, true)
	tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tpInst1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst1), tgLog, true)
	tpInst1.Group = tgLog
	tpInst1.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: inst1,
	}
	tpInst21 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst2), tgLog, true)
	tpInst21.Group = tgLog
	tpInst21.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: inst2,
	}
	tpInst22 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst2), tgLog, true)
	tpInst22.Group = tgLog
	tpInst22.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: inst2,
	}
	tpSite := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), tgMetric, true)
	tpSite.Group = tgMetric
	tpSite.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: site,
	}
	tpRegion := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(region), tgMetric, true)
	tpRegion.Group = tgMetric
	tpRegion.Relation = &telemetryv1.TelemetryProfile_Region{
		Region: region,
	}
	telemetryCache.LoadAllTelemetryProfiles()

	testCases := map[string]struct {
		resID string
		exp   []*telemetryv1.TelemetryProfile
	}{
		"ValidTelInst1": {
			resID: inst1.ResourceId,
			exp:   []*telemetryv1.TelemetryProfile{tpInst1},
		},
		"ValidTelInst2": {
			resID: inst2.ResourceId,
			exp:   []*telemetryv1.TelemetryProfile{tpInst21, tpInst22},
		},
		"ValidTelSite": {
			resID: site.ResourceId,
			exp:   []*telemetryv1.TelemetryProfile{tpSite},
		},
		"ValidTelRegion": {
			resID: region.ResourceId,
			exp:   []*telemetryv1.TelemetryProfile{tpRegion},
		},
		"EmptyID": {
			resID: "",
		},
		"InvalidID": {
			resID: "qwe",
		},
	}

	for tName, tc := range testCases {
		t.Run(tName, func(t *testing.T) {
			res := telemetryCache.ListTelemetryProfileByRelation(tenant1, tc.resID)
			assert.Len(t, tc.exp, len(res))
			inv_testing.OrderByResourceID(tc.exp)
			inv_testing.OrderByResourceID(res)
			for i := 0; i < len(tc.exp); i++ {
				if eq, diff := inv_testing.ProtoEqualOrDiff(tc.exp[i], res[i]); !eq {
					t.Errorf("wrong telemetry profile in cache: %v", diff)
				}
			}
		})
	}
}

func Test_TelemetryCache_UpdateTelemetryGroups(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a schedule cache with the testing client
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
		invclient.WithInventoryAddress("bufconn"),
		invclient.WithDialOption(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
		),
	)
	assert.NotNil(t, telemetryCache)
	assert.NoError(t, err)
	defer telemetryCache.Stop()

	os := dao.CreateOs(t, tenant1)
	inst := dao.CreateInstance(t, tenant1, nil, os)
	site := dao.CreateSite(t, tenant1)
	region := dao.CreateRegion(t, tenant1)
	tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tgMetric2 := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tpInst := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst), tgMetric, true)
	tpInst.Group = tgMetric
	tpInst.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: inst,
	}
	tpSite := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), tgMetric, true)
	tpSite.Group = tgMetric
	tpSite.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: site,
	}
	tpRegion := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(region), tgMetric, true)
	tpRegion.Group = tgMetric
	tpRegion.Relation = &telemetryv1.TelemetryProfile_Region{
		Region: region,
	}
	// Create just one TP for other tenant for validating isolation
	siteT2 := dao.CreateSite(t, tenant2)
	tgMetricT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	telProfT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgMetricT2, true)
	telProfT2.Group = tgMetricT2
	telProfT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}

	invClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	updateTgReq := &inv_v1.Resource{
		Resource: &inv_v1.Resource_TelemetryGroup{
			TelemetryGroup: &telemetryv1.TelemetryGroupResource{
				Name: "Updated Name",
			},
		},
	}

	updatedRes, err := invClient.Update(
		ctx,
		tenant1,
		tgMetric.ResourceId,
		&fieldmaskpb.FieldMask{Paths: []string{telemetryv1.TelemetryGroupResourceFieldName}},
		updateTgReq,
	)
	require.NoError(t, err)
	tgMetric.Name = "Updated Name"
	tgMetric.UpdatedAt = updatedRes.GetTelemetryGroup().GetUpdatedAt()

	time.Sleep(200 * time.Millisecond)

	assertSameTpInCache(t, tenant1, telemetryCache, inst.ResourceId, tpInst)
	assertSameTpInCache(t, tenant1, telemetryCache, site.ResourceId, tpSite)
	assertSameTpInCache(t, tenant1, telemetryCache, region.ResourceId, tpRegion)
	// Ensure isolation and no modification to other tenant cache
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.GetResourceId(), telProfT2)

	updateTgReq2 := &inv_v1.Resource{
		Resource: &inv_v1.Resource_TelemetryGroup{
			TelemetryGroup: &telemetryv1.TelemetryGroupResource{
				Name: "Updated Name2",
			},
		},
	}

	// Update OTHER
	_, err = invClient.Update(
		ctx,
		tenant1,
		tgMetric2.ResourceId,
		&fieldmaskpb.FieldMask{Paths: []string{telemetryv1.TelemetryGroupResourceFieldName}},
		updateTgReq2,
	)
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assertSameTpInCache(t, tenant1, telemetryCache, inst.ResourceId, tpInst)
	assertSameTpInCache(t, tenant1, telemetryCache, site.ResourceId, tpSite)
	assertSameTpInCache(t, tenant1, telemetryCache, region.ResourceId, tpRegion)
	// Ensure isolation and no modification to other tenant cache
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.GetResourceId(), telProfT2)
}

func Test_TelemetryCache_UpdateTelemetryProfilesRelation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a schedule cache with the testing client
	ctx, cancel := context.WithTimeout(context.Background(), 1500000*time.Second)
	defer cancel()

	telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
		invclient.WithInventoryAddress("bufconn"),
		invclient.WithDialOption(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
		),
	)
	require.NotNil(t, telemetryCache)
	require.NoError(t, err)
	defer telemetryCache.Stop()

	os := dao.CreateOs(t, tenant1)
	inst := dao.CreateInstance(t, tenant1, nil, os)
	inst2 := dao.CreateInstance(t, tenant1, nil, os)
	site := dao.CreateSite(t, tenant1)
	region := dao.CreateRegion(t, tenant1)
	tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	telProf := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst), tgMetric, true)
	// Create just one TP for other tenant for validating isolation
	siteT2 := dao.CreateSite(t, tenant2)

	tgMetricT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	telProfT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgMetricT2, true)
	telProfT2.Group = tgMetricT2
	telProfT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}
	invClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	testCases := map[string]struct {
		tpResID            string
		upTelemetryProfile *telemetryv1.TelemetryProfile
		relationResID      string
		fm                 []string
		exp                *telemetryv1.TelemetryProfile
	}{
		"FromInstanceToRegion": {
			upTelemetryProfile: &telemetryv1.TelemetryProfile{
				Relation: &telemetryv1.TelemetryProfile_Region{Region: region},
			},
			fm:            []string{telemetryv1.TelemetryProfileEdgeRegion},
			relationResID: region.ResourceId,
		},
		"FromRegionToSite": {
			upTelemetryProfile: &telemetryv1.TelemetryProfile{
				Relation: &telemetryv1.TelemetryProfile_Site{Site: site},
			},
			fm:            []string{telemetryv1.TelemetryProfileEdgeSite},
			relationResID: site.ResourceId,
		},
		"FromSiteToInstance": {
			upTelemetryProfile: &telemetryv1.TelemetryProfile{
				Relation: &telemetryv1.TelemetryProfile_Instance{Instance: inst2},
			},
			fm:            []string{telemetryv1.TelemetryProfileEdgeInstance},
			relationResID: inst2.ResourceId,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			updateTpReq := &inv_v1.Resource{Resource: &inv_v1.Resource_TelemetryProfile{
				TelemetryProfile: tc.upTelemetryProfile,
			}}
			_, err = invClient.Update(
				ctx,
				tenant1,
				telProf.ResourceId,
				&fieldmaskpb.FieldMask{Paths: tc.fm},
				updateTpReq,
			)
			require.NoError(t, err)
			time.Sleep(200 * time.Millisecond)
			cachedTp := telemetryCache.ListTelemetryProfileByRelation(tenant1, tc.relationResID)
			require.Len(t, cachedTp, 1)
			fmutils.Filter(cachedTp[0], tc.fm)
			fmutils.Filter(tc.upTelemetryProfile, tc.fm)

			if eq, diff := inv_testing.ProtoEqualOrDiff(tc.upTelemetryProfile, cachedTp[0]); !eq {
				t.Errorf("wrong telemetry profile in cache: %v", diff)
			}

			// Ensure isolation and no modification to other tenant cache
			assertSameTpInCache(t, tenant2, telemetryCache, siteT2.GetResourceId(), telProfT2)
		})
	}
}

func Test_TelemetryCache_UpdateTelemetryProfilesGroup(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a schedule cache with the testing client
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
		invclient.WithInventoryAddress("bufconn"),
		invclient.WithDialOption(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
		),
	)
	require.NotNil(t, telemetryCache)
	require.NoError(t, err)
	defer telemetryCache.Stop()

	site := dao.CreateSite(t, tenant1)
	tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tgMetric2 := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tpSite := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), tgMetric, true)
	tpSite.Group = tgMetric
	tpSite.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: site,
	}
	// Create just one TP for other tenant for validating isolation
	siteT2 := dao.CreateSite(t, tenant2)
	tgMetricT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	telProfT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgMetricT2, true)
	telProfT2.Group = tgMetricT2
	telProfT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}
	invClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	updateTpReq := &inv_v1.Resource{
		Resource: &inv_v1.Resource_TelemetryProfile{
			TelemetryProfile: &telemetryv1.TelemetryProfile{
				Group: tgMetric2,
			},
		},
	}
	updatedRes, err := invClient.Update(
		ctx,
		tenant1,
		tpSite.ResourceId,
		&fieldmaskpb.FieldMask{Paths: []string{telemetryv1.TelemetryProfileEdgeGroup}},
		updateTpReq,
	)
	require.NoError(t, err)
	tpSite.Group = tgMetric2
	tpSite.UpdatedAt = updatedRes.GetTelemetryProfile().GetUpdatedAt()

	time.Sleep(200 * time.Millisecond)
	assertSameTpInCache(t, tenant1, telemetryCache, site.ResourceId, tpSite)
	// Ensure isolation and no modification to other tenant cache
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.GetResourceId(), telProfT2)
}

func Test_TelemetryCache_UpdateTelemetryProfilesField(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// Init a schedule cache with the testing client
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	telemetryCache, err := invclient.NewTelemetryCacheClientWithOptions(ctx,
		invclient.WithInventoryAddress("bufconn"),
		invclient.WithDialOption(
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return inv_testing.BufconnLis.Dial() }),
		),
	)
	require.NotNil(t, telemetryCache)
	require.NoError(t, err)
	defer telemetryCache.Stop()

	site := dao.CreateSite(t, tenant1)
	tgMetric := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	tpSite := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(site), tgMetric, true)
	tpSite.Group = tgMetric
	tpSite.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: site,
	}
	// Create just one TP for other tenant for validating isolation
	siteT2 := dao.CreateSite(t, tenant2)
	tgMetricT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	telProfT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(siteT2), tgMetricT2, true)
	telProfT2.Group = tgMetricT2
	telProfT2.Relation = &telemetryv1.TelemetryProfile_Site{
		Site: siteT2,
	}
	invClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	updateTpReq := &inv_v1.Resource{
		Resource: &inv_v1.Resource_TelemetryProfile{
			TelemetryProfile: &telemetryv1.TelemetryProfile{
				MetricsInterval: 10,
			},
		},
	}
	updatedRes, err := invClient.Update(
		ctx,
		tenant1,
		tpSite.ResourceId,
		&fieldmaskpb.FieldMask{Paths: []string{telemetryv1.TelemetryProfileFieldMetricsInterval}},
		updateTpReq,
	)
	require.NoError(t, err)
	tpSite.MetricsInterval = 10
	tpSite.UpdatedAt = updatedRes.GetTelemetryProfile().GetUpdatedAt()
	time.Sleep(200 * time.Millisecond)
	assertSameTpInCache(t, tenant1, telemetryCache, site.ResourceId, tpSite)
	// Ensure isolation and no modification to other tenant cache
	assertSameTpInCache(t, tenant2, telemetryCache, siteT2.GetResourceId(), telProfT2)
}

func assertSameTpInCache(
	t *testing.T,
	tenantID string,
	telCache *invclient.TelemetryCache,
	relationResourceID string,
	expectedTP *telemetryv1.TelemetryProfile,
) {
	t.Helper()
	cachedTp := telCache.ListTelemetryProfileByRelation(tenantID, relationResourceID)
	require.Len(t, cachedTp, 1)
	if eq, diff := inv_testing.ProtoEqualOrDiff(expectedTP, cachedTp[0]); !eq {
		t.Errorf("wrong telemetry profile in cache: %v", diff)
	}
}

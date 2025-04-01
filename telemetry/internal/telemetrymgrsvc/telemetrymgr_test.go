// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package telemetrymgr_test

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

	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	tm "github.com/open-edge-platform/infra-managers/telemetry/internal/telemetrymgrsvc"
	telemetry_testing "github.com/open-edge-platform/infra-managers/telemetry/internal/testing"
	pb "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
)

var rbacRules = "../../rego/authz.rego"

const (
	tenant1 = "11111111-1111-1111-1111-111111111111"
	tenant2 = "22222222-2222-2222-2222-222222222222"
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

//nolint:funlen // this test contains several other's test cases
func TestGetTelemetryConfigByGUID(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// create telemetryclient
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient
	opaServer, err := rbac.New(rbacRules)
	require.NoError(t, err)
	cli := tm.NewTelemetrymgrServer(telemetryInvClient, true, opaServer, true)

	// Create a context, context must contain tenantID as well, interceptor is not used for these tests.
	ctxT1 := inv_testing.CreateIncomingContextWithENJWT(t, context.Background(), tenant1)
	ctxT1 = tenant.AddTenantIDToContext(ctxT1, tenant1)

	t.Run("ErrorNoHost", func(t *testing.T) {
		// Create a request
		req := &pb.GetTelemetryConfigByGuidRequest{
			Guid: "57ed598c-4b94-11ee-806c-3a7c7693aac3", // not valid guid
		}
		_, err = cli.GetTelemetryConfigByGUID(ctxT1, req)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	t.Run("ErrorNoInstance", func(t *testing.T) {
		// create region
		region := dao.CreateRegion(t, tenant1)
		// create site
		site := dao.CreateSite(t, tenant1, inv_testing.SiteRegion(region))
		// create host
		host := dao.CreateHost(t, tenant1, inv_testing.HostSite(site))
		host.Site = site

		// Create a request
		req := &pb.GetTelemetryConfigByGuidRequest{
			Guid: host.GetUuid(),
		}
		_, err = cli.GetTelemetryConfigByGUID(ctxT1, req)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.NotFound, sts.Code())
	})

	t.Run("ErrorNoJWT", func(t *testing.T) {
		// Create a request
		req := &pb.GetTelemetryConfigByGuidRequest{
			Guid: "57ed598c-4b94-11ee-806c-3a7c7693aac3", // not valid guid
		}
		_, err = cli.GetTelemetryConfigByGUID(context.Background(), req)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, sts.Code())
	})

	t.Run("ErrorWrongJWT", func(t *testing.T) {
		// Create a context
		ctx1 := inv_testing.CreateIncomingContextWithJWT(t, context.Background(), "")
		// Create a request
		req := &pb.GetTelemetryConfigByGuidRequest{
			Guid: "57ed598c-4b94-11ee-806c-3a7c7693aac3", // not valid guid
		}
		_, err = cli.GetTelemetryConfigByGUID(ctx1, req)
		require.Error(t, err)
		sts, _ := status.FromError(err)
		assert.Equal(t, codes.PermissionDenied, sts.Code())
	})

	// test get profiles
	t.Run("GetProfile", func(t *testing.T) {
		// create region
		region := dao.CreateRegion(t, tenant1)
		// create site
		site := dao.CreateSite(t, tenant1, inv_testing.SiteRegion(region))
		// create host
		host := dao.CreateHost(t, tenant1, inv_testing.HostSite(site))
		host.Site = site
		// create instance
		os1 := dao.CreateOs(t, tenant1)
		inst := dao.CreateInstance(t, tenant1, host, os1)
		// create group and profiles
		group := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
		profile := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(region), group, true)
		profile1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(inst), group, true)

		// Load all telemetry profiles, client doesn't react to events.
		telemetryInvClient.TelemetryCache.LoadAllTelemetryProfiles()

		// sync
		// profile picked
		profile.Group = group
		profile.Relation = &telemetryv1.TelemetryProfile_Region{
			Region: region,
		}
		profile.MetricsInterval = 300
		// profile1 not picked
		profile1.Group = group
		profile1.Relation = &telemetryv1.TelemetryProfile_Instance{
			Instance: inst,
		}
		profile1.MetricsInterval = 400
		// create expexted cfg
		telemetryCfgArray := make([]*pb.GetTelemetryConfigResponse_TelemetryCfg, 0, 2)
		for _, cfgRow := range group.Groups {
			cfg := &pb.GetTelemetryConfigResponse_TelemetryCfg{
				Input:    cfgRow,
				Type:     pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS,
				Kind:     pb.CollectorKind_COLLECTOR_KIND_HOST,
				Interval: 300,
			}

			telemetryCfgArray = append(telemetryCfgArray, cfg)
		}
		telemetryResponse := &pb.GetTelemetryConfigResponse{
			HostGuid:  host.GetUuid(),
			Timestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
			Cfg:       telemetryCfgArray,
		}

		// Create a request
		req := &pb.GetTelemetryConfigByGuidRequest{
			Guid: host.GetUuid(), // real guid
		}

		resp, err := cli.GetTelemetryConfigByGUID(ctxT1, req)
		require.NoError(t, err)
		// dont check time stamp
		resp.Timestamp = telemetryResponse.Timestamp
		if eq, diff := inv_testing.ProtoEqualOrDiff(resp, telemetryResponse); !eq {
			t.Errorf("TestGetTelemetryConfigByGUID() data not equal: %v", diff)
		}
	})
}

//nolint:funlen // this test contains several other's test cases
func TestGetTelemetryConfigByGUID_TenantIsolation(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	// create telemetryclient
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient
	opaServer, err := rbac.New(rbacRules)
	require.NoError(t, err)
	cli := tm.NewTelemetrymgrServer(telemetryInvClient, true, opaServer, true)

	// Create a context, context must contain tenantID as well, interceptor is not used for these tests.
	ctxT1 := inv_testing.CreateIncomingContextWithENJWT(t, context.Background(), tenant1)
	ctxT1 = tenant.AddTenantIDToContext(ctxT1, tenant1)
	ctxT2 := inv_testing.CreateIncomingContextWithENJWT(t, context.Background(), tenant2)
	ctxT2 = tenant.AddTenantIDToContext(ctxT2, tenant2)

	// Tenant1
	regionT1 := dao.CreateRegion(t, tenant1)
	siteT1 := dao.CreateSite(t, tenant1, inv_testing.SiteRegion(regionT1))
	hostT1 := dao.CreateHost(t, tenant1, inv_testing.HostSite(siteT1))
	osT1 := dao.CreateOs(t, tenant1)
	dao.CreateInstance(t, tenant1, hostT1, osT1)
	hostT1.Site = siteT1
	groupT1 := dao.CreateTelemetryGroupMetrics(t, tenant1, true)
	profileT1 := dao.CreateTelemetryProfile(t, tenant1, inv_testing.TelemetryProfileTarget(regionT1), groupT1, true)
	// Tenant2
	osT2 := dao.CreateOs(t, tenant2)
	hostT2 := dao.CreateHost(t, tenant2)
	instT2 := dao.CreateInstance(t, tenant2, hostT2, osT2)
	groupT2 := dao.CreateTelemetryGroupMetrics(t, tenant2, true)
	profileT2 := dao.CreateTelemetryProfile(t, tenant2, inv_testing.TelemetryProfileTarget(instT2), groupT2, true)

	// Load all telemetry profiles, client doesn't react to events.
	telemetryInvClient.TelemetryCache.LoadAllTelemetryProfiles()

	// profile picked
	profileT1.Group = groupT1
	profileT1.Relation = &telemetryv1.TelemetryProfile_Region{
		Region: regionT1,
	}
	profileT1.MetricsInterval = 300
	// profile1 not picked
	profileT2.Group = groupT2
	profileT2.Relation = &telemetryv1.TelemetryProfile_Instance{
		Instance: instT2,
	}
	profileT2.MetricsInterval = 400
	// create expected cfg for tenant1
	telemetryCfgArrayT1 := make([]*pb.GetTelemetryConfigResponse_TelemetryCfg, 0, 2)
	for _, cfgRow := range groupT1.Groups {
		cfg := &pb.GetTelemetryConfigResponse_TelemetryCfg{
			Input:    cfgRow,
			Type:     pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS,
			Kind:     pb.CollectorKind_COLLECTOR_KIND_HOST,
			Interval: 300,
		}
		telemetryCfgArrayT1 = append(telemetryCfgArrayT1, cfg)
	}
	telemetryResponseT1 := &pb.GetTelemetryConfigResponse{
		HostGuid:  hostT1.GetUuid(),
		Timestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
		Cfg:       telemetryCfgArrayT1,
	}
	// create expected cfg for tenant2
	telemetryCfgArrayT2 := make([]*pb.GetTelemetryConfigResponse_TelemetryCfg, 0, 2)
	for _, cfgRow := range groupT2.Groups {
		cfg := &pb.GetTelemetryConfigResponse_TelemetryCfg{
			Input:    cfgRow,
			Type:     pb.TelemetryResourceKind_TELEMETRY_RESOURCE_KIND_METRICS,
			Kind:     pb.CollectorKind_COLLECTOR_KIND_HOST,
			Interval: 300,
		}
		telemetryCfgArrayT2 = append(telemetryCfgArrayT2, cfg)
	}
	telemetryResponseT2 := &pb.GetTelemetryConfigResponse{
		HostGuid:  hostT2.GetUuid(),
		Timestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
		Cfg:       telemetryCfgArrayT2,
	}

	// Tenant1
	reqT1 := &pb.GetTelemetryConfigByGuidRequest{
		Guid: hostT1.GetUuid(), // real guid
	}
	respT1, err := cli.GetTelemetryConfigByGUID(ctxT1, reqT1)
	require.NoError(t, err)
	// dont check time stamp
	respT1.Timestamp = telemetryResponseT1.Timestamp
	if eq, diff := inv_testing.ProtoEqualOrDiff(respT1, telemetryResponseT1); !eq {
		t.Errorf("TestGetTelemetryConfigByGUID() data not equal: %v", diff)
	}

	// Tenant2
	reqT2 := &pb.GetTelemetryConfigByGuidRequest{
		Guid: hostT2.GetUuid(), // real guid
	}
	respT2, err := cli.GetTelemetryConfigByGUID(ctxT2, reqT2)
	require.NoError(t, err)
	// dont check time stamp
	respT2.Timestamp = telemetryResponseT2.Timestamp
	if eq, diff := inv_testing.ProtoEqualOrDiff(respT2, telemetryResponseT2); !eq {
		t.Errorf("TestGetTelemetryConfigByGUID() data not equal: %v", diff)
	}

	// Ensure isolation
	resp, err := cli.GetTelemetryConfigByGUID(ctxT1, reqT2)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Nil(t, resp)

	resp, err = cli.GetTelemetryConfigByGUID(ctxT2, reqT1)
	require.Error(t, err)
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Nil(t, resp)
}

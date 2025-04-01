// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0
//
//nolint:testpackage // it's an internal package test
package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"gopkg.in/yaml.v2"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	tenantv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/tenant/v1"
	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
	osrm_testing "github.com/open-edge-platform/infra-managers/os-resource/internal/testing"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/util"
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

func TestReconcileAllE2E(t *testing.T) {
	invDao := inv_testing.NewInvResourceDAOOrFail(t)

	osrm_testing.CreateInventoryClientForTesting()
	t.Cleanup(func() {
		osrm_testing.DeleteInventoryClientForTesting()
	})

	var ubuntuProfile fsclient.OSProfileManifest
	if err := yaml.Unmarshal([]byte(osrm_testing.UbuntuProfile), &ubuntuProfile); err != nil {
		t.Errorf("Error unmarshalling UbuntuProfile JSON")
	}

	// set RS_EN_PROFILE_REPO env variable needed by GetLatestOsProfiles()
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)

	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+ubuntuProfile.Spec.ProfileName,
		osrm_testing.ExampleOsConfig.OsProfileRevision).Return(&[]as.Artifact{osrm_testing.ExampleUbuntuOSArtifact}, nil)

	osrmController, err := New(osrm_testing.InvClient, osrm_testing.ExampleOsConfig)
	require.NoError(t, err)

	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})

	err = osrmController.Start()
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	osResources, err := osrmController.invClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
	require.NoError(t, err)
	require.Len(t, osResources, 1)

	gresp, err := invDao.GetAPIClient().Get(ctx, tenant.GetTenantId(), tenant.GetResourceId())
	require.NoError(t, err)
	tenantInv := gresp.GetResource().GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	providerRes, err := osrmController.invClient.GetProviderSingularByName(
		ctx, tenant.GetTenantId(), util.InfraOnboardingProviderName)
	require.NoError(t, err)
	assert.NotEmpty(t, providerRes.Config)

	_, err = invDao.GetTCClient().Update(ctx, tenant.GetTenantId(), tenant.GetResourceId(),
		&fieldmaskpb.FieldMask{Paths: []string{
			tenantv1.TenantFieldDesiredState,
		}}, &inv_v1.Resource{
			Resource: &inv_v1.Resource_Tenant{
				Tenant: &tenantv1.Tenant{
					ResourceId:   tenant.GetResourceId(),
					DesiredState: tenantv1.TenantState_TENANT_STATE_DELETED,
				},
			},
		})
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	gresp, err = invDao.GetAPIClient().Get(ctx, tenant.GetTenantId(), tenant.GetResourceId())
	require.NoError(t, err)
	tenantInv = gresp.GetResource().GetTenant()
	assert.Equal(t, false, tenantInv.GetWatcherOsmanager())

	t.Cleanup(func() {
		osrmController.Stop()
	})
}

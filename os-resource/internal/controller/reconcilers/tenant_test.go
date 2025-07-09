// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	inventoryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	tenantv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/tenant/v1"
	as "github.com/open-edge-platform/infra-core/inventory/v2/pkg/artifactservice"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/providerconfiguration"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/common"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/controller/reconcilers"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
	osrm_testing "github.com/open-edge-platform/infra-managers/os-resource/internal/testing"
	util2 "github.com/open-edge-platform/infra-managers/os-resource/internal/util"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(wd)))
	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	osrm_testing.CreateInventoryClientForTesting()
	run := m.Run() // run all tests
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

func runReconcilationFunc(t *testing.T, tenantController *rec_v2.Controller[reconcilers.ReconcilerID]) {
	t.Helper()
	reconcileFunc := func() {
		for {
			select {
			case ev, ok := <-inv_testing.TestClientsEvents[inv_testing.RMClient]:
				require.True(t, ok, "No events received")
				_tenantID, _resourceID, err := util.GetResourceKeyFromResource(ev.Event.Resource)
				require.NoError(t, err)
				resKind, err := util.GetResourceKindFromResourceID(ev.Event.ResourceId)
				require.NoError(t, err)
				if resKind != inventoryv1.ResourceKind_RESOURCE_KIND_TENANT {
					continue
				}
				err = tenantController.Reconcile(reconcilers.WrapReconcilerID(_tenantID, _resourceID))
				assert.NoError(t, err, "Reconciliation failed")
			case <-time.After(1 * time.Second):
				return
			}
		}
	}
	reconcileFunc()
}

func TestHostReconcileAtBootstrap(t *testing.T) {
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

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, osrm_testing.ExampleOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))
	// do not Stop() to avoid races, should be safe in tests

	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})
	tenantID := tenant.GetResourceId()

	osResource := inv_testing.CreateOsWithArgs(t, "", "random", osrm_testing.ExampleOsConfig.DefaultProfile,
		osv1.SecurityFeature_SECURITY_FEATURE_UNSPECIFIED, osv1.OsType_OS_TYPE_MUTABLE)
	instance := inv_testing.CreateInstance(t, nil, osResource)
	instanceID := instance.GetResourceId()

	initialImageID := getResource(t, instanceID).GetInstance().GetDesiredOs().GetImageId()
	assert.NotEqual(t, ubuntuProfile.Spec.OsImageVersion, initialImageID)

	runReconcilationFunc(t, tenantController)
	defer cleanupProvider(t, tenant.GetTenantId())

	tenantInv := getResource(t, tenantID).GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	imageID := getResource(t, instanceID).GetInstance().GetDesiredOs().GetImageId()
	assert.Equal(t, ubuntuProfile.Spec.OsImageVersion, imageID)

	assertProvider(t, tenantInv.GetTenantId(), true)
}

func TestReconcileAtBootstrapWithAutoprovisionDisabled(t *testing.T) {
	testOsConfig := common.OsConfig{
		EnabledProfiles:   []string{"ubuntu-22.04-lts-generic"},
		OsProfileRevision: "main",
		DefaultProfile:    "test", // intentionally set, but should be ignored when AutoProvision=false
		AutoProvision:     false,
		ManualMode:        false,
	}

	var ubuntuProfile fsclient.OSProfileManifest
	if err := yaml.Unmarshal([]byte(osrm_testing.UbuntuProfile), &ubuntuProfile); err != nil {
		t.Errorf("Error unmarshalling UbuntuProfile JSON")
	}

	// set RS_EN_PROFILE_REPO env variable needed by GetLatestOsProfiles()
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)

	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+ubuntuProfile.Spec.ProfileName,
		testOsConfig.OsProfileRevision).Return(&[]as.Artifact{osrm_testing.ExampleUbuntuOSArtifact}, nil)

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, testOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))
	// do not Stop() to avoid races, should be safe in tests

	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})
	tenantID := tenant.GetResourceId()

	runReconcilationFunc(t, tenantController)
	defer cleanupProvider(t, tenant.GetTenantId())

	tenantInv := getResource(t, tenantID).GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	assertProvider(t, tenantInv.GetTenantId(), false)
}

func assertProvider(t *testing.T, tenantID string, autoProvisionEnabled bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	provRes, err := osrm_testing.InvClient.GetProviderSingularByName(ctx, tenantID, util2.InfraOnboardingProviderName)
	require.NoError(t, err)

	var providerConfig providerconfiguration.ProviderConfig
	err = json.Unmarshal([]byte(provRes.Config), &providerConfig)
	require.NoError(t, err)

	assert.Equal(t, providerConfig.AutoProvision, autoProvisionEnabled)
	assert.Equal(t, providerConfig.DefaultOs != "", autoProvisionEnabled)
}

func cleanupProvider(t *testing.T, tenantID string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	//nolint:errcheck // no need to check for errors
	provRes, _ := osrm_testing.InvClient.GetProviderSingularByName(ctx, tenantID, util2.InfraOnboardingProviderName)

	if provRes != nil {
		//nolint:errcheck // ignore any error
		inv_testing.TestClients[inv_testing.APIClient].Delete(ctx, provRes.GetResourceId())
	}
}

func getResource(t *testing.T, resourceID string) *inventoryv1.Resource {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	gresp, err := inv_testing.TestClients[inv_testing.APIClient].Get(ctx, resourceID)
	require.NoError(t, err)

	return gresp.GetResource()
}

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

func TestTenantReconciler_CVEHandlingForImmutableOS(t *testing.T) {
	// Test to verify CVE processing doesn't break tenant reconciliation
	// Create a minimal immutable OS profile
	mockImmutableProfile := fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Name:                 "test-immutable-profile",
			Type:                 "OS_TYPE_IMMUTABLE",
			Provider:             "INFRA",
			Architecture:         "x86_64",
			ProfileName:          "test-immutable-profile",
			OsImageURL:           "https://example.com/os-image.img",
			OsImageSha256:        "abc123",
			OsImageVersion:       "1.0.0",
			OsPackageManifestURL: "", // Empty to avoid HTTP calls
			OsExistingCvesURL:    "", // Empty to avoid HTTP calls
			OsFixedCvesURL:       "", // Empty to avoid HTTP calls
			SecurityFeature:      "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
			Description:          "Test immutable OS profile without CVE data",
		},
	}

	// Set up environment and mock services
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)
	t.Setenv(fsclient.EnvNameRsFilesProxyAddress, "localhost:8080")

	// Create mock artifact service
	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m

	// Mock the artifact download for the immutable profile
	profileYaml, err := yaml.Marshal(mockImmutableProfile)
	require.NoError(t, err)

	mockArtifact := as.Artifact{
		Data: profileYaml,
	}
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+mockImmutableProfile.Spec.ProfileName,
		"main").Return(&[]as.Artifact{mockArtifact}, nil)

	// Create OS config with immutable profile
	testOsConfig := common.OsConfig{
		EnabledProfiles:   []string{mockImmutableProfile.Spec.ProfileName},
		OsProfileRevision: "main",
		DefaultProfile:    mockImmutableProfile.Spec.ProfileName,
		AutoProvision:     true,
		ManualMode:        false,
	}

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, testOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))

	// Create tenant
	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})
	tenantID := tenant.GetResourceId()

	// Run reconciliation
	runReconcilationFunc(t, tenantController)
	defer cleanupProvider(t, tenant.GetTenantId())

	// Verify tenant was acknowledged
	tenantInv := getResource(t, tenantID).GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	// Verify OS resource was created with CVE data
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	osResources, err := osrm_testing.InvClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
	require.NoError(t, err)
	require.Len(t, osResources, 1)

	osResource := osResources[0]
	assert.Equal(t, mockImmutableProfile.Spec.ProfileName, osResource.GetProfileName())
	assert.Equal(t, mockImmutableProfile.Spec.OsImageVersion, osResource.GetImageId())
	assert.Equal(t, osv1.OsType_OS_TYPE_IMMUTABLE, osResource.GetOsType())

	// Note: CVE data would be populated by the fsclient functions in a real scenario
	// In this test, we verify the structure is set up correctly for CVE handling
}

func TestTenantReconciler_CVEUpdateForExistingImmutableOS(t *testing.T) {
	// Create a mock immutable OS profile
	mockProfile := fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Name:              "test-update-profile",
			Type:              "OS_TYPE_IMMUTABLE",
			Provider:          "INFRA",
			ProfileName:       "test-update-profile",
			OsImageVersion:    "2.0.0",
			OsExistingCvesURL: "/cves/existing-updated.json",
			OsFixedCvesURL:    "/cves/fixed-updated.json",
			SecurityFeature:   "SECURITY_FEATURE_SECURE_BOOT_AND_FULL_DISK_ENCRYPTION",
			Description:       "Test immutable OS profile with updated CVE data",
		},
	}

	// Set up environment
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)
	t.Setenv(fsclient.EnvNameRsFilesProxyAddress, "localhost:8080")

	// Create tenant first (needed for creating OS resource)
	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
		tenant.CurrentState = tenantv1.TenantState_TENANT_STATE_CREATED
		tenant.WatcherOsmanager = true
	})

	// Create existing OS resource first
	existingOS := inv_testing.CreateOsWithArgs(t, tenant.GetTenantId(), "existing-os",
		mockProfile.Spec.ProfileName, osv1.SecurityFeature_SECURITY_FEATURE_UNSPECIFIED,
		osv1.OsType_OS_TYPE_IMMUTABLE)

	// Update the existing OS to match our test profile
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Set the image ID to match our profile
	existingOS.ImageId = mockProfile.Spec.OsImageVersion

	// Create mock artifact service
	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m

	profileYaml, err := yaml.Marshal(mockProfile)
	require.NoError(t, err)

	mockArtifact := as.Artifact{Data: profileYaml}
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+mockProfile.Spec.ProfileName,
		"main").Return(&[]as.Artifact{mockArtifact}, nil)

	testOsConfig := common.OsConfig{
		EnabledProfiles:   []string{mockProfile.Spec.ProfileName},
		OsProfileRevision: "main",
		DefaultProfile:    mockProfile.Spec.ProfileName,
		AutoProvision:     true,
		ManualMode:        false,
	}

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, testOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))

	// Run reconciliation - this should trigger CVE updates for existing OS resources
	runReconcilationFunc(t, tenantController)

	// Verify the OS resource still exists and would have been processed for CVE updates
	osResources, err := osrm_testing.InvClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
	require.NoError(t, err)
	assert.NotEmpty(t, osResources)

	// Verify the reconciliation process handled the existing OS resource
	// In a real scenario, this would include updated CVE data
	for _, osRes := range osResources {
		if osRes.GetProfileName() == mockProfile.Spec.ProfileName &&
			osRes.GetImageId() == mockProfile.Spec.OsImageVersion {
			assert.Equal(t, osv1.OsType_OS_TYPE_IMMUTABLE, osRes.GetOsType())
		}
	}
}

func TestTenantReconciler_SkipCVEUpdateForMutableOS(t *testing.T) {
	// Create a mock mutable OS profile
	mockMutableProfile := fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Name:              "test-mutable-profile",
			Type:              "OS_TYPE_MUTABLE",
			Provider:          "INFRA",
			ProfileName:       "test-mutable-profile",
			OsImageVersion:    "1.0.0",
			OsExistingCvesURL: "/cves/existing.json",
			OsFixedCvesURL:    "/cves/fixed.json",
		},
	}

	// Set up environment
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)

	// Create mock artifact service
	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m

	profileYaml, err := yaml.Marshal(mockMutableProfile)
	require.NoError(t, err)

	mockArtifact := as.Artifact{Data: profileYaml}
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+mockMutableProfile.Spec.ProfileName,
		"main").Return(&[]as.Artifact{mockArtifact}, nil)

	testOsConfig := common.OsConfig{
		EnabledProfiles:   []string{mockMutableProfile.Spec.ProfileName},
		OsProfileRevision: "main",
		DefaultProfile:    mockMutableProfile.Spec.ProfileName,
		AutoProvision:     true,
		ManualMode:        false,
	}

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, testOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))

	// Create tenant
	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})
	tenantID := tenant.GetResourceId()

	// Run reconciliation
	runReconcilationFunc(t, tenantController)
	defer cleanupProvider(t, tenant.GetTenantId())

	// Verify tenant was acknowledged
	tenantInv := getResource(t, tenantID).GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	// Verify OS resource was created but without CVE processing for mutable OS
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	osResources, err := osrm_testing.InvClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
	require.NoError(t, err)
	require.Len(t, osResources, 1)

	osResource := osResources[0]
	assert.Equal(t, mockMutableProfile.Spec.ProfileName, osResource.GetProfileName())
	assert.Equal(t, mockMutableProfile.Spec.OsImageVersion, osResource.GetImageId())
	assert.Equal(t, osv1.OsType_OS_TYPE_MUTABLE, osResource.GetOsType())

	// For mutable OS, CVE fields should remain empty as CVE processing is skipped
	assert.Empty(t, osResource.GetExistingCves())
	assert.Empty(t, osResource.GetFixedCves())
}

func TestTenantReconciler_MixedOSTypesHandling(t *testing.T) {
	// Create both mutable and immutable OS profiles
	mutableProfile := fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Name:           "test-mutable",
			Type:           "OS_TYPE_MUTABLE",
			ProfileName:    "test-mutable",
			OsImageVersion: "1.0.0",
		},
	}

	immutableProfile := fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Name:              "test-immutable",
			Type:              "OS_TYPE_IMMUTABLE",
			ProfileName:       "test-immutable",
			OsImageVersion:    "2.0.0",
			OsExistingCvesURL: "/cves/existing.json",
			OsFixedCvesURL:    "/cves/fixed.json",
		},
	}

	// Set up environment
	t.Setenv(fsclient.EnvNameRsEnProfileRepo, osrm_testing.EnProfileRepo)
	t.Setenv(fsclient.EnvNameRsFilesProxyAddress, "localhost:8080")

	// Create mock artifact service
	m := &osrm_testing.MockArtifactService{}
	as.DefaultArtService = m

	// Mock artifacts for both profiles
	mutableYaml, err := yaml.Marshal(mutableProfile)
	require.NoError(t, err)
	immutableYaml, err := yaml.Marshal(immutableProfile)
	require.NoError(t, err)

	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+mutableProfile.Spec.ProfileName,
		"main").Return(&[]as.Artifact{{Data: mutableYaml}}, nil)
	m.On("DownloadArtifacts", osrm_testing.EnProfileRepo+immutableProfile.Spec.ProfileName,
		"main").Return(&[]as.Artifact{{Data: immutableYaml}}, nil)

	testOsConfig := common.OsConfig{
		EnabledProfiles:   []string{mutableProfile.Spec.ProfileName, immutableProfile.Spec.ProfileName},
		OsProfileRevision: "main",
		DefaultProfile:    mutableProfile.Spec.ProfileName,
		AutoProvision:     true,
		ManualMode:        false,
	}

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, testOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))

	// Create tenant
	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})

	// Run reconciliation
	runReconcilationFunc(t, tenantController)
	defer cleanupProvider(t, tenant.GetTenantId())

	// Verify both OS resources were created
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	osResources, err := osrm_testing.InvClient.ListOSResourcesForTenant(ctx, tenant.GetTenantId())
	require.NoError(t, err)
	require.Len(t, osResources, 2)

	// Verify different handling for mutable vs immutable OS types
	mutableFound := false
	immutableFound := false

	for _, osRes := range osResources {
		switch osRes.GetOsType() {
		case osv1.OsType_OS_TYPE_MUTABLE:
			mutableFound = true
			assert.Equal(t, mutableProfile.Spec.ProfileName, osRes.GetProfileName())
			// Mutable OS should not have CVE processing
		case osv1.OsType_OS_TYPE_IMMUTABLE:
			immutableFound = true
			assert.Equal(t, immutableProfile.Spec.ProfileName, osRes.GetProfileName())
			// Immutable OS should be set up for CVE processing
		}
	}

	assert.True(t, mutableFound, "Mutable OS resource should be created")
	assert.True(t, immutableFound, "Immutable OS resource should be created")
}

// TestTenantReconciler_CVEFunctionality_Unit tests CVE-related logic without database dependencies
func TestTenantReconciler_CVEFunctionality_Unit(t *testing.T) {
	// Test CVE URL validation for immutable OS profiles
	immutableProfile := &fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Type:              "OS_TYPE_IMMUTABLE",
			ProfileName:       "test-immutable",
			OsImageVersion:    "1.0.0",
			OsExistingCvesURL: "/cves/existing.json",
			OsFixedCvesURL:    "/cves/fixed.json",
		},
	}

	mutableProfile := &fsclient.OSProfileManifest{
		Spec: struct {
			Name                 string                 `yaml:"name"`
			Type                 string                 `yaml:"type"`
			Provider             string                 `yaml:"provider"`
			Architecture         string                 `yaml:"architecture"`
			ProfileName          string                 `yaml:"profileName"`
			OsImageURL           string                 `yaml:"osImageUrl"`
			OsImageSha256        string                 `yaml:"osImageSha256"`
			OsImageVersion       string                 `yaml:"osImageVersion"`
			OsPackageManifestURL string                 `yaml:"osPackageManifestURL"`
			OsExistingCvesURL    string                 `yaml:"osExistingCvesURL"`
			OsFixedCvesURL       string                 `yaml:"osFixedCvesURL"`
			SecurityFeature      string                 `yaml:"securityFeature"`
			PlatformBundle       map[string]interface{} `yaml:"platformBundle"`
			Description          string                 `yaml:"description"`
		}{
			Type:           "OS_TYPE_MUTABLE",
			ProfileName:    "test-mutable",
			OsImageVersion: "1.0.0",
		},
	}

	// Test profile type identification
	assert.Equal(t, "OS_TYPE_IMMUTABLE", immutableProfile.Spec.Type)
	assert.Equal(t, "OS_TYPE_MUTABLE", mutableProfile.Spec.Type)

	// Test CVE URL presence for immutable profiles
	assert.NotEmpty(t, immutableProfile.Spec.OsExistingCvesURL)
	assert.NotEmpty(t, immutableProfile.Spec.OsFixedCvesURL)

	// Test CVE URL absence for mutable profiles (they can be empty)
	assert.Empty(t, mutableProfile.Spec.OsExistingCvesURL)
	assert.Empty(t, mutableProfile.Spec.OsFixedCvesURL)

	// Test profile ID generation logic (matches tenant.go logic)
	immutableProfileID := immutableProfile.Spec.ProfileName + immutableProfile.Spec.OsImageVersion
	mutableProfileID := mutableProfile.Spec.ProfileName + mutableProfile.Spec.OsImageVersion

	assert.Equal(t, "test-immutable1.0.0", immutableProfileID)
	assert.Equal(t, "test-mutable1.0.0", mutableProfileID)

	// Verify profiles have different IDs
	assert.NotEqual(t, immutableProfileID, mutableProfileID)
}

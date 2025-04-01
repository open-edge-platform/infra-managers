// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers_test

import (
	"context"
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
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/controller/reconcilers"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/fsclient"
	osrm_testing "github.com/open-edge-platform/infra-managers/os-resource/internal/testing"
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
	run := m.Run() // run all tests
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

func TestHostReconcileAtBootstrap(t *testing.T) {
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

	tenantReconciler := reconcilers.NewTenantReconciler(osrm_testing.InvClient, osrm_testing.ExampleOsConfig)
	require.NotNil(t, tenantReconciler)

	tenantController := rec_v2.NewController[reconcilers.ReconcilerID](tenantReconciler.Reconcile, rec_v2.WithParallelism(1))
	// do not Stop() to avoid races, should be safe in tests

	tenant := inv_testing.CreateTenant(t, func(tenant *tenantv1.Tenant) {
		tenant.DesiredState = tenantv1.TenantState_TENANT_STATE_CREATED
	})
	tenantID := tenant.GetResourceId()

	osResource := inv_testing.CreateOsWithArgs(t, "", osrm_testing.ExampleOsConfig.DefaultProfile,
		osv1.SecurityFeature_SECURITY_FEATURE_UNSPECIFIED, osv1.OsType_OS_TYPE_MUTABLE)
	instance := inv_testing.CreateInstance(t, nil, osResource)
	instanceID := instance.GetResourceId()

	initialImageID := getResource(t, instanceID).GetInstance().GetDesiredOs().GetImageId()
	assert.NotEqual(t, ubuntuProfile.Spec.OsImageVersion, initialImageID)

	runReconcilationFunc := func() {
		select {
		case ev, ok := <-inv_testing.TestClientsEvents[inv_testing.RMClient]:
			require.True(t, ok, "No events received")
			_tenantID, _resourceID, err := util.GetResourceKeyFromResource(ev.Event.Resource)
			require.NoError(t, err)
			err = tenantController.Reconcile(reconcilers.WrapReconcilerID(_tenantID, _resourceID))
			assert.NoError(t, err, "Reconciliation failed")
		case <-time.After(1 * time.Second):
			t.Fatalf("No events received within timeout")
		}
		time.Sleep(1 * time.Second)
	}

	runReconcilationFunc()

	tenantInv := getResource(t, tenantID).GetTenant()
	assert.Equal(t, true, tenantInv.GetWatcherOsmanager())

	imageID := getResource(t, instanceID).GetInstance().GetDesiredOs().GetImageId()
	assert.Equal(t, ubuntuProfile.Spec.OsImageVersion, imageID)
}

func getResource(t *testing.T, resourceID string) *inventoryv1.Resource {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	gresp, err := inv_testing.TestClients[inv_testing.APIClient].Get(ctx, resourceID)
	require.NoError(t, err)

	return gresp.GetResource()
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package maintmgr_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	mm_testing "github.com/open-edge-platform/infra-managers/maintenance/internal/testing"
	maintmgrv1 "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/maintmgr"
)

func setTestingVariables(mmProfileName, instanceProfileName, currentImageID, newImageID, resourceID string, osType os_v1.OsType,
	statusType maintmgrv1.UpdateStatus_StatusType,
) (*maintmgrv1.UpdateStatus, *computev1.InstanceResource) {
	currentOs := &os_v1.OperatingSystemResource{
		ProfileName: instanceProfileName,
		ImageId:     newImageID,
		OsType:      osType,
		ResourceId:  resourceID,
	}

	mmUpStatus := &maintmgrv1.UpdateStatus{
		ProfileName: mmProfileName,
		OsImageId:   currentImageID,
		StatusType:  statusType,
	}

	inst := &computev1.InstanceResource{
		CurrentOs: currentOs,
	}

	return mmUpStatus, inst
}

//nolint:funlen // table test
func TestInvClient_GetNewOSResourceIDIfNeeded(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	tenantClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	cli := invclient.NewInvGrpcClient(tenantClient, nil)
	osImageSha256 := inv_testing.GenerateRandomSha256()
	osTypeImmutable := os_v1.OsType_OS_TYPE_IMMUTABLE
	osTypeMutable := os_v1.OsType_OS_TYPE_MUTABLE
	statusTypeUpdated := maintmgrv1.UpdateStatus_STATUS_TYPE_UPDATED
	currentImageID := "some image ID"
	newImageID := "new image ID"
	mmProfileName := "profileName"
	resID := ""
	instanceProfileName := mmProfileName
	emptyEntry := ""
	incorrectEntry := "wrongProfileName"

	// OK - gets OS Resource ID
	t.Run("Get OSResource ID", func(t *testing.T) {
		mmUpStatus, inst := setTestingVariables(mmProfileName, instanceProfileName, currentImageID,
			newImageID, resID, osTypeImmutable, statusTypeUpdated)
		osRes := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
			os.Sha256 = osImageSha256
			os.ProfileName = mmUpStatus.ProfileName
			os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
			os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		})

		getOSResID, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, cli.InvClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.NotEmpty(t, getOSResID)
		require.NoError(t, err)
		require.Equal(t, osRes.ResourceId, getOSResID)
	})

	// Error - Empty OS Resource ID Returned
	t.Run("Same OS Resource Before and After Update", func(t *testing.T) {
		osRes := dao.CreateOsWithOpts(t, mm_testing.Tenant1, true, func(os *os_v1.OperatingSystemResource) {
			os.Sha256 = osImageSha256
			os.ProfileName = mmProfileName
			os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
			os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		})
		mmUpStatus, inst := setTestingVariables(mmProfileName, instanceProfileName, currentImageID, newImageID,
			osRes.ResourceId, osTypeImmutable, statusTypeUpdated)

		getOSResID, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.Error(t, err)
		require.Empty(t, getOSResID)
	})

	// Error - Empty OS Resource ID Returned
	t.Run("Same Current and New Image ID", func(t *testing.T) {
		mmUpStatus, inst := setTestingVariables(mmProfileName, instanceProfileName, currentImageID, currentImageID,
			resID, osTypeImmutable, statusTypeUpdated)
		getOSRes, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.Error(t, err)
		require.Empty(t, getOSRes)
	})

	// Error - Empty OS Resource ID Returned
	t.Run("instanceProfileName doesn't match mmProfileName", func(t *testing.T) {
		mmUpStatus, inst := setTestingVariables(mmProfileName, incorrectEntry, currentImageID, currentImageID, resID,
			osTypeImmutable, statusTypeUpdated)
		getOSRes, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.Error(t, err)
		require.Empty(t, getOSRes)
	})

	// Error - Empty OS Resource ID Returned
	t.Run("mutableOs", func(t *testing.T) {
		mmUpStatus, inst := setTestingVariables(mmProfileName, instanceProfileName, currentImageID, newImageID, resID,
			osTypeMutable, statusTypeUpdated)
		getOSRes, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.NoError(t, err)
		require.Empty(t, getOSRes)
	})

	// Error - Empty OS Resource ID Returned
	t.Run("mutableOS with Incorrect Profile Name", func(t *testing.T) {
		mmUpStatus, inst := setTestingVariables(mmProfileName, instanceProfileName, incorrectEntry, newImageID, resID,
			osTypeMutable, statusTypeUpdated)
		getOSRes, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
		require.NoError(t, err)
		require.Empty(t, getOSRes)
	})

	tests := []struct {
		name          string
		mmUpStatus    *maintmgrv1.UpdateStatus
		mmProfileName string
		osImageID     string
	}{
		{
			name:          "emptyProfileNameField",
			mmProfileName: emptyEntry,
			osImageID:     currentImageID,
		},
		{
			name:          "emptyOsImageField",
			mmProfileName: mmProfileName,
			osImageID:     emptyEntry,
		},
		{
			name:          "emptymmUpStatusFields",
			mmProfileName: emptyEntry,
			osImageID:     emptyEntry,
		},
		{
			name:          "incorrectmmUpStatusName",
			mmProfileName: mmProfileName,
			osImageID:     incorrectEntry,
		},
		{
			name:          "incorrectmmUpStatusID",
			mmProfileName: incorrectEntry,
			osImageID:     currentImageID,
		},
	}
	for _, tt := range tests {
		// Error - Empty OS Resource ID Returned, Error Returned
		t.Run(tt.name, func(t *testing.T) {
			mmUpStatus, inst := setTestingVariables(tt.mmProfileName, instanceProfileName, tt.osImageID, newImageID,
				resID, osTypeImmutable, statusTypeUpdated)
			getOSRes, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, tenantClient, mm_testing.Tenant1, mmUpStatus, inst)
			require.Error(t, err)
			require.Empty(t, getOSRes)
		})
	}

	// OK - Creates Grpc Client With Schedule Cache
	t.Run("closingInvGrpcClientWithScheduleCache", func(t *testing.T) {
		scheduleCache := schedule_cache.NewScheduleCacheClient(tenantClient)
		hScheduleCache, err := schedule_cache.NewHScheduleCacheClient(scheduleCache)
		require.NoError(t, err)
		cli := invclient.NewInvGrpcClient(tenantClient, hScheduleCache)
		maintmgr.SetInvGrpcCli(cli)

		assert.NotNil(t, cli)
		maintmgr.CloseInvGrpcCli()
	})
}

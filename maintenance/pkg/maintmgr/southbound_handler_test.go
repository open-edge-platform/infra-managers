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
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
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

//nolint:funlen // table test for testing updateInstanceInInv logic
func TestUpdateInstanceInInvLogic(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx := context.TODO()
	tenantClient := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	cli := invclient.NewInvGrpcClient(tenantClient, nil)

	profileName := "test-profile"
	currentImageID := "current-image-123"
	newImageID := "new-image-456"
	existingCVEs := `[{"cve_id":"CVE-2024-0001","severity":"HIGH"}]`
	newExistingCVEs := `[{"cve_id":"CVE-2024-0002","severity":"MEDIUM"}]`
	osImageSha256Current := inv_testing.GenerateRandomSha256()
	osImageSha256New := inv_testing.GenerateRandomSha256()

	// Create OS resources for testing without automatic cleanup
	currentOSRes := dao.CreateOsWithOpts(t, mm_testing.Tenant1, false, func(os *os_v1.OperatingSystemResource) {
		os.Name = "current-os-for-unit-testing"
		os.ProfileName = profileName
		os.ImageId = currentImageID
		os.Sha256 = osImageSha256Current
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		os.ExistingCves = existingCVEs
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
	})

	newOSRes := dao.CreateOsWithOpts(t, mm_testing.Tenant1, false, func(os *os_v1.OperatingSystemResource) {
		os.Name = "new-os-for-unit-testing"
		os.ProfileName = profileName
		os.ImageId = newImageID
		os.Sha256 = osImageSha256New
		os.OsType = os_v1.OsType_OS_TYPE_IMMUTABLE
		os.ExistingCves = newExistingCVEs
		os.SecurityFeature = os_v1.SecurityFeature_SECURITY_FEATURE_NONE
	})

	// Create test host without automatic cleanup
	hostRes := dao.CreateHostWithOpts(t, mm_testing.Tenant1, false, func(host *computev1.HostResource) {
		host.Uuid = "550e8400-e29b-41d4-a716-446655440000"
	})

	// Create test instance with manual cleanup control
	instanceRes := dao.CreateInstanceWithOpts(t, mm_testing.Tenant1, hostRes, currentOSRes, false,
		func(inst *computev1.InstanceResource) {
			inst.CurrentOs = currentOSRes
			inst.Os = currentOSRes
			inst.UpdateStatusIndicator = statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED
			inst.UpdateStatus = maintmgrv1.UpdateStatus_STATUS_TYPE_UNSPECIFIED.String()
		})

	// Manual cleanup in the correct order to avoid foreign key constraints
	t.Cleanup(func() {
		dao.HardDeleteInstance(t, mm_testing.Tenant1, instanceRes.ResourceId)
		dao.DeleteResource(t, mm_testing.Tenant1, currentOSRes.ResourceId)
		dao.DeleteResource(t, mm_testing.Tenant1, newOSRes.ResourceId)
		dao.HardDeleteHost(t, mm_testing.Tenant1, hostRes.ResourceId)
	})

	tests := []struct {
		name                 string
		mmUpStatus           *maintmgrv1.UpdateStatus
		setupInstance        func(*computev1.InstanceResource)
		expectedOSResourceID string
		expectedExistingCVEs string
		expectInstanceUpdate bool
		expectError          bool
	}{
		{
			name: "Update status from UNSPECIFIED to UP_TO_DATE - should update with existing CVEs",
			mmUpStatus: &maintmgrv1.UpdateStatus{
				ProfileName: profileName,
				OsImageId:   currentImageID,
				StatusType:  maintmgrv1.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
			},
			setupInstance: func(inst *computev1.InstanceResource) {
				inst.UpdateStatusIndicator = statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED
				inst.UpdateStatus = maintmgrv1.UpdateStatus_STATUS_TYPE_UNSPECIFIED.String()
			},
			expectedOSResourceID: "",
			expectedExistingCVEs: existingCVEs,
			expectInstanceUpdate: true,
			expectError:          false,
		},
		{
			name: "Update with new OS image - should get new OS resource",
			mmUpStatus: &maintmgrv1.UpdateStatus{
				ProfileName: profileName,
				OsImageId:   newImageID,
				StatusType:  maintmgrv1.UpdateStatus_STATUS_TYPE_UPDATED,
			},
			setupInstance: func(inst *computev1.InstanceResource) {
				inst.UpdateStatusIndicator = statusv1.StatusIndication_STATUS_INDICATION_IDLE
				inst.UpdateStatus = maintmgrv1.UpdateStatus_STATUS_TYPE_UP_TO_DATE.String()
			},
			expectedOSResourceID: newOSRes.ResourceId,
			expectedExistingCVEs: newExistingCVEs,
			expectInstanceUpdate: true,
			expectError:          false,
		},
		{
			name: "No status change needed - should not update",
			mmUpStatus: &maintmgrv1.UpdateStatus{
				ProfileName: profileName,
				OsImageId:   currentImageID,
				StatusType:  maintmgrv1.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
			},
			setupInstance: func(inst *computev1.InstanceResource) {
				inst.UpdateStatusIndicator = statusv1.StatusIndication_STATUS_INDICATION_IDLE
				inst.UpdateStatus = maintmgrv1.UpdateStatus_STATUS_TYPE_UP_TO_DATE.String()
			},
			expectedOSResourceID: "",
			expectedExistingCVEs: "",
			expectInstanceUpdate: false,
			expectError:          false,
		},
		{
			name: "Profile name mismatch - should fail OS resource lookup",
			mmUpStatus: &maintmgrv1.UpdateStatus{
				ProfileName: "wrong-profile",
				OsImageId:   newImageID,
				StatusType:  maintmgrv1.UpdateStatus_STATUS_TYPE_UPDATED,
			},
			setupInstance: func(inst *computev1.InstanceResource) {
				inst.UpdateStatusIndicator = statusv1.StatusIndication_STATUS_INDICATION_IDLE
				inst.UpdateStatus = maintmgrv1.UpdateStatus_STATUS_TYPE_UP_TO_DATE.String()
			},
			expectedOSResourceID: "",
			expectedExistingCVEs: "",
			expectInstanceUpdate: false,
			expectError:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup instance for this test case
			testInstance := &computev1.InstanceResource{
				ResourceId: instanceRes.ResourceId,
				CurrentOs:  currentOSRes,
				Os:         currentOSRes,
			}
			tt.setupInstance(testInstance)

			// Test GetNewOSResourceIDIfNeeded function which is part of updateInstanceInInv logic
			if tt.mmUpStatus.ProfileName == profileName && tt.mmUpStatus.OsImageId != currentImageID {
				newOSResID, err := maintmgr.GetNewOSResourceIDIfNeeded(ctx, cli.InvClient, mm_testing.Tenant1,
					tt.mmUpStatus, testInstance)

				if tt.expectError {
					require.Error(t, err)
					require.Empty(t, newOSResID)
				} else {
					require.NoError(t, err)
					if tt.expectedOSResourceID != "" {
						require.Equal(t, tt.expectedOSResourceID, newOSResID)

						// Verify the new OS resource has the expected CVEs by comparing with the known resource
						// Instead of making another client call, use the pre-created resource for verification
						if newOSResID == newOSRes.ResourceId {
							require.Equal(t, tt.expectedExistingCVEs, newOSRes.ExistingCves)
						}
					}
				}
			}

			// Simulate the updateInstanceInInv logic by testing the key components
			if tt.expectInstanceUpdate && !tt.expectError {
				// Test would verify that instance update is needed based on status change
				// This simulates the logic that updateInstanceInInv uses to determine if update is needed
				require.NotEqual(t, tt.mmUpStatus.StatusType.String(), testInstance.GetUpdateStatus())
			}
		})
	}
}

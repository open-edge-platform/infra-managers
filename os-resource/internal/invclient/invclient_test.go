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

	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/invclient"
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

// TestInventoryClient_FindOSResourceID_NoDuplicates verifies if FindOSResourceID() returns a singular object,
// even if Inventory wildcard matching returns more than one resource. See ITEP-14836 for more details.
func TestInventoryClient_FindOSResourceID_NoDuplicates(t *testing.T) {
	os1 := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-profile1"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "1"
		osr.Name = "test-1"
	})
	os2 := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-profile123"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "1"
		osr.Name = "test-2"
	})

	invDAO := inv_testing.NewInvResourceDAOOrFail(t)
	invClient, err := invclient.NewOSRMInventoryClient(invDAO.GetAPIClient(), invDAO.GetAPIClientWatcher(), make(chan bool))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	osResourceID1, err := invClient.FindOSResourceID(ctx, client.FakeTenantID, "test-profile1", "1")
	require.NoError(t, err)
	assert.Equal(t, os1.GetResourceId(), osResourceID1)

	osResourceID2, err := invClient.FindOSResourceID(ctx, client.FakeTenantID, "test-profile123", "1")
	require.NoError(t, err)
	assert.Equal(t, os2.GetResourceId(), osResourceID2)

	// should return error, non-singular object
	_, err = invClient.FindOSResourceID(ctx, client.FakeTenantID, "test-profile", "1")
	require.Error(t, err)
}

// TestInventoryClient_UpdateOSResourceExistingCves_Success tests successful scenarios
// for the UpdateOSResourceExistingCves function.
func TestInventoryClient_UpdateOSResourceExistingCves_Success(t *testing.T) {
	invDAO := inv_testing.NewInvResourceDAOOrFail(t)
	invClient, err := invclient.NewOSRMInventoryClient(invDAO.GetAPIClient(), invDAO.GetAPIClientWatcher(), make(chan bool))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create an initial OS resource
	originalOS := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-update-profile"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "update-test-1"
		osr.Name = "test-update-os"
		osr.ExistingCves = ""
		osr.ExistingCvesUrl = ""
	})

	tests := []struct {
		name           string
		setupOSRes     func() *osv1.OperatingSystemResource
		validateResult func(t *testing.T, resourceID string)
	}{
		{
			name: "Successful update with ExistingCves only",
			setupOSRes: func() *osv1.OperatingSystemResource {
				osRes := &osv1.OperatingSystemResource{
					ResourceId: originalOS.GetResourceId(),
					ExistingCves: `[{"cve_id":"CVE-2024-1111","priority":"CRITICAL",` +
						`"affected_packages":["kernel","linux-headers"]},` +
						`{"cve_id":"CVE-2024-2222","priority":"HIGH",` +
						`"affected_packages":["openssl","libssl1.1"]}]`,
				}
				return osRes
			},
			validateResult: func(t *testing.T, resourceID string) {
				t.Helper()
				// Retrieve the updated resource and verify the changes
				updatedRes, err := invDAO.GetAPIClient().Get(ctx, client.FakeTenantID, resourceID)
				require.NoError(t, err)
				require.NotNil(t, updatedRes)

				osRes := updatedRes.GetResource().GetOs()
				require.NotNil(t, osRes)

				expectedCVEs := `[{"cve_id":"CVE-2024-1111","priority":"CRITICAL",` +
					`"affected_packages":["kernel","linux-headers"]},` +
					`{"cve_id":"CVE-2024-2222","priority":"HIGH",` +
					`"affected_packages":["openssl","libssl1.1"]}]`
				assert.Equal(t, expectedCVEs, osRes.GetExistingCves())
			},
		},
		{
			name: "Successful update with empty ExistingCves",
			setupOSRes: func() *osv1.OperatingSystemResource {
				osRes := &osv1.OperatingSystemResource{
					ResourceId:   originalOS.GetResourceId(),
					ExistingCves: "",
				}
				return osRes
			},
			validateResult: func(t *testing.T, resourceID string) {
				t.Helper()
				updatedRes, err := invDAO.GetAPIClient().Get(ctx, client.FakeTenantID, resourceID)
				require.NoError(t, err)

				osRes := updatedRes.GetResource().GetOs()
				require.NotNil(t, osRes)

				assert.Equal(t, "", osRes.GetExistingCves())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osRes := tt.setupOSRes()

			err := invClient.UpdateOSResourceExistingCves(ctx, client.FakeTenantID, osRes)
			assert.NoError(t, err)
			tt.validateResult(t, osRes.GetResourceId())
		})
	}
}

// TestInventoryClient_UpdateOSResourceExistingCves_Failure tests failure scenarios
// for the UpdateOSResourceExistingCves function.
func TestInventoryClient_UpdateOSResourceExistingCves_Failure(t *testing.T) {
	invDAO := inv_testing.NewInvResourceDAOOrFail(t)
	invClient, err := invclient.NewOSRMInventoryClient(invDAO.GetAPIClient(), invDAO.GetAPIClientWatcher(), make(chan bool))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name       string
		setupOSRes func() *osv1.OperatingSystemResource
	}{
		{
			name: "Failure with invalid resource ID",
			setupOSRes: func() *osv1.OperatingSystemResource {
				osRes := &osv1.OperatingSystemResource{
					ResourceId:   "non-existent-resource-id",
					ExistingCves: `[{"cve_id":"CVE-2024-1234","priority":"HIGH","affected_packages":["openssl"]}]`,
				}
				return osRes
			},
		},
		{
			name: "Failure with empty resource ID",
			setupOSRes: func() *osv1.OperatingSystemResource {
				osRes := &osv1.OperatingSystemResource{
					ResourceId:   "",
					ExistingCves: `[{"cve_id":"CVE-2024-1234","priority":"HIGH","affected_packages":["openssl"]}]`,
				}
				return osRes
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			osRes := tt.setupOSRes()

			err := invClient.UpdateOSResourceExistingCves(ctx, client.FakeTenantID, osRes)
			assert.Error(t, err)
		})
	}
}

// TestInventoryClient_UpdateOSResourceExistingCves_ContextTimeout tests
// UpdateOSResourceExistingCves with context timeout.
func TestInventoryClient_UpdateOSResourceExistingCves_ContextTimeout(t *testing.T) {
	invDAO := inv_testing.NewInvResourceDAOOrFail(t)
	invClient, err := invclient.NewOSRMInventoryClient(invDAO.GetAPIClient(), invDAO.GetAPIClientWatcher(), make(chan bool))
	require.NoError(t, err)

	// Create an OS resource to update
	originalOS := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-timeout-profile"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "timeout-test-1"
		osr.Name = "test-timeout-os"
	})

	// Create a context that times out immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Allow the context to timeout
	time.Sleep(1 * time.Millisecond)

	osRes := &osv1.OperatingSystemResource{
		ResourceId:   originalOS.GetResourceId(),
		ExistingCves: `[{"cve_id":"CVE-2024-1234","priority":"HIGH","affected_packages":["openssl"]}]`,
	}

	err = invClient.UpdateOSResourceExistingCves(ctx, client.FakeTenantID, osRes)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

// TestInventoryClient_UpdateOSResourceExistingCves_EmptyTenantID tests
// UpdateOSResourceExistingCves with empty tenant ID.
func TestInventoryClient_UpdateOSResourceExistingCves_EmptyTenantID(t *testing.T) {
	invDAO := inv_testing.NewInvResourceDAOOrFail(t)
	invClient, err := invclient.NewOSRMInventoryClient(invDAO.GetAPIClient(), invDAO.GetAPIClientWatcher(), make(chan bool))
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Create an OS resource to update
	originalOS := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-empty-tenant-profile"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "empty-tenant-test-1"
		osr.Name = "test-empty-tenant-os"
	})

	osRes := &osv1.OperatingSystemResource{
		ResourceId:   originalOS.GetResourceId(),
		ExistingCves: `[{"cve_id":"CVE-2024-1234","priority":"HIGH","affected_packages":["openssl"]}]`,
	}

	err = invClient.UpdateOSResourceExistingCves(ctx, "", osRes)
	assert.Error(t, err)
}

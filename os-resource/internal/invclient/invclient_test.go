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
	})
	os2 := inv_testing.CreateOsWithOpts(t, true, func(osr *osv1.OperatingSystemResource) {
		osr.ProfileName = "test-profile123"
		osr.Sha256 = inv_testing.GenerateRandomSha256()
		osr.ImageId = "1"
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

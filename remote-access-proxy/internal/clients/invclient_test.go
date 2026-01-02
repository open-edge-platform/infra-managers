// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package clients_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/clients"
	rmtAccess_testing "github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
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

// Verify NewRAInventoryClient and event processing.
func TestEventsWatcher(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmtAccess_testing.CreateRemoteAccessMgrClient(t)

	rmtAccessMgrClient := rmtAccess_testing.RmtAccessCfgClient

	rac := dao.CreateRemoteAccessConfiguration(t, rmtAccess_testing.Tenant1)

	select {
	case ev, ok := <-rmtAccessMgrClient.Watcher:
		require.True(t, ok, "No events received")
		assert.Equal(t, inv_v1.SubscribeEventsResponse_EVENT_KIND_CREATED, ev.Event.EventKind, "Wrong event kind")

		expectedKind, err := util.GetResourceKindFromResourceID(ev.Event.ResourceId)
		require.NoError(t, err, "resource manager did receive a strange event")
		require.Equal(t, inv_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF, expectedKind, "Wrong resource kind")

		tID, rID, err := util.GetResourceKeyFromResource(ev.Event.Resource)
		require.NoError(t, err)

		assert.Equal(t, rmtAccess_testing.Tenant1, tID)
		assert.Equal(t, rac.GetResourceId(), rID)

	case <-time.After(2 * time.Second):
		t.Fatalf("No events received within timeout")
	}
}

// Verify NewRAInventoryClientWithOptions. Exit for error connections.
func TestNewNetworkClientWithOptions(t *testing.T) {
	_, err := clients.NewRAInventoryClientWithOptions(
		clients.WithInventoryAddress("foobar:500001"),
		clients.WithEnableTracing(true),
	)
	require.Error(t, err)
	grpcStatus, _ := status.FromError(err)
	require.Equal(t, codes.Unavailable, grpcStatus.Code())
}

func TestRmtAccessInventoryClient_GetRemoteAccessConf(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	rmtAccess_testing.CreateRemoteAccessMgrClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	rmtAccessCfgClient := rmtAccess_testing.RmtAccessCfgClient
	tenantID := rmtAccess_testing.Tenant1

	t.Run("not found", func(t *testing.T) {
		got, err := rmtAccessCfgClient.GetRemoteAccessConf(ctx, tenantID, "remoteaccess-deadbeef")
		require.Error(t, err)
		require.Nil(t, got)
	})

	t.Run("ok", func(t *testing.T) {
		rac := dao.CreateRemoteAccessConfiguration(t, tenantID)

		got, err := rmtAccessCfgClient.GetRemoteAccessConf(ctx, tenantID, rac.GetResourceId())
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, rac.GetResourceId(), got.GetResourceId())
		assert.Equal(t, tenantID, got.GetTenantId())
		require.NotNil(t, got.GetInstance())
		assert.Equal(t, rac.GetInstance().GetResourceId(), got.GetInstance().GetResourceId())
		assert.Equal(t, rac.GetDesiredState(), got.GetDesiredState())
	})

}

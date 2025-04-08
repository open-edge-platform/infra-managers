// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/networking/internal/handlers"
	"github.com/open-edge-platform/infra-managers/networking/internal/reconcilers"
	net_testing "github.com/open-edge-platform/infra-managers/networking/internal/testing"
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

// Test proper initialization.
func TestNewNBHandler(t *testing.T) {
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateNBHandler(t)

	require.NotNil(t, net_testing.NBHandler)
	require.NotNil(t, net_testing.NBHandler.Controllers)
	require.Equal(t, 1, len(net_testing.NBHandler.Controllers))
}

// Verify Watcher filtering works as expected.
func TestWatcherFilter(t *testing.T) {
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateNBHandler(t)

	// Use a mock reconciler
	var done atomic.Bool
	controller := rec_v2.NewController[reconcilers.ReconcilerID](func(_ context.Context,
		request rec_v2.Request[reconcilers.ReconcilerID],
	) rec_v2.Directive[reconcilers.ReconcilerID] {
		t.Error("Should not be called")
		done.Store(true)
		return request.Ack()
	}, rec_v2.WithParallelism(1))
	net_testing.NBHandler.Controllers[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = controller

	err := net_testing.NBHandler.Start()
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	// Reconciliation should be started at this point
	inv_testing.CreateHost(t, nil, nil)

	assert.False(t, done.Load())
}

// Verify event reconciliation.
func TestEventReconcile(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateNBHandler(t)

	// Use a mock reconciler
	var done atomic.Bool
	controller := rec_v2.NewController[reconcilers.ReconcilerID](func(_ context.Context,
		request rec_v2.Request[reconcilers.ReconcilerID],
	) rec_v2.Directive[reconcilers.ReconcilerID] {
		assert.Equal(t, net_testing.Tenant1, request.ID.GetTenantID())
		done.Store(true)
		return request.Ack()
	}, rec_v2.WithParallelism(1))
	net_testing.NBHandler.Controllers[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = controller

	err := net_testing.NBHandler.Start()
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	// Reconciliation should be started at this point
	host := dao.CreateHost(t, net_testing.Tenant1)
	hostNic := dao.CreateHostNic(t, net_testing.Tenant1, host)
	dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)
	// Reconciliation delay
	time.Sleep(1 * time.Second)
	assert.True(t, done.Load())
}

// Verify startup and periodic reconcileAll.
func TestReconcileAll(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateNBHandler(t)

	// Use a mock reconciler
	var done atomic.Bool
	controller := rec_v2.NewController[reconcilers.ReconcilerID](func(_ context.Context,
		request rec_v2.Request[reconcilers.ReconcilerID],
	) rec_v2.Directive[reconcilers.ReconcilerID] {
		assert.Equal(t, net_testing.Tenant1, request.ID.GetTenantID())
		done.Store(true)
		return request.Ack()
	}, rec_v2.WithParallelism(1))
	net_testing.NBHandler.Controllers[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = controller

	// Create beforehand the resources
	host := dao.CreateHost(t, net_testing.Tenant1)
	hostNic := dao.CreateHostNic(t, net_testing.Tenant1, host)
	dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)

	// Rewrite the ticker period
	handlers.TickerPeriod = 2 * time.Second
	err := net_testing.NBHandler.Start()
	require.NoError(t, err)
	// Initial reconcileAll
	time.Sleep(1 * time.Second)
	assert.True(t, done.Load())
	done.Store(false)
	// Periodic reconcileAll
	assert.False(t, done.Load())
	time.Sleep(2 * time.Second)
	assert.True(t, done.Load())
}

// Verify unhandled behavior.
func TestNoControllers(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	net_testing.CreateNetworkingClient(t)
	net_testing.CreateNBHandler(t)

	// Use a mock reconciler
	var done atomic.Bool
	controller := rec_v2.NewController[reconcilers.ReconcilerID](func(_ context.Context,
		request rec_v2.Request[reconcilers.ReconcilerID],
	) rec_v2.Directive[reconcilers.ReconcilerID] {
		assert.Equal(t, net_testing.Tenant1, request.ID.GetTenantID())
		done.Store(true)
		return request.Ack()
	}, rec_v2.WithParallelism(1))
	net_testing.NBHandler.Controllers[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = controller

	err := net_testing.NBHandler.Start()
	require.NoError(t, err)
	time.Sleep(1 * time.Second)
	// Reconciliation should be started at this point
	host := dao.CreateHost(t, net_testing.Tenant1)
	hostNic := dao.CreateHostNic(t, net_testing.Tenant1, host)
	dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)
	// Reconciliation delay
	time.Sleep(1 * time.Second)
	assert.True(t, done.Load())

	// Reset scenario
	done.Store(false)
	assert.False(t, done.Load())
	delete(net_testing.NBHandler.Controllers, inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS)
	delete(net_testing.NBHandler.Filters, inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS)
	assert.False(t, done.Load())
	dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)
	// Reconciliation delay
	time.Sleep(2 * time.Second)
	assert.False(t, done.Load())
}

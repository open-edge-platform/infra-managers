// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package handlers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/host/internal/hostmgr/handlers"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
	test_utils "github.com/open-edge-platform/infra-managers/host/test/utils"
)

const (
	tenant1 = "11111111-1111-1111-1111-111111111111"
	tenant2 = "22222222-2222-2222-2222-222222222222"
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

// Test proper initialization.
func TestNewNBHandler(t *testing.T) {
	test_utils.CreateHrmClient(t)
	err := test_utils.CreateNBHandler(t)
	require.NoError(t, err)
	require.NotNil(t, test_utils.HostManagerNBHandler)
}

func TestReconcileAll(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	test_utils.CreateHrmClient(t)
	err := test_utils.CreateNBHandler(t)
	require.NoError(t, err)

	host1T1 := dao.CreateHost(t, tenant1)
	host2T1 := dao.CreateHostNoCleanup(t, tenant1)
	host3T1 := dao.CreateHost(t, tenant1)
	host1T2 := dao.CreateHost(t, tenant2)
	host2T2 := dao.CreateHostNoCleanup(t, tenant2)

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host1T1))
	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.NoError(t, alivemgr.UpdateHostHeartBeat(host2T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))
	require.NoError(t, alivemgr.UpdateHostHeartBeat(host3T1))
	require.True(t, alivemgr.IsHostTracked(host3T1))

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host1T2))
	require.True(t, alivemgr.IsHostTracked(host1T2))
	require.NoError(t, alivemgr.UpdateHostHeartBeat(host2T2))
	require.True(t, alivemgr.IsHostTracked(host2T2))

	handlers.TickerPeriod = 1 * time.Second

	err = test_utils.HostManagerNBHandler.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		test_utils.HostManagerNBHandler.Stop()
	})

	// give time to run 1st reconcileAll()
	time.Sleep(1 * time.Second)

	// all should be tracked
	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))
	require.True(t, alivemgr.IsHostTracked(host3T1))
	require.True(t, alivemgr.IsHostTracked(host1T2))
	require.True(t, alivemgr.IsHostTracked(host2T2))

	// invalidate host3
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// we set both current and desired states to simulate untrusting behavior
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Host{
			Host: &computev1.HostResource{
				ResourceId:   host3T1.GetResourceId(),
				DesiredState: computev1.HostState_HOST_STATE_UNTRUSTED,
			},
		},
	}
	fmk := fieldmaskpb.FieldMask{Paths: []string{computev1.HostResourceFieldDesiredState}}
	_, err = inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, host3T1.GetResourceId(), &fmk, res)
	require.NoError(t, err)

	_, err = inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, host3T1.GetResourceId(),
			&fieldmaskpb.FieldMask{Paths: []string{computev1.HostResourceFieldCurrentState}},
			&inv_v1.Resource{
				Resource: &inv_v1.Resource_Host{
					Host: &computev1.HostResource{
						ResourceId:   host3T1.GetResourceId(),
						CurrentState: computev1.HostState_HOST_STATE_UNTRUSTED,
					},
				},
			})
	require.NoError(t, err)

	// wait for next reconciliation
	time.Sleep(1 * time.Second)
	// host3 should not be tracked anymore
	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))
	require.False(t, alivemgr.IsHostTracked(host3T1))
	// Hosts from tenant2 should all still be tracked
	require.True(t, alivemgr.IsHostTracked(host1T2))
	require.True(t, alivemgr.IsHostTracked(host2T2))

	// delete host2
	dao.HardDeleteHost(t, tenant1, host2T1.GetResourceId())
	// wait for next reconciliation
	time.Sleep(1 * time.Second)

	// host3 (invalidated) and host2 (deleted) should not be tracked anymore
	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.False(t, alivemgr.IsHostTracked(host2T1))
	require.False(t, alivemgr.IsHostTracked(host3T1))
	// Hosts from Tenant 2 should all still be tracked
	require.True(t, alivemgr.IsHostTracked(host1T2))
	require.True(t, alivemgr.IsHostTracked(host2T2))
}

func TestHostReconcile(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	test_utils.CreateHrmClient(t)
	err := test_utils.CreateNBHandler(t)
	require.NoError(t, err)

	host1T1 := dao.CreateHost(t, tenant1)
	host2T1 := dao.CreateHostNoCleanup(t, tenant1)
	host3T1 := dao.CreateHost(t, tenant1)
	host1T2 := dao.CreateHost(t, tenant2)

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host1T1))
	require.True(t, alivemgr.IsHostTracked(host1T1))

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host2T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host3T1))
	require.True(t, alivemgr.IsHostTracked(host3T1))

	require.NoError(t, alivemgr.UpdateHostHeartBeat(host1T2))
	require.True(t, alivemgr.IsHostTracked(host1T2))

	handlers.TickerPeriod = 2 * time.Minute

	err = test_utils.HostManagerNBHandler.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		test_utils.HostManagerNBHandler.Stop()
	})

	// set UNTRUSTED desired state to generate event
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Host{
			Host: &computev1.HostResource{
				ResourceId:   host3T1.GetResourceId(),
				DesiredState: computev1.HostState_HOST_STATE_UNTRUSTED,
			},
		},
	}
	fmk := fieldmaskpb.FieldMask{Paths: []string{computev1.HostResourceFieldDesiredState}}
	_, err = inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, host3T1.GetResourceId(), &fmk, res)
	require.NoError(t, err)

	// give time to handle event
	time.Sleep(1 * time.Second)

	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))
	require.False(t, alivemgr.IsHostTracked(host3T1))
	require.True(t, alivemgr.IsHostTracked(host1T2))

	// delete, generates event
	dao.HardDeleteHost(t, tenant1, host2T1.GetResourceId())

	// give time to handle event
	time.Sleep(1 * time.Second)

	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.False(t, alivemgr.IsHostTracked(host2T1))
	require.False(t, alivemgr.IsHostTracked(host3T1))
	require.True(t, alivemgr.IsHostTracked(host1T2))
}

func TestInitializeAliveMgrWithHosts(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	test_utils.CreateHrmClient(t)
	err := test_utils.CreateNBHandler(t)
	require.NoError(t, err)
	hostOs := dao.CreateOs(t, tenant1)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// No instance
	host0T1 := dao.CreateHost(t, tenant1)

	// Instance with current state Unspecified
	host1T1 := dao.CreateHost(t, tenant1)
	_ = dao.CreateInstance(t, tenant1, host1T1, hostOs)

	// Instance with current state Running
	host2T1 := dao.CreateHost(t, tenant1)
	insHost2 := dao.CreateInstance(t, tenant1, host2T1, hostOs)

	fmkCurrent := fieldmaskpb.FieldMask{Paths: []string{computev1.InstanceResourceFieldCurrentState}}

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Instance{
			Instance: &computev1.InstanceResource{
				ResourceId:   insHost2.GetResourceId(),
				CurrentState: computev1.InstanceState_INSTANCE_STATE_RUNNING,
			},
		},
	}

	_, err = inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, insHost2.GetResourceId(), &fmkCurrent, res)
	require.NoError(t, err)

	err = test_utils.HostManagerNBHandler.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		test_utils.HostManagerNBHandler.Stop()
	})

	require.False(t, alivemgr.IsHostTracked(host0T1))
	require.False(t, alivemgr.IsHostTracked(host1T1))
	require.True(t, alivemgr.IsHostTracked(host2T1))
}

func TestReconcileResource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	dao := inv_testing.NewInvResourceDAOOrFail(t)
	test_utils.CreateHrmClient(t)
	err := test_utils.CreateNBHandler(t)
	require.NoError(t, err)
	hostOs := dao.CreateOs(t, tenant1)

	// Instance with current state Unspecified
	host1T1 := dao.CreateHost(t, tenant1)
	insHost1 := dao.CreateInstance(t, tenant1, host1T1, hostOs)

	// Instance with current state Running
	host2T1 := dao.CreateHost(t, tenant1)
	_ = dao.CreateInstance(t, tenant1, host2T1, hostOs)

	handlers.TickerPeriod = 2 * time.Minute
	err = test_utils.HostManagerNBHandler.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		test_utils.HostManagerNBHandler.Stop()
	})

	require.False(t, alivemgr.IsHostTracked(host1T1))
	require.False(t, alivemgr.IsHostTracked(host2T1))

	fmkCurrent := fieldmaskpb.FieldMask{Paths: []string{computev1.InstanceResourceFieldCurrentState}}
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Instance{
			Instance: &computev1.InstanceResource{
				ResourceId:   insHost1.GetResourceId(),
				CurrentState: computev1.InstanceState_INSTANCE_STATE_RUNNING,
			},
		},
	}
	_, err = inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient().
		Update(ctx, tenant1, insHost1.GetResourceId(), &fmkCurrent, res)
	require.NoError(t, err)

	// give time to handle event
	time.Sleep(1 * time.Second)

	require.True(t, alivemgr.IsHostTracked(host1T1))
	require.False(t, alivemgr.IsHostTracked(host2T1))
}

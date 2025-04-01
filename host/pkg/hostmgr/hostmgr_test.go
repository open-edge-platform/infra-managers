// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
	"github.com/open-edge-platform/infra-managers/host/pkg/config"
	"github.com/open-edge-platform/infra-managers/host/pkg/hostmgr"
	"github.com/open-edge-platform/infra-managers/host/pkg/invclient"
)

const (
	tenant1 = "11111111-1111-1111-1111-111111111111"
	tenant2 = "22222222-2222-2222-2222-222222222222"
)

func TestHostManager_InvClient(t *testing.T) {
	wg := sync.WaitGroup{}

	_, _, err := hostmgr.StartInvGrpcCli(&wg, config.HostMgrConfig{
		EnableTracing:       true,
		InsecureGRPC:        true,
		EnableHostDiscovery: true,
	})
	require.Error(t, err)
}

func TestHostManager_AvailServer(_ *testing.T) {
	termChan := make(chan bool)
	go func() { hostmgr.StartAvailableManager(termChan) }()
	termChan <- true
}

func TestHostManager_ConnectionLost(t *testing.T) {
	termChan := make(chan bool)
	go func() { hostmgr.StartAvailableManager(termChan) }()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	host := dao.CreateHost(t, tenant1)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()

	hostUp := &computev1.HostResource{
		ResourceId: host.GetResourceId(),
	}

	// Should not update, only in RUNNING or BOOTING state
	err := invclient.UpdateHostStatus(ctx, client, tenant1, hostUp)
	require.NoError(t, err)

	err = alivemgr.UpdateHostHeartBeat(host)
	require.NoError(t, err)
	require.True(t, alivemgr.IsHostTracked(host))

	// alivemgr has no inv client, so it will not update host status, check via GetHostHeartBeat
	// host should be in running state
	hasHostHeartbeat, err := alivemgr.GetHostHeartBeat(host)
	require.NoError(t, err)
	require.True(t, hasHostHeartbeat)

	// sleep until host timer expires, default timeout is 30 seconds
	// timers will be checked every 10 seconds,
	// 60 seconds sleep should be enough to move host into connection lost state.
	time.Sleep(60 * time.Second)

	// alivemgr has no inv client, so it will not update host status, check via GetHostHeartBeat
	// host should be in connection lost state
	hasHostHeartbeat, err = alivemgr.GetHostHeartBeat(host)
	require.NoError(t, err)
	require.False(t, hasHostHeartbeat)

	// Set heartbeat again to start tracking
	err = alivemgr.UpdateHostHeartBeat(host)
	require.NoError(t, err)

	// alivemgr has no inv client, so it will not update host status, check via GetHostHeartBeat
	// host should be in running state
	hasHostHeartbeat, err = alivemgr.GetHostHeartBeat(host)
	require.NoError(t, err)
	require.True(t, hasHostHeartbeat)

	termChan <- true
}

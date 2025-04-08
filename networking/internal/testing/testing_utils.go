// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"testing"
	"time"

	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inventory_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
	"github.com/open-edge-platform/infra-managers/networking/internal/handlers"
	"github.com/open-edge-platform/infra-managers/networking/internal/reconcilers"
)

// CreateNetworkingClient is an helper function to create a new Networking Client.
func CreateNetworkingClient(tb testing.TB) {
	tb.Helper()
	var err error
	resourceKinds := []inventory_v1.ResourceKind{
		inventory_v1.ResourceKind_RESOURCE_KIND_IPADDRESS,
	}
	err = inv_testing.CreateClient(clientName, inventory_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER, resourceKinds, "")
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create Inventory NetRM client")
	}

	NetClient, err = clients.NewNetInventoryClient(
		inv_testing.TestClients[clientName].GetTenantAwareInventoryClient(), inv_testing.TestClientsEvents[clientName])
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create NetRM client")
	}
	tb.Cleanup(func() {
		NetClient.Stop()
		delete(inv_testing.TestClients, clientName)
		delete(inv_testing.TestClientsEvents, clientName)
	})
}

// CreateIPController is an helper function to create an IP Controller.
func CreateIPController(tb testing.TB) {
	tb.Helper()
	ipReconciler, err := reconcilers.NewIPReconciler(NetClient, false)

	require.NoError(tb, err)
	IPController = rec_v2.NewController[reconcilers.ReconcilerID](ipReconciler.Reconcile, rec_v2.WithParallelism(1))
	tb.Cleanup(func() { IPController.Stop() })
}

// CreateNBHandler is an helper function to create a NB handler.
func CreateNBHandler(tb testing.TB) {
	tb.Helper()
	var err error
	NBHandler, err = handlers.NewNBHandler(NetClient, false)

	require.NoError(tb, err)
	tb.Cleanup(func() { NBHandler.Stop() })
}

// AssertIPAddress asserts on IPAddress state and status.
func AssertIPAddress(
	tb testing.TB,
	tenantID string,
	resID string,
	expectedDesiredState network_v1.IPAddressState,
	expectedCurrentState network_v1.IPAddressState,
	expectedStatus network_v1.IPAddressStatus,
) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	gresp, err := inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().Get(ctx, tenantID, resID)
	require.NoError(tb, err)
	ip := gresp.GetResource().GetIpaddress()
	require.Equal(tb, expectedDesiredState, ip.GetDesiredState())
	require.Equal(tb, expectedCurrentState, ip.GetCurrentState())
	require.Equal(tb, expectedStatus, ip.GetStatus())
}

// AssertReconcile asserts on received events and on the executed reconciliation.
func AssertReconcile(tb testing.TB, events int) {
	tb.Helper()
	for i := 0; i < events; {
		select {
		case ev, ok := <-NetClient.Watcher:
			// Event must be received
			require.True(tb, ok, "No events received")
			if ev.Event.EventKind == inventory_v1.SubscribeEventsResponse_EVENT_KIND_DELETED {
				continue
			}
			// Verify reconciliation
			i++
			tID, resID, err := util.GetResourceKeyFromResource(ev.Event.GetResource())
			require.NoError(tb, err)
			assert.NoError(tb, IPController.Reconcile(reconcilers.NewReconcilerID(tID, resID)), "Reconciliation failed")
		case <-time.After(1 * time.Second):
			// Timeout to avoid waiting events indefinitely
			tb.Fatalf("No events received within timeout")
		}
	}
}

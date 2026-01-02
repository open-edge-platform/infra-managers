package testing

import (
	"context"
	"testing"
	"time"

	remoteaccess_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	"github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/clients"
	"github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/reconcilers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	inventory_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"

	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

// CreateRemoteAccessMgrClient is a helper function to create a new Remote Access Manager Client.
func CreateRemoteAccessMgrClient(tb testing.TB) {
	tb.Helper()
	var err error
	resourceKinds := []inventory_v1.ResourceKind{
		inventory_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF,
	}
	err = inv_testing.CreateClient(clientName, inventory_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER, resourceKinds, "")
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create Inventory RmtAccessRM client")
	}

	RmtAccessCfgClient, err = clients.NewRAInventoryClient(
		inv_testing.TestClients[clientName].GetTenantAwareInventoryClient(), inv_testing.TestClientsEvents[clientName])
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create RmtAccessRM client")
	}
	tb.Cleanup(func() {
		RmtAccessCfgClient.Stop()
		delete(inv_testing.TestClients, clientName)
		delete(inv_testing.TestClientsEvents, clientName)
	})
}

// CreateRAController is an helper function to create an Ra Controller.
func CreateRAController(tb testing.TB) {
	tb.Helper()

	raReconciler, err := reconcilers.NewRAPReconciler(RmtAccessCfgClient, nil, false)
	require.NoError(tb, err)

	RAController = rec_v2.NewController[reconcilers.ReconcilerID](raReconciler.Reconcile, rec_v2.WithParallelism(1))
	tb.Cleanup(func() { RAController.Stop() })
}

// AssertRemoteAccessConfig asserts on remoteAccessConfig state.
func AssertRemoteAccessConfig(
	tb testing.TB,
	tenantID string,
	resID string,
	expectedDesiredState remoteaccess_v1.RemoteAccessState,
	expectedCurrentState remoteaccess_v1.RemoteAccessState,
) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	gresp, err := inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().Get(ctx, tenantID, resID)
	require.NoError(tb, err)
	rmtAccessCfg := gresp.GetResource().GetRemoteAccess()
	require.Equal(tb, expectedDesiredState, rmtAccessCfg.GetDesiredState())
	require.Equal(tb, expectedCurrentState, rmtAccessCfg.GetCurrentState())
}

// AssertReconcile asserts on received events and on the executed reconciliation.
func AssertReconcile(tb testing.TB, events int) {
	tb.Helper()
	for i := 0; i < events; {
		select {
		case ev, ok := <-RmtAccessCfgClient.Watcher:
			// Event must be received
			require.True(tb, ok, "No events received")
			if ev.Event.EventKind == inventory_v1.SubscribeEventsResponse_EVENT_KIND_DELETED {
				continue
			}
			// Verify reconciliation
			i++
			tID, resID, err := util.GetResourceKeyFromResource(ev.Event.GetResource())
			require.NoError(tb, err)
			assert.NoError(tb, RAController.Reconcile(reconcilers.NewReconcilerID(tID, resID)), "Reconciliation failed")
		case <-time.After(1 * time.Second):
			// Timeout to avoid waiting events indefinitely
			tb.Fatalf("No events received within timeout")
		}
	}
}

func assertRACState(t *testing.T, tenant, resID string, wantDesired, wantCurrent remoteaccess_v1.RemoteAccessState) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	gresp, err := inv_testing.TestClients[inv_testing.APIClient].GetTenantAwareInventoryClient().Get(ctx, tenant, resID)
	require.NoError(t, err)

	got := gresp.GetResource().GetRemoteAccess()
	require.NotNil(t, got)

	require.Equal(t, wantDesired, got.GetDesiredState())
	require.Equal(t, wantCurrent, got.GetCurrentState())
}

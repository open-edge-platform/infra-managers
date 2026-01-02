package main

import (
	"sync"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"golang.org/x/net/context"
)

func main() {
	ctx := context.Background()

	events := make(chan *client.WatchEvents, 10)
	wg := &sync.WaitGroup{}
	invCli, _ := client.NewTenantAwareInventoryClient(ctx, client.InventoryClientConfig{
		Name:        "seed",
		Address:     "localhost:50051",
		Events:      events,
		Wg:          wg,
		SecurityCfg: &client.SecurityConfig{Insecure: true},
		ClientKind:  inv_v1.ClientKind_CLIENT_KIND_API,
		ResourceKinds: []inv_v1.ResourceKind{
			inv_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF,
		},
	})

	tenantID := "11111111-1111-1111-1111-111111111111"

	ra := &remoteaccessv1.RemoteAccessConfiguration{
		TenantId:            tenantID,
		DesiredState:        remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ENABLED,
		ExpirationTimestamp: uint64(time.Now().Add(20 * time.Minute).Unix()),
	}

	r := &inv_v1.Resource{
		Resource: &inv_v1.Resource_RemoteAccess{RemoteAccess: ra},
	}

	_, err := invCli.Create(ctx, tenantID, r)
	if err != nil {
		panic(err)
	}
}

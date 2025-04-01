// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package clients_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
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

// Verify NewNetInventoryClient and event processing.
func TestEventsWatcher(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	net_testing.CreateNetworkingClient(t)

	netClient := net_testing.NetClient
	host := dao.CreateHost(t, net_testing.Tenant1)
	hostNic := dao.CreateHostNic(t, net_testing.Tenant1, host)
	ipRes := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)

	select {
	case ev, ok := <-netClient.Watcher:
		require.True(t, ok, "No events received")
		assert.Equal(t, inv_v1.SubscribeEventsResponse_EVENT_KIND_CREATED, ev.Event.EventKind, "Wrong event kind")
		expectedKind, err := util.GetResourceKindFromResourceID(ev.Event.ResourceId)
		require.NoError(t, err, "resource manager did receive a strange event")
		require.Equal(t, expectedKind, inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS, "Wrong resource kind")
		tID, rID, err := util.GetResourceKeyFromResource(ev.Event.Resource)
		require.NoError(t, err)
		assert.Equal(t, net_testing.Tenant1, tID)
		assert.Equal(t, ipRes.GetResourceId(), rID)
	case <-time.After(1 * time.Second):
		// Timeout to avoid waiting events indefinitely
		t.Fatalf("No events received within timeout")
	}
}

// Verify NewNetInventoryClientWithOptions. Exit for error connections.
func TestNewNetworkClientWithOptions(t *testing.T) {
	_, err := clients.NewNetInventoryClientWithOptions(
		clients.WithInventoryAddress("foobar:500001"),
		clients.WithEnableTracing(true),
	)
	require.Error(t, err)
	grpcStatus, _ := status.FromError(err)
	require.Equal(t, codes.Unavailable, grpcStatus.Code())
}

// Verify GetIPAddressesInSite.
func TestGetIPAddressesInSite(t *testing.T) {
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	net_testing.CreateNetworkingClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	netClient := net_testing.NetClient

	ipAddresses, err := netClient.GetIPAddressesInSite(ctx, net_testing.Tenant1,
		&network_v1.IPAddressResource{
			Address: "192.168.0.1/24",
		})
	require.NoError(t, err)
	require.Equal(t, 0, len(ipAddresses))

	// IPAddress with Site
	site := dao.CreateSite(t, net_testing.Tenant1)
	host := dao.CreateHost(t, net_testing.Tenant1, inv_testing.HostSite(site))
	host.Site = site
	hostNic := dao.CreateHostNic(t, net_testing.Tenant1, host)
	hostNic.Host = host
	ipAddress := dao.CreateIPAddress(t, net_testing.Tenant1, hostNic, true)
	ipAddress.Nic = hostNic
	// IPAddress without Site
	hostNoSite := dao.CreateHost(t, net_testing.Tenant1)
	hostNicNoSite := dao.CreateHostNic(t, net_testing.Tenant1, hostNoSite)
	hostNicNoSite.Host = hostNoSite
	ipAddressNoSite := dao.CreateIPAddress(t, net_testing.Tenant1, hostNicNoSite, true)
	ipAddressNoSite.Nic = hostNicNoSite

	ipAddresses, err = netClient.GetIPAddressesInSite(ctx, net_testing.Tenant1, ipAddressNoSite)
	require.NoError(t, err)
	require.Len(t, ipAddresses, 1)
	if eq, diff := inv_testing.ProtoEqualOrDiff(ipAddressNoSite, ipAddresses[0]); !eq {
		t.Errorf("TestGetIPAddressesInSite() data not equal: %v", diff)
	}

	ipAddresses, err = netClient.GetIPAddressesInSite(ctx, net_testing.Tenant1, ipAddress)
	require.NoError(t, err)
	require.Len(t, ipAddresses, 1)
	if eq, diff := inv_testing.ProtoEqualOrDiff(ipAddress, ipAddresses[0]); !eq {
		t.Errorf("TestGetIPAddressesInSite() data not equal: %v", diff)
	}
}

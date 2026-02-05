// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package utils provides utility functions for host manager testing.
//
//nolint:revive // Package name utils is intentional for test utilities
package utils

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/host/internal/hostmgr/handlers"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
)

const (
	defaultTimeout = 120 * time.Second
	loggerName     = "HostMgrUtils"
	clientName     = "TestHrmInventoryClient"
)

// HrmClient is the inventory client for testing.
var (
	HrmClient            client.TenantAwareInventoryClient
	HrmEventsClient      chan *client.WatchEvents
	HostManagerNBHandler *handlers.HostManagerNBHandler
	zlog                 = logging.GetLogger(loggerName)
)

// CreateHostManagerClient Create a host manager southbound gRPC client.
//
//nolint:staticcheck // currently using deprecated functions.
func CreateHostManagerClient(target string, bufconnLis *bufconn.Listener) (pb.HostmgrClient, *grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if bufconnLis != nil {
		opts = append(opts,
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return bufconnLis.Dial() }))
	}

	dialOpt := grpc.WithTransportCredentials(insecure.NewCredentials())
	opts = append(opts, dialOpt)

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, nil, err
	}
	hostManagerClient := pb.NewHostmgrClient(conn)

	return hostManagerClient, conn, nil
}

// CreateNBHandler create new NB handler with its own inventory client.
func CreateNBHandler(tb testing.TB) error {
	tb.Helper()

	nbHandler, err := handlers.NewNBHandler(
		HrmClient,
		HrmEventsClient,
	)
	if err != nil {
		zlog.Error().Err(err).Msgf("Failed to create northbound handler.")
		return err
	}
	HostManagerNBHandler = nbHandler

	return nil
}

// CreateHrmClient creates and initializes the host manager test client.
func CreateHrmClient(tb testing.TB) {
	tb.Helper()

	resourceKinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_HOST,
		inv_v1.ResourceKind_RESOURCE_KIND_INSTANCE,
	}
	err := inv_testing.CreateClient(clientName, inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER, resourceKinds, "")
	if err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Inventory HRM client")
	}

	// assigning created client to global variable
	HrmClient = inv_testing.TestClients[clientName].GetTenantAwareInventoryClient()
	HrmEventsClient = inv_testing.TestClientsEvents[clientName]

	tb.Cleanup(func() {
		if err := HrmClient.Close(); err != nil {
			zlog.InfraSec().InfraErr(err).Msgf("")
		}
		zlog.InfraSec().Info().Msgf("Inventory client stopped")
		delete(inv_testing.TestClients, clientName)
		delete(inv_testing.TestClientsEvents, clientName)
	})
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"time"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/invclient"
)

var (
	zlog = logging.GetLogger("OS-Resource-Manager-Testing")
	// InvClient is the inventory client for testing.
	InvClient *invclient.InventoryClient
)

// CreateInventoryClientForTesting is an helper function to create a new client.
func CreateInventoryClientForTesting() {
	var err error
	InvClient, err = invclient.NewOSRMInventoryClient(
		inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient(),
		inv_testing.TestClientsEvents[inv_testing.RMClient], nil)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Inventory client")
	}
}

// DeleteInventoryClientForTesting deletes the test inventory client.
func DeleteInventoryClientForTesting() {
	InvClient.Close()
	time.Sleep(1 * time.Second)
	delete(inv_testing.TestClients, inv_testing.RMClient)
	delete(inv_testing.TestClientsEvents, inv_testing.RMClient)
}

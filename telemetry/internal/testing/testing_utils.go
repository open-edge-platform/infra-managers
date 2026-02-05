// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package testing provides testing utilities for telemetry manager.
package testing

import (
	"testing"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

const (
	clientName = "TestTelemetryInventoryClient"
	loggerName = "TestLogger"
)

var (
	// Zlog is the logger for testing.
	Zlog = logging.GetLogger(loggerName)
	// TelemetryClient is the telemetry inventory client for testing.
	TelemetryClient *invclient.TelemetryInventoryClient
)

// CreateTelemetryClientForTesting creates a telemetry client for testing.
func CreateTelemetryClientForTesting(tb testing.TB) {
	tb.Helper()
	var err error
	resourceKinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE,
	}
	err = inv_testing.CreateClient(clientName, inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER, resourceKinds, "")
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create Inventory NetRM client")
	}

	TelemetryClient, err = invclient.NewTelemetryInventoryClient(
		inv_testing.TestClients[clientName].GetTenantAwareInventoryClient(),
		invclient.NewTelemetryCacheClient(inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()),
		inv_testing.TestClientsEvents[clientName],
	)
	if err != nil {
		Zlog.Fatal().Err(err).Msg("Cannot create TelRM client")
	}
	tb.Cleanup(func() {
		TelemetryClient.Close()
		delete(inv_testing.TestClients, clientName)
		delete(inv_testing.TestClientsEvents, clientName)
	})
}

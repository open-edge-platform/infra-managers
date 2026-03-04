// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package testing provides testing utilities for the networking manager.
package testing

import (
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
	"github.com/open-edge-platform/infra-managers/networking/internal/handlers"
	"github.com/open-edge-platform/infra-managers/networking/internal/reconcilers"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

const (
	clientName = "TestNetInventoryClient"
	loggerName = "TestLogger"
)

const (
	// Tenant1 is the first test tenant identifier.
	Tenant1 = "11111111-1111-1111-1111-111111111111"
	// Tenant2 is the second test tenant identifier.
	Tenant2 = "22222222-2222-2222-2222-222222222222"
)

var (
	// Zlog is the logger instance for testing.
	Zlog = logging.GetLogger(loggerName)
	// NetClient is the network inventory client for testing.
	NetClient *clients.NetInventoryClient
	// IPController is the IP controller for testing.
	IPController *rec_v2.Controller[reconcilers.ReconcilerID]
	// NBHandler is the northbound handler for testing.
	NBHandler *handlers.NBHandler
)

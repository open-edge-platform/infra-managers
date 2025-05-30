// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

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
	Tenant1 = "11111111-1111-1111-1111-111111111111"
	Tenant2 = "22222222-2222-2222-2222-222222222222"
)

var (
	Zlog         = logging.GetLogger(loggerName)
	NetClient    *clients.NetInventoryClient
	IPController *rec_v2.Controller[reconcilers.ReconcilerID]
	NBHandler    *handlers.NBHandler
)

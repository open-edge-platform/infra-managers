// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package northbound_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	nb "github.com/open-edge-platform/infra-managers/telemetry/internal/handlers/northbound"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

func TestNewNBHandler(t *testing.T) {
	telCli := &invclient.TelemetryInventoryClient{}
	nbHandler, err := nb.NewNBHandler(telCli)

	assert.NoError(t, err)
	assert.NotNil(t, nbHandler)
}

func TestStartAndStop(t *testing.T) {
	telCli := &invclient.TelemetryInventoryClient{}
	nbHandler, err := nb.NewNBHandler(telCli)
	assert.NoError(t, err)

	out := nbHandler.Start()
	assert.NoError(t, out)

	time.Sleep(100 * time.Millisecond)

	nbHandler.Stop()
}

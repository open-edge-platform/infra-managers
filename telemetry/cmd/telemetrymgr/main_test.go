// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

func TestHostManager_InvClient(t *testing.T) {
	wg := sync.WaitGroup{}

	_, err := invclient.NewTelemetryInventoryClientWithOptions(
		&wg,
		invclient.WithInventoryAddress("localhost:50051"),
		invclient.WithEnableTracing(false),
	)
	require.Error(t, err)

	wg.Wait()
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package maintmgr_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/maintmgr"
)

func TestMaintManager_InvClient(t *testing.T) {
	wg := sync.WaitGroup{}

	// Error with invalid Inventory Address
	err := maintmgr.StartInvGrpcCli(&wg, true, "", "", "", "", true, false, 0, false)
	require.Error(t, err)

	invGrpcClient := invclient.NewInvGrpcClient(nil, nil)
	require.NotNil(t, invGrpcClient)
	maintmgr.SetInvGrpcCli(invGrpcClient)
}

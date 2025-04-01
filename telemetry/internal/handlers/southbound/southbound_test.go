// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package southbound_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	sb "github.com/open-edge-platform/infra-managers/telemetry/internal/handlers/southbound"
	telemetry_testing "github.com/open-edge-platform/infra-managers/telemetry/internal/testing"
)

var rbacRules = "../../../rego/authz.rego"

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(wd)))
	policyPath := projectRoot + "/out"
	migrationsDir := projectRoot + "/out"

	inv_testing.StartTestingEnvironment(policyPath, "", migrationsDir)
	run := m.Run() // run all tests
	inv_testing.StopTestingEnvironment()

	os.Exit(run)
}

func TestNewSBHandler(t *testing.T) {
	// create telemetryclient
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient
	sbHandler, err := sb.NewSBHandler("localhost:0", nil, false,
		telemetryInvClient, true, rbacRules, true, true, false, "")

	assert.NoError(t, err)
	assert.NotNil(t, sbHandler)
}

func TestStartAndStop(t *testing.T) {
	// create telemetryclient
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient
	sbHandler, err := sb.NewSBHandler("localhost:0", nil, false,
		telemetryInvClient, true, rbacRules, true, true, false, "")

	assert.NoError(t, err)

	out := sbHandler.Start()
	assert.NoError(t, out)

	sbHandler.Stop()
	time.Sleep(1 * time.Second)
}

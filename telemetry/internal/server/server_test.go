// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/server"
	telemetry_testing "github.com/open-edge-platform/infra-managers/telemetry/internal/testing"
)

var rbacRules = "../../rego/authz.rego"

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

func TestStartTelemetrymgrGrpcServer(t *testing.T) {
	// Create a mock listener
	lis, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer func() {
		if closeErr := lis.Close(); closeErr != nil {
			t.Logf("Failed to close listener: %v", closeErr)
		}
	}()

	// Create a mock wait group
	wg := &sync.WaitGroup{}

	// create telemetryclient
	telemetry_testing.CreateTelemetryClientForTesting(t)
	telemetryInvClient := telemetry_testing.TelemetryClient

	// Create channels for termination and readiness signals
	termChan := make(chan bool)
	readyChan := make(chan bool)

	// Start the server in a separate goroutine
	go server.StartTelemetrymgrGrpcServer(termChan, readyChan,
		wg, lis, telemetryInvClient,
		server.EnableTracing(false),
		server.EnableAuth(true),
		server.WithRbacRulesPath(rbacRules),
		server.EnableValidate(true),
		server.EnableSanitizeGrpcErr(true),
	)

	// Wait for the server to be ready
	<-readyChan

	// Send a termination signal
	termChan <- true

	// Wait for the server to stop
	wg.Wait()

	// Check if the listener is closed
	_, err = lis.Accept()
	assert.Error(t, err)
}

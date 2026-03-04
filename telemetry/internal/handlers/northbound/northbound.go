// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package northbound provides northbound handler for telemetry manager.
package northbound

import (
	"sync"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
)

// Misc variables.
var (
	loggerName = "NBHandler"
	zlog       = logging.GetLogger(loggerName)
)

// NBHandler is responsible for checking Telemetry connection with Inventory.
type NBHandler struct {
	telemetryClient *invclient.TelemetryInventoryClient
	wg              *sync.WaitGroup
	sigTerm         chan bool
}

// NewNBHandler creates a new northbound handler.
func NewNBHandler(telCli *invclient.TelemetryInventoryClient) (*NBHandler, error) {
	// Note that the resourceID from the events will provide the mapping
	nbHandler := &NBHandler{
		telemetryClient: telCli,
		wg:              &sync.WaitGroup{},
		sigTerm:         make(chan bool),
	}
	return nbHandler, nil
}

// Start starts the northbound handler.
func (nbh *NBHandler) Start() error {
	// 1. Start controlLoop
	// 2. start by watching events
	nbh.wg.Add(1)
	go nbh.controlLoop()
	zlog.InfraSec().Info().Msgf("NB handler started")
	return nil
}

// Stop stops the NB handler and its control loop.
func (nbh *NBHandler) Stop() {
	// 1. Take down the controlLoop
	// 2. Wait the end of the control loop
	// 3. Graceful shutdown of the controllers
	close(nbh.sigTerm)
	nbh.wg.Wait()
	zlog.InfraSec().Info().Msgf("NB handler stopped")
}

// NB control loop implementation.
func (nbh *NBHandler) controlLoop() {
	for {
		select {
		case _, ok := <-nbh.telemetryClient.Watcher:
			if !ok {
				// Event channel is closed, stream ended. Bye!
				// Note this will cover the sigterm scenario as well
				zlog.InfraSec().Fatal().Msg("gRPC stream with inventory closed")
			}
		case <-nbh.sigTerm:
			// signal done
			nbh.wg.Done()
			return
		}
	}
}

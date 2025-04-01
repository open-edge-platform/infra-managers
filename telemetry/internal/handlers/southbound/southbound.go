// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package southbound

import (
	"net"
	"sync"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/server"
)

// Misc variables.
var (
	loggerName = "SBHandler"
	zlog       = logging.GetLogger(loggerName)
)

type SBHandler struct {
	servaddr              string
	termChan              chan bool
	readyChan             chan bool
	wg                    *sync.WaitGroup
	enableTracing         bool
	telCli                *invclient.TelemetryInventoryClient
	enableAuth            bool
	rbacRules             string
	enableVal             bool
	enableSanitizeGrpcErr bool
	enableMetrics         bool
	metricsAddress        string
}

func NewSBHandler(
	servaddr string,
	readyChan chan bool,
	enableTracing bool,
	telCli *invclient.TelemetryInventoryClient,
	enableAuth bool,
	rbacRules string,
	enableVal bool,
	enableSanitizeGrpcErr bool,
	enableMetrics bool,
	metricsAddress string,
) (*SBHandler, error) {
	sbHandler := &SBHandler{
		servaddr:              servaddr,
		enableTracing:         enableTracing,
		wg:                    &sync.WaitGroup{},
		termChan:              make(chan bool),
		readyChan:             readyChan,
		telCli:                telCli,
		enableAuth:            enableAuth,
		rbacRules:             rbacRules,
		enableVal:             enableVal,
		enableSanitizeGrpcErr: enableSanitizeGrpcErr,
		enableMetrics:         enableMetrics,
		metricsAddress:        metricsAddress,
	}
	return sbHandler, nil
}

func (sbh *SBHandler) Start() error {
	// start gRPC server for southbound
	go func() {
		lis, err := net.Listen("tcp", sbh.servaddr)
		if err != nil {
			zlog.InfraSec().Fatal().Err(err).Msgf("Error listening with TCP on address %s", sbh.servaddr)
		}
		server.StartTelemetrymgrGrpcServer(
			sbh.termChan, sbh.readyChan, sbh.wg, lis, sbh.telCli,
			server.EnableTracing(sbh.enableTracing),
			server.EnableAuth(sbh.enableAuth),
			server.WithRbacRulesPath(sbh.rbacRules),
			server.EnableSanitizeGrpcErr(sbh.enableSanitizeGrpcErr),
			server.EnableValidate(sbh.enableVal),
			server.EnableMetrics(sbh.enableMetrics),
			server.WithMetricsAddress(sbh.metricsAddress),
		)
	}()
	zlog.InfraSec().Info().Msgf("SB handler started")
	return nil
}

func (sbh *SBHandler) Stop() {
	// stop gRPC server
	close(sbh.termChan)
	sbh.wg.Wait()
	zlog.InfraSec().Info().Msgf("SB handler stopped")
}

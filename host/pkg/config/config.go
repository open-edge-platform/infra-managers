// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration structures and validation for the Host Manager.
package config

import (
	"net"
	"time"

	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
)

// HostMgrConfig contains configuration for the host manager..
type HostMgrConfig struct {
	EnableTracing       bool
	EnableMetrics       bool
	TraceURL            string
	InventoryAddr       string
	CACertPath          string
	TLSKeyPath          string
	TLSCertPath         string
	InsecureGRPC        bool
	EnableHostDiscovery bool
	EnableUUIDCache     bool
	UUIDCacheTTL        time.Duration
	UUIDCacheTTLOffset  int
}

// Validate checks if the configuration is valid.
func (c HostMgrConfig) Validate() error {
	if c.InventoryAddr == "" {
		return inv_errors.Errorfc(codes.InvalidArgument,
			"empty inventory address: %s", c.InventoryAddr)
	}

	_, err := net.ResolveTCPAddr("tcp", c.InventoryAddr)
	if err != nil {
		return inv_errors.Errorfc(codes.InvalidArgument,
			"invalid inventory address %s: %s", c.InventoryAddr, err)
	}

	if !c.InsecureGRPC {
		if c.CACertPath == "" || c.TLSCertPath == "" || c.TLSKeyPath == "" {
			return inv_errors.Errorfc(codes.InvalidArgument,
				"gRPC connections should be secure, but one of secrets is not provided")
		}
	}

	return nil
}

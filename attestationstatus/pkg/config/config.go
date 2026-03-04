// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package config provides configuration structures and validation for the Attestation Status Manager.
package config

import (
	"net"

	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
)

// AttestationStatusMgrConfig contains configuration for the Attestation Status Manager.
type AttestationStatusMgrConfig struct {
	EnableMetrics bool
	EnableTracing bool
	TraceURL      string
	InventoryAddr string
	CACertPath    string
	TLSKeyPath    string
	TLSCertPath   string
	InsecureGRPC  bool
}

// Validate checks if the configuration is valid.
func (c AttestationStatusMgrConfig) Validate() error {
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

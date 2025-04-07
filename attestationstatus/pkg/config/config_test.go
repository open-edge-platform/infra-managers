// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/open-edge-platform/infra-managers/attestationstatus/pkg/config"
)

func TestAttestationStatusMgrConfig_Validate(t *testing.T) {
	type fields struct {
		EnableMetrics bool
		EnableTracing bool
		TraceURL      string
		InventoryAddr string
		CACertPath    string
		TLSKeyPath    string
		TLSCertPath   string
		InsecureGRPC  bool
	}
	tests := []struct {
		name       string
		fields     fields
		wantErr    bool
		grpcStatus codes.Code
	}{
		{
			name: "Success",
			fields: fields{
				InventoryAddr: "localhost:50051",
				CACertPath:    "",
				TLSKeyPath:    "",
				TLSCertPath:   "",
				InsecureGRPC:  true,
			},
			wantErr: false,
		},
		{
			name: "Failed_EmptyInventoryAddress",
			fields: fields{
				InventoryAddr: "",
				InsecureGRPC:  true,
			},
			wantErr:    true,
			grpcStatus: codes.InvalidArgument,
		},
		{
			name: "Failed_InvalidInventoryAddress",
			fields: fields{
				InventoryAddr: "192.168.0.1",
				InsecureGRPC:  true,
			},
			wantErr:    true,
			grpcStatus: codes.InvalidArgument,
		},
		{
			name: "Failed_NoSecrets",
			fields: fields{
				InventoryAddr: "localhost:50051",
				CACertPath:    "",
				TLSKeyPath:    "",
				TLSCertPath:   "",
				InsecureGRPC:  false,
			},
			wantErr:    true,
			grpcStatus: codes.InvalidArgument,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.AttestationStatusMgrConfig{
				EnableMetrics: tt.fields.EnableMetrics,
				EnableTracing: tt.fields.EnableTracing,
				TraceURL:      tt.fields.TraceURL,
				InventoryAddr: tt.fields.InventoryAddr,
				CACertPath:    tt.fields.CACertPath,
				TLSKeyPath:    tt.fields.TLSKeyPath,
				TLSCertPath:   tt.fields.TLSCertPath,
				InsecureGRPC:  tt.fields.InsecureGRPC,
			}
			if err := c.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				if tt.wantErr {
					require.Equal(t, tt.grpcStatus, status.Code(err))
				}
			}
		})
	}
}

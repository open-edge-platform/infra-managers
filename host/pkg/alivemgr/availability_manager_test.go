// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package alivemgr_test

import (
	"errors"
	"testing"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
)

func TestStartAlvMgr(t *testing.T) {
	tests := []struct {
		name string
		args chan bool
	}{
		{
			name: "succ",
			args: make(chan bool),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(_ *testing.T) {
			alivemgr.StartAlvMgr(tt.args)
		})
	}
}

func TestUpdateHostHeartBeat(t *testing.T) {
	type args struct {
		host     *computev1.HostResource
		termChan chan bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr error
	}{
		{
			name: "succ",
			args: args{
				host:     &computev1.HostResource{ResourceId: "11111111", TenantId: "11111111-1111-1111-1111-111111111111"},
				termChan: make(chan bool),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ret := alivemgr.UpdateHostHeartBeat(tt.args.host)
			t.Logf("UpdateHostHeartBeat return %s with %t", ret, ret)
			if !errors.Is(ret, tt.wantErr) {
				t.Errorf("UpdateHostHeartBeat() error = %v, wantErr %v", ret, tt.wantErr)
				return
			}

			ret = alivemgr.UpdateHostHeartBeat(tt.args.host)
			t.Logf("UpdateHostHeartBeat return %s with %t", ret, ret)
			if !errors.Is(ret, tt.wantErr) {
				t.Errorf("UpdateHostHeartBeat() error = %v, wantErr %v", ret, tt.wantErr)
				return
			}
		})
	}
}

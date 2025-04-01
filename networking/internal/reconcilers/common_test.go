// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers_test

import (
	"errors"
	"testing"

	"github.com/onosproject/onos-lib-go/pkg/controller/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-managers/networking/internal/reconcilers"
	"github.com/open-edge-platform/infra-managers/networking/internal/utils"
)

const (
	testTenantID   = "tID"
	testResourceID = "rID"
)

func Test_handleInventoryError(t *testing.T) {
	request := controller.Request[reconcilers.ReconcilerID]{}
	request.ID = "some ID"

	type args struct {
		err error
	}
	tests := []struct {
		name          string
		args          args
		wantDirective bool
		wantType      interface{}
	}{
		{
			name: "NoError",
			args: args{
				err: nil,
			},
			wantDirective: false,
		},
		{
			name: "NotFound",
			args: args{
				err: inv_errors.Errorfc(codes.NotFound, ""),
			},
			wantDirective: true,
			wantType:      &controller.Ack[reconcilers.ReconcilerID]{},
		},
		{
			name: "PermissionDenied",
			args: args{
				err: inv_errors.Errorfc(codes.PermissionDenied, ""),
			},
			wantDirective: true,
			wantType:      &controller.Ack[reconcilers.ReconcilerID]{},
		},
		{
			name: "AlreadyExists",
			args: args{
				err: inv_errors.Errorfc(codes.AlreadyExists, ""),
			},
			wantDirective: true,
			wantType:      &controller.Ack[reconcilers.ReconcilerID]{},
		},
		{
			name: "Unauthenticated",
			args: args{
				err: inv_errors.Errorfc(codes.Unauthenticated, ""),
			},
			wantDirective: true,
			wantType:      &controller.Ack[reconcilers.ReconcilerID]{},
		},
		{
			name: "OtherGRPCError",
			args: args{
				err: inv_errors.Errorfc(codes.Internal, ""),
			},
			wantDirective: true,
			wantType:      &controller.RetryWith[reconcilers.ReconcilerID]{},
		},
		{
			name: "NonInventoryError",
			args: args{
				err: errors.New(""),
			},
			wantDirective: true,
			wantType:      &controller.Ack[reconcilers.ReconcilerID]{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directive := reconcilers.HandleInventoryError(tt.args.err, request)
			require.Equal(t, directive != nil, tt.wantDirective)
			if tt.wantDirective {
				assert.IsType(t, tt.wantType, directive)
			}
		})
	}
}

func Test_ReconcilerID(t *testing.T) {
	t.Run("TestNewReconcilerID", func(t *testing.T) {
		id := reconcilers.NewReconcilerID("", "")
		assert.Equal(t, "_", string(id))

		id = reconcilers.NewReconcilerID(testTenantID, testResourceID)
		assert.Equal(t, "tID_rID", string(id))
	})

	t.Run("TestGetTenantID", func(t *testing.T) {
		id := reconcilers.NewReconcilerID("", testResourceID)
		assert.Equal(t, "", id.GetTenantID())
		id = reconcilers.NewReconcilerID(testTenantID, testResourceID)
		assert.Equal(t, "tID", id.GetTenantID())
	})

	t.Run("TestGetResourceID", func(t *testing.T) {
		id := reconcilers.NewReconcilerID(testTenantID, "")
		assert.Equal(t, "", id.GetResourceID())
		id = reconcilers.NewReconcilerID(testTenantID, testResourceID)
		assert.Equal(t, testResourceID, id.GetResourceID())
	})

	t.Run("TestString", func(t *testing.T) {
		id := reconcilers.NewReconcilerID(testTenantID, testResourceID)
		assert.Equal(t, utils.FormatTenantResourceID(testTenantID, testResourceID), id.String())
	})
}

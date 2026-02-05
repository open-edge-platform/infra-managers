// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package errors provides custom error types for the maintenance manager.
//
//nolint:revive // Package name intentionally conflicts with stdlib for custom error handling
package errors

import (
	"errors"

	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
)

// Wrap wraps the error by carrying it over a grpc status. If no error matches an expected error from Maintenance Manager
// the error is wrapped with Wrap helper from inventory.
//
// err is the error to be wrapped.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case
		errors.As(err, &pb.UpdateStatusMultiError{}),
		errors.As(err, &pb.UpdateStatusValidationError{}),
		errors.As(err, &pb.PlatformUpdateStatusRequestMultiError{}),
		errors.As(err, &pb.PlatformUpdateStatusRequestValidationError{}),
		errors.As(err, &pb.SingleScheduleMultiError{}),
		errors.As(err, &pb.SingleScheduleValidationError{}),
		errors.As(err, &pb.RepeatedScheduleMultiError{}),
		errors.As(err, &pb.RepeatedScheduleValidationError{}),
		errors.As(err, &pb.UpdateScheduleMultiError{}),
		errors.As(err, &pb.UpdateScheduleValidationError{}),
		errors.As(err, &pb.PlatformUpdateStatusResponseMultiError{}),
		errors.As(err, &pb.PlatformUpdateStatusResponseValidationError{}),
		errors.As(err, &pb.UpdateSourceMultiError{}),
		errors.As(err, &pb.UpdateSourceValidationError{}):
		return inv_errors.Errorfc(codes.InvalidArgument, "%s", err.Error())
	}
	// No wrapping from maintenance manager, fallback to wrap from Inventory
	return inv_errors.Wrap(err)
}

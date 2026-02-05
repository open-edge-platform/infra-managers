// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package errors provides error wrapping utilities for the Host Manager.
//
//nolint:revive // Package name intentionally conflicts with stdlib for custom error handling
package errors

import (
	"errors"

	"google.golang.org/grpc/codes"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
)

// Wrap wraps an error with additional context and status code.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	// common handler for the validation errors
	switch {
	case errors.As(err, &pb.SystemNetworkMultiError{}),
		errors.As(err, &pb.SystemNetworkValidationError{}),
		errors.As(err, &pb.IPAddressMultiError{}),
		errors.As(err, &pb.IPAddressValidationError{}),
		errors.As(err, &pb.SystemInfoValidationError{}),
		errors.As(err, &pb.SystemInfoMultiError{}),
		errors.As(err, &pb.BiosInfoValidationError{}),
		errors.As(err, &pb.BiosInfoMultiError{}),
		errors.As(err, &pb.UpdateInstanceStateStatusByHostGUIDRequestMultiError{}),
		errors.As(err, &pb.UpdateInstanceStateStatusByHostGUIDRequestValidationError{}),
		errors.As(err, &pb.UpdateHostStatusByHostGuidRequestMultiError{}),
		errors.As(err, &pb.UpdateHostStatusByHostGuidRequestValidationError{}),
		errors.As(err, &pb.UpdateHostSystemInfoByGUIDRequestMultiError{}),
		errors.As(err, &pb.UpdateHostSystemInfoByGUIDRequestValidationError{}):
		return inv_errors.Errorfc(codes.InvalidArgument, "%s", err.Error())
	}
	return inv_errors.Wrap(err)
}

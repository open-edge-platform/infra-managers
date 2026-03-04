// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package errors provides error handling utilities for the networking manager.
//
//nolint:revive // errors is an appropriate name for error handling package
package errors

import (
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	_ "github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging" // otherwise init() is not called
)

// Wrap wraps an error with inventory error handling.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	// common handler for the validation errors
	return inv_errors.Wrap(err)
}

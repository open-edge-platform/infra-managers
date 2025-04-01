// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package errors

import (
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	_ "github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging" // otherwise init() is not called
)

func Wrap(err error) error {
	if err == nil {
		return nil
	}
	// common handler for the validation errors
	return inv_errors.Wrap(err)
}

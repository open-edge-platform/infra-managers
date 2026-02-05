// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package utils provides utility functions for the networking manager.
//
//nolint:revive // utils is an acceptable package name for utility functions
package utils

import "fmt"

// FormatTenantResourceID formats a tenant and resource ID into a display string.
func FormatTenantResourceID(tenantID, resourceID string) string {
	return fmt.Sprintf("[tenantID=%s, resourceID=%s]", tenantID, resourceID)
}

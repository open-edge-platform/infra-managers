// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package utils

import "fmt"

func FormatTenantResourceID(tenantID, resourceID string) string {
	return fmt.Sprintf("[tenantID=%s, resourceID=%s]", tenantID, resourceID)
}

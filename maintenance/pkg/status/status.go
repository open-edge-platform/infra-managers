// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
)

var (
	StatusUnknown     = "Unknown"
	StatusUpdating    = "Updating"
	StatusCompleted   = "Update completed"
	StatusFailed      = "Update failed"
	StatusUpToDate    = "No new updates available"
	StatusDownloading = "Downloading artifacts"
	StatusDownloaded  = "Download complete"

	UpdateStatusUnknown     = inv_status.New(StatusUnknown, statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	UpdateStatusInProgress  = inv_status.New(StatusUpdating, statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	UpdateStatusDone        = inv_status.New(StatusCompleted, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	UpdateStatusFailed      = inv_status.New(StatusFailed, statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	UpdateStatusUpToDate    = inv_status.New(StatusUpToDate, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	UpdateStatusDownloading = inv_status.New(StatusDownloading, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	UpdateStatusDownloaded  = inv_status.New(StatusDownloaded, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
)

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package status provides status types for the maintenance manager.
package status

import (
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
)

var (
	// StatusUnknown represents an unknown status.
	StatusUnknown = "Unknown"
	// StatusUpdating represents an update in progress status.
	StatusUpdating = "Updating"
	// StatusCompleted represents a completed update status.
	StatusCompleted = "Update completed"
	// StatusFailed represents a failed update status.
	StatusFailed = "Update failed"
	// StatusUpToDate represents a status where no updates are available.
	StatusUpToDate = "No new updates available"
	// StatusDownloading represents a status where artifacts are being downloaded.
	StatusDownloading = "Downloading artifacts"
	// StatusDownloaded represents a status where download is complete.
	StatusDownloaded = "Download complete"

	// UpdateStatusUnknown represents an unknown update status with unspecified indication.
	UpdateStatusUnknown = inv_status.New(StatusUnknown, statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	// UpdateStatusInProgress represents an in-progress update status.
	UpdateStatusInProgress = inv_status.New(StatusUpdating, statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	// UpdateStatusDone represents a completed update status.
	UpdateStatusDone = inv_status.New(StatusCompleted, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// UpdateStatusFailed represents a failed update status.
	UpdateStatusFailed = inv_status.New(StatusFailed, statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	// UpdateStatusUpToDate represents an up-to-date status with no updates needed.
	UpdateStatusUpToDate = inv_status.New(StatusUpToDate, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// UpdateStatusDownloading represents a downloading update status.
	UpdateStatusDownloading = inv_status.New(StatusDownloading, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// UpdateStatusDownloaded represents a downloaded update status.
	UpdateStatusDownloaded = inv_status.New(StatusDownloaded, statusv1.StatusIndication_STATUS_INDICATION_IDLE)
)

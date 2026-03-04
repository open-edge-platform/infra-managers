// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package status provides status-related utilities for the Host Manager.
package status

import (
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
)

var (
	// HostStatusEmpty indicates not yet initialized host status. Should be only used for testing.
	HostStatusEmpty = inv_status.New("", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	// HostStatusUnknown represents an unknown host status.
	HostStatusUnknown = inv_status.New("Unknown", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	// HostStatusNoConnection represents a host with no connection.
	HostStatusNoConnection = inv_status.New("No Connection", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	// HostStatusRunning represents a host that is running.
	HostStatusRunning = inv_status.New("Running", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// HostStatusBooting represents a host that is booting.
	HostStatusBooting = inv_status.New("Booting", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	// HostStatusError represents a host in error state.
	HostStatusError = inv_status.New("Error", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	// HostStatusInvalidating represents a host being invalidated.
	HostStatusInvalidating = inv_status.New("Invalidating", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	// HostStatusInvalidated represents a host that has been invalidated.
	HostStatusInvalidated = inv_status.New("Invalidated", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// HostStatusDeleting represents a host being deleted.
	HostStatusDeleting = inv_status.New("Deleting", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)

	// InstanceStatusEmpty represents an empty instance status (for testing).
	InstanceStatusEmpty = inv_status.New("", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	// InstanceStatusUnknown represents an unknown instance status.
	InstanceStatusUnknown = inv_status.New("Unknown", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	// InstanceStatusRunning represents a running instance.
	InstanceStatusRunning = inv_status.New("Running", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	// InstanceStatusError represents an instance in error state.
	InstanceStatusError = inv_status.New("Error", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	// InstanceStatusInitializing represents an instance being initialized.
	InstanceStatusInitializing = inv_status.New("Initializing", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
)

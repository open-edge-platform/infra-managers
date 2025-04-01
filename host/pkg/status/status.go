// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
)

var (
	// HostStatusEmpty indicates not yet initialized host status. Should be only used for testing.
	HostStatusEmpty        = inv_status.New("", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	HostStatusUnknown      = inv_status.New("Unknown", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	HostStatusNoConnection = inv_status.New("No Connection", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	HostStatusRunning      = inv_status.New("Running", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	HostStatusBooting      = inv_status.New("Booting", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	HostStatusError        = inv_status.New("Error", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
	HostStatusInvalidating = inv_status.New("Invalidating", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)
	HostStatusInvalidated  = inv_status.New("Invalidated", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	HostStatusDeleting     = inv_status.New("Deleting", statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS)

	InstanceStatusEmpty   = inv_status.New("", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	InstanceStatusUnknown = inv_status.New("Unknown", statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED)
	InstanceStatusRunning = inv_status.New("Running", statusv1.StatusIndication_STATUS_INDICATION_IDLE)
	InstanceStatusError   = inv_status.New("Error", statusv1.StatusIndication_STATUS_INDICATION_ERROR)
)

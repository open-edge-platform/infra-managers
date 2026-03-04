// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package reconcilers provides reconciliation logic for networking resources.
package reconcilers

import (
	"fmt"
	"strings"
	"time"

	grpc_status "google.golang.org/grpc/status"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-managers/networking/internal/utils"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

// Constants used for the exp retries.
const (
	minDelay = 1 * time.Second
	maxDelay = 30 * time.Second
)

// ReconcilerID represents a reconciler identifier, uniquely identifying a reconciler instance for a specific tenant and resource.
type ReconcilerID string

func (id ReconcilerID) String() string {
	return utils.FormatTenantResourceID(id.GetTenantID(), id.GetResourceID())
}

// GetTenantID returns the tenant ID from the reconciler ID.
func (id ReconcilerID) GetTenantID() string {
	return strings.Split(string(id), "_")[0]
}

// GetResourceID returns the resource ID from the reconciler ID.
func (id ReconcilerID) GetResourceID() string {
	return strings.Split(string(id), "_")[1]
}

// NewReconcilerID creates a new ReconcilerID from tenant and resource IDs.
func NewReconcilerID(tenantID, resourceID string) ReconcilerID {
	return ReconcilerID(fmt.Sprintf("%s_%s", tenantID, resourceID))
}

// HandleInventoryError is a generic handler for inventory errors. It should only be used by reconciliation logic.
func HandleInventoryError(err error, request rec_v2.Request[ReconcilerID]) rec_v2.Directive[ReconcilerID] {
	if _, ok := grpc_status.FromError(err); !ok {
		// not a gRPC error, might be some internal error, no retry
		zlog.InfraErr(err).Msgf("Received non-gRPC error for request ID %s, no retry", request.ID)
		return request.Ack()
	}

	// Not found, already_exists, unauthenticated and permission_denied are fine or non-recoverable. No retries
	switch {
	case inv_errors.IsNotFound(err) || inv_errors.IsAlreadyExists(err):
		zlog.InfraErr(err).Msgf("Received non-transient, but safe error for request ID %s, "+
			"stopping reconciliation", request.ID)
		return request.Ack()
	case inv_errors.IsUnauthenticated(err) || inv_errors.IsPermissionDenied(err):
		// unrecoverable errors
		zlog.InfraErr(err).Msgf(
			"Received non-transient, unrecoverable inventory error for request ID %s, "+
				"stopping reconciliation", request.ID)
		return request.Ack()
	}

	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Retry reconciliation %s after exp. backoff", request.ID)
		return request.Retry(err).With(rec_v2.ExponentialBackoff(minDelay, maxDelay))
	}

	return nil
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package reconcilers provides reconciliation logic for OS resources.
package reconcilers

import (
	"strings"
	"time"

	grpc_status "google.golang.org/grpc/status"

	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

const (
	minDelay = 1 * time.Second
	maxDelay = 60 * time.Second
)

// ReconcilerID represents a unique identifier for a reconciler.
type ReconcilerID string

// WrapReconcilerID creates a ReconcilerID from a string.
func WrapReconcilerID(tenantID, resourceID string) ReconcilerID {
	return ReconcilerID(tenantID + "/" + resourceID)
}

// UnwrapReconcilerID extracts the string from a ReconcilerID.
func UnwrapReconcilerID(id ReconcilerID) (string, string) {
	unwrapped := strings.Split(id.String(), "/")
	return unwrapped[0], unwrapped[1]
}

func (id ReconcilerID) String() string {
	return string(id)
}

// HandleInventoryError handles errors from inventory operations during reconciliation.
func HandleInventoryError(err error, request rec_v2.Request[ReconcilerID]) rec_v2.Directive[ReconcilerID] {
	if _, ok := grpc_status.FromError(err); !ok {
		return request.Ack()
	}

	if inv_errors.IsNotFound(err) || inv_errors.IsAlreadyExists(err) ||
		inv_errors.IsUnauthenticated(err) || inv_errors.IsPermissionDenied(err) {
		return request.Ack()
	}

	if err != nil {
		return request.Retry(err).With(rec_v2.ExponentialBackoff(minDelay, maxDelay))
	}

	return nil
}

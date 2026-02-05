// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers

import (
	"context"

	"google.golang.org/protobuf/proto"

	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tracing"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

// Misc variables.
var (
	loggerName = "IPReconciler"
	zlog       = logging.GetLogger(loggerName)
)

// IPReconciler reconciles IP address resources for networking.
type IPReconciler struct {
	netClient      *clients.NetInventoryClient
	tracingEnabled bool
}

// NewIPReconciler creates a new IP reconciler.
func NewIPReconciler(cl *clients.NetInventoryClient, tracingEnabled bool) (*IPReconciler, error) {
	ipRec := &IPReconciler{
		netClient:      cl,
		tracingEnabled: tracingEnabled,
	}

	return ipRec, nil
}

// Reconciliation helper to verify if reconciliation is needed.
func (ipr *IPReconciler) skipReconciliation(request rec_v2.Request[ReconcilerID], ip *network_v1.IPAddressResource,
) rec_v2.Directive[ReconcilerID] {
	if ip.DesiredState == ip.CurrentState {
		return request.Ack()
	}
	// TODO ITEP-618
	return nil
}

// Reconcile implements the main reconcile logic.
func (ipr *IPReconciler) Reconcile(ctx context.Context, request rec_v2.Request[ReconcilerID]) rec_v2.Directive[ReconcilerID] {
	if ipr.tracingEnabled {
		ctx = tracing.StartTrace(ctx, "NetworkingManager", "IPReconciler")
		defer tracing.StopTrace(ctx)
	}
	resourceID := request.ID.GetResourceID()
	tenantID := request.ID.GetTenantID()
	zlog.Debug().Msgf("Reconciling IPAddress %s", resourceID)
	// Retrieve the Address first
	ip, err := ipr.netClient.GetIPAddress(ctx, tenantID, resourceID)
	directive := HandleInventoryError(err, request)
	if directive != nil {
		return directive
	}
	// Reconcile the Address
	directive = ipr.skipReconciliation(request, ip)
	if directive != nil {
		zlog.Debug().Msgf("IP %s reconciliation skipped", request.ID)
		return directive
	}
	return ipr.reconcileIPAddress(ctx, request, ip)
}

// IP reconcile logic.
func (ipr *IPReconciler) reconcileIPAddress(ctx context.Context, request rec_v2.Request[ReconcilerID],
	ip *network_v1.IPAddressResource,
) rec_v2.Directive[ReconcilerID] {
	zlog.Debug().Msgf("Reconciling IPAddress %s, with Current state: %v, Desired state: %v.",
		ip.GetAddress(), ip.GetCurrentState(), ip.GetDesiredState())
	tenantID := request.ID.GetTenantID()
	// ip allocation is not supported at the moment
	var err error
	if ip.GetAddress() == "" {
		err = ipr.handleIPAllocation(ctx, tenantID, ip)
		zlog.Debug().Msgf("IP %v allocation completed", ip)
	} else {
		// TODO: ITEP-622
		err = ipr.handleIPDuplication(ctx, tenantID, ip)
		zlog.Debug().Msgf("IP %v duplication verified", ip)
	}
	directive := HandleInventoryError(err, request)
	if directive != nil {
		return directive
	}
	return request.Ack()
}

func (ipr *IPReconciler) handleIPAllocation(ctx context.Context, tenantID string, ip *network_v1.IPAddressResource) error {
	newIP := &network_v1.IPAddressResource{
		Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_ASSIGNMENT_ERROR,
		StatusDetail: "IPAddress assignment is unsupported",
		CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR,
	}
	return ipr.netClient.UpdateIPAddressStatusAndState(ctx, tenantID, ip.GetResourceId(), newIP)
}

func (ipr *IPReconciler) handleIPDuplication(ctx context.Context, tenantID string, ip *network_v1.IPAddressResource) error {
	ips, err := ipr.netClient.GetIPAddressesInSite(ctx, tenantID, ip)
	// Any error we get here we are fine
	if err != nil {
		return err
	}
	if len(ips) == 0 {
		// No IPs found. Weird but we can continue
		zlog.InfraSec().Warn().Msgf("No IPs found while checking duplication")
		return nil
	}
	resID := ip.GetResourceId()
	ip = &network_v1.IPAddressResource{
		Status:       ip.Status,
		StatusDetail: ip.StatusDetail,
		CurrentState: ip.CurrentState,
	}
	var newIP *network_v1.IPAddressResource
	if len(ips) > 1 {
		zlog.Debug().Msgf("Found more than one IPAddress %s", ip.GetAddress())
		// More than one, means duplicate. Report error only if necessary.
		newIP = &network_v1.IPAddressResource{
			Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURATION_ERROR,
			StatusDetail: "IPAddress duplication is unsupported",
			CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_ERROR,
		}
	} else {
		zlog.Debug().Msgf("No duplicates found for IPAddress %s", ip.GetAddress())
		// Exactly one, we are ok. Update state only if necessary.
		newIP = &network_v1.IPAddressResource{
			Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
			StatusDetail: "IPAddress is configured",
			CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
		}
	}
	if !proto.Equal(ip, newIP) {
		return ipr.netClient.UpdateIPAddressStatusAndState(ctx, tenantID, resID, newIP)
	}
	return nil
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package reconcilers

import (
	"context"
	"strings"
	"time"

	"github.com/open-edge-platform/cluster-api-provider-intel/pkg/tracing"
	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-managers/remote-access/pkg/clients"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

// Misc variables.
var (
	loggerName = "RAReconciler"
	zlog       = logging.GetLogger(loggerName)
)

type RAReconciler struct {
	netClient      *clients.RmtAccessInventoryClient
	tracingEnabled bool
}

func NewRAReconciler(cl *clients.RmtAccessInventoryClient, tracingEnabled bool) (*RAReconciler, error) {
	return &RAReconciler{netClient: cl, tracingEnabled: tracingEnabled}, nil
}

type SpecStatus struct {
	Readiness SpecReadiness
	Reason    string
}

// Reconcile implements the main reconcile logic.
func (rar *RAReconciler) Reconcile(ctx context.Context, req rec_v2.Request[ReconcilerID]) rec_v2.Directive[ReconcilerID] {
	if rar.tracingEnabled {
		ctx = tracing.StartTrace(ctx, "RemoteAccessManager", "RAReconciler")
		defer tracing.StopTrace(ctx)
	}

	tenantID := req.ID.GetTenantID()
	resourceID := req.ID.GetResourceID()

	ra, d := rar.fetchRA(ctx, tenantID, resourceID, req)
	if d != nil {
		return d
	}

	now := time.Now().UTC()
	spec := evaluateSpecEval(ra, now)

	if rar.shouldSkip(ra, spec) {
		// refresh cache snapshot even on skip
		return req.Ack()
	}

	return rar.reconcileWithSpec(ctx, req, ra, spec, now)
}

func (rar *RAReconciler) fetchRA(
	ctx context.Context,
	tenantID, resourceID string,
	req rec_v2.Request[ReconcilerID],
) (*remoteaccessv1.RemoteAccessConfiguration, rec_v2.Directive[ReconcilerID]) {

	ra, err := rar.netClient.GetRemoteAccessConf(ctx, tenantID, resourceID)
	if d := HandleInventoryError(err, req); d != nil {
		return nil, d
	}

	if ra == nil {
		zlog.Warn().Msgf("RemoteAccessConfiguration %s not found, nothing to reconcile", resourceID)
		return nil, req.Ack()
	}

	return ra, nil
}

func evaluateSpecEval(ra *remoteaccessv1.RemoteAccessConfiguration, now time.Time) SpecStatus {
	r, reason := evaluateSpec(ra, now)
	return SpecStatus{Readiness: r, Reason: reason}
}

// evaluateSpec classifies RemoteAccessConfiguration as READY, PENDING or INVALID
// and returns a human-readable reason used for status text.
func evaluateSpec(ra *remoteaccessv1.RemoteAccessConfiguration, now time.Time) (SpecReadiness, string) {
	if ra == nil {
		return SpecReadinessInvalid, "configuration is nil"
	}

	var fatalIssues []string
	var pendingIssues []string

	checkIdentity(ra, &fatalIssues)
	checkExpiration(ra, now, &fatalIssues, &pendingIssues)
	checkRAPBinding(ra, &pendingIssues)
	checkAgentTarget(ra, &pendingIssues)
	checkAuth(ra, &pendingIssues)
	checkDesiredState(ra, &fatalIssues)

	switch {
	case len(fatalIssues) > 0:
		return SpecReadinessInvalid, strings.Join(fatalIssues, "; ")
	case len(pendingIssues) > 0:
		return SpecReadinessPending, strings.Join(pendingIssues, "; ")
	default:
		return SpecReadinessReady, ""
	}
}

// Reconciliation helper to verify if reconciliation is needed.
func (rar *RAReconciler) shouldSkip(ra *remoteaccessv1.RemoteAccessConfiguration, spec SpecStatus) bool {
	return spec.Readiness == SpecReadinessReady && ra.GetDesiredState() == ra.GetCurrentState()
}

func (rar *RAReconciler) reconcileWithSpec(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	ra *remoteaccessv1.RemoteAccessConfiguration,
	spec SpecStatus,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {

	tenantID := req.ID.GetTenantID()
	resourceID := ra.GetResourceId()

	zlog.Debug().Msgf(
		"Reconciling RA %s, current=%v desired=%v readiness=%v reason=%q",
		resourceID, ra.GetCurrentState(), ra.GetDesiredState(), spec.Readiness, spec.Reason,
	)

	//cacheStatus := desiredToCacheStatus(ra, spec.Readiness)
	//
	//st := rmtaccessconfmgr.State{
	//	ObservedAt: now,
	//	Status:     cacheStatus,
	//}
	//
	//if cacheStatus == pb.ConfigStatus_CONFIG_STATUS_ACTIVE ||
	//	cacheStatus == pb.ConfigStatus_CONFIG_STATUS_PENDING ||
	//	cacheStatus == pb.ConfigStatus_CONFIG_STATUS_DISABLED {
	//	st.Spec = buildAgentSpec(ra)
	//}
	//
	//if cacheStatus == pb.ConfigStatus_CONFIG_STATUS_ERROR {
	//	st.Error = &pb.ConfigError{
	//		Code: "SPEC_INVALID",
	//	}
	//}

	switch spec.Readiness {
	case SpecReadinessInvalid:
		return rar.markError(ctx, req, tenantID, resourceID, spec.Reason, now)

	case SpecReadinessPending:
		return req.Ack()

	case SpecReadinessReady:
		return rar.convergeState(ctx, req, tenantID, resourceID, ra, now)

	default:
		return req.Ack()
	}
}

func (rar *RAReconciler) markError(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	tenantID, resourceID string,
	reason string,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {
	patch := &remoteaccessv1.RemoteAccessConfiguration{
		ResourceId:                   resourceID,
		CurrentState:                 remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR,
		ConfigurationStatus:          reason,
		ConfigurationStatusTimestamp: uint64(now.Unix()),
	}
	err := rar.netClient.UpdateRemoteAccessConfigState(ctx, tenantID, resourceID, patch)
	if d := HandleInventoryError(err, req); d != nil {
		return d
	}
	return req.Ack()
}

func (rar *RAReconciler) convergeState(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	tenantID, resourceID string,
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {
	targetState := ra.GetDesiredState()
	if targetState == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_UNSPECIFIED {
		targetState = remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR
	}

	if ra.GetCurrentState() == targetState {
		return req.Ack()
	}

	patch := &remoteaccessv1.RemoteAccessConfiguration{
		ResourceId:                   resourceID,
		CurrentState:                 targetState,
		ConfigurationStatus:          "remote access configuration applied",
		ConfigurationStatusTimestamp: uint64(now.Unix()),
	}
	err := rar.netClient.UpdateRemoteAccessConfigState(ctx, tenantID, resourceID, patch)
	if d := HandleInventoryError(err, req); d != nil {
		return d
	}
	return req.Ack()
}

type SpecReadiness int

const (
	SpecReadinessReady SpecReadiness = iota
	SpecReadinessPending
	SpecReadinessInvalid
)

func checkIdentity(ra *remoteaccessv1.RemoteAccessConfiguration, fatal *[]string) {
	if strings.TrimSpace(ra.GetResourceId()) == "" {
		*fatal = append(*fatal, "missing resource_id")
	}
	if ra.GetInstance() == nil {
		*fatal = append(*fatal, "missing instance reference")
	}
	if strings.TrimSpace(ra.GetTenantId()) == "" {
		*fatal = append(*fatal, "missing tenant_id")
	}
}

func checkExpiration(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
	fatal, pending *[]string,
) {
	ts := ra.GetExpirationTimestamp()
	switch {
	case ts == 0:
		*pending = append(*pending, "expiration_timestamp not set yet")
	case int64(ts) <= now.Unix():
		*fatal = append(*fatal, "expiration_timestamp is in the past")
	}
}

func checkRAPBinding(ra *remoteaccessv1.RemoteAccessConfiguration, pending *[]string) {
	if ra.GetLocalPort() == 0 {
		*pending = append(*pending, "local_port not allocated yet by RAP")
	}
	if strings.TrimSpace(ra.GetProxyHost()) == "" {
		*pending = append(*pending, "proxy_host not set yet")
	}
}

func checkAgentTarget(ra *remoteaccessv1.RemoteAccessConfiguration, pending *[]string) {
	if strings.TrimSpace(ra.GetTargetHost()) == "" {
		*pending = append(*pending, "target_host not set yet")
	}
	if ra.GetTargetPort() == 0 {
		*pending = append(*pending, "target_port not set yet")
	}
}

func checkAuth(ra *remoteaccessv1.RemoteAccessConfiguration, pending *[]string) {
	if strings.TrimSpace(ra.GetUser()) == "" {
		*pending = append(*pending, "user (SSH user) not set yet")
	}
	if strings.TrimSpace(ra.GetSessionToken()) == "" {
		*pending = append(*pending, "session_token not set yet")
	}
}

func checkDesiredState(ra *remoteaccessv1.RemoteAccessConfiguration, fatal *[]string) {
	if ra.GetDesiredState() == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_UNSPECIFIED {
		*fatal = append(*fatal, "desired_state is UNSPECIFIED")
	}
}

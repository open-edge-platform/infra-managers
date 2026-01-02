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
	"github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/clients"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

var (
	rapLoggerName = "RAPReconciler"
	zlog          = logging.GetLogger(rapLoggerName)
)

// RAPRuntime represents in-memory / runtime state of the Remote Access Proxy.
// It MUST NOT perform inventory reconciliation logic.
// It MUST NOT decide desired/current state transitions.
// Its only responsibility is to ensure or tear down runtime sessions.
type RAPRuntime interface {

	// EnsureSession ensures that a runtime session exists for the given spec.
	// It may create, update or refresh an existing session.
	// It MUST be idempotent.
	EnsureSession(
		ctx context.Context,
		tenantID string,
		resourceID string,
		spec *RAPSpec,
	) error

	// DisableSession removes any runtime artifacts associated with the resource.
	// It MUST be safe to call even if no session exists.
	DisableSession(
		ctx context.Context,
		tenantID string,
		resourceID string,
		reason string,
	) error
}

type RAPReconciler struct {
	netClient      *clients.RmtAccessInventoryClient
	runtime        RAPRuntime
	tracingEnabled bool
}

func NewRAPReconciler(
	cl *clients.RmtAccessInventoryClient,
	runtime RAPRuntime,
	tracingEnabled bool,
) (*RAPReconciler, error) {
	return &RAPReconciler{
		netClient:      cl,
		runtime:        runtime,
		tracingEnabled: tracingEnabled,
	}, nil
}

// RAPSpec is a pure runtime view of RemoteAccessConfiguration.
// It contains only fields required by the proxy runtime.
type RAPSpec struct {
	ResourceID string
	TenantID   string

	ProxyHost string
	LocalPort uint32

	TargetHost string
	TargetPort uint32

	User         string
	SessionToken string

	DesiredState remoteaccessv1.RemoteAccessState
	ExpirationTs uint64 // unix seconds
}

func (r *RAPReconciler) Reconcile(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
) rec_v2.Directive[ReconcilerID] {

	if r.tracingEnabled {
		ctx = tracing.StartTrace(ctx, "RemoteAccessProxy", "RAPReconciler")
		defer tracing.StopTrace(ctx)
	}

	tenantID := req.ID.GetTenantID()
	resourceID := req.ID.GetResourceID()

	ra, d := r.fetchRemoteAccess(ctx, tenantID, resourceID, req)
	if d != nil {
		return d
	}

	now := time.Now().UTC()
	specStatus := evaluateSpec(ra, now)

	if r.shouldSkip(ra, specStatus) {
		return req.Ack()
	}

	return r.reconcileWithSpec(ctx, req, tenantID, resourceID, ra, specStatus, now)
}

func (r *RAPReconciler) fetchRemoteAccess(
	ctx context.Context,
	tenantID string,
	resourceID string,
	req rec_v2.Request[ReconcilerID],
) (*remoteaccessv1.RemoteAccessConfiguration, rec_v2.Directive[ReconcilerID]) {

	ra, err := r.netClient.GetRemoteAccessConf(ctx, tenantID, resourceID)
	if d := HandleInventoryError(err, req); d != nil {
		return nil, d
	}

	// Inventory object disappeared -> ensure runtime cleanup
	if ra == nil {
		zlog.Warn().Msgf(
			"RemoteAccessConfiguration %s not found, cleaning up runtime",
			resourceID,
		)
		_ = r.runtime.DisableSession(ctx, tenantID, resourceID, "inventory record missing")
		return nil, req.Ack()
	}

	return ra, nil
}

// SpecReadiness describes whether the configuration can be applied by RAP.
type SpecReadiness int

const (
	SpecReady SpecReadiness = iota
	SpecPending
	SpecInvalid
)

type SpecStatus struct {
	Readiness SpecReadiness
	Reason    string
}

// evaluateSpec classifies the configuration without performing side effects.
func evaluateSpec(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
) SpecStatus {

	if ra == nil {
		return SpecStatus{SpecInvalid, "configuration is nil"}
	}

	var fatal []string
	var pending []string

	checkIdentity(ra, &fatal)
	checkDesiredState(ra, &fatal)
	checkExpirationForRAP(ra, now, &fatal, &pending)
	checkRAPBinding(ra, &pending)
	checkAgentTarget(ra, &pending)

	switch {
	case len(fatal) > 0:
		return SpecStatus{SpecInvalid, strings.Join(fatal, "; ")}
	case len(pending) > 0:
		return SpecStatus{SpecPending, strings.Join(pending, "; ")}
	default:
		return SpecStatus{SpecReady, ""}
	}
}

// Skip reconciliation only when:
// - spec is READY
// - desired state already equals current state
func (r *RAPReconciler) shouldSkip(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	spec SpecStatus,
) bool {
	return spec.Readiness == SpecReady &&
		ra.GetDesiredState() == ra.GetCurrentState()
}

func (r *RAPReconciler) reconcileWithSpec(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	tenantID string,
	resourceID string,
	ra *remoteaccessv1.RemoteAccessConfiguration,
	spec SpecStatus,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {

	zlog.Debug().Msgf(
		"Reconciling RAP for %s: current=%v desired=%v readiness=%v",
		resourceID,
		ra.GetCurrentState(),
		ra.GetDesiredState(),
		spec.Readiness,
	)

	switch spec.Readiness {

	case SpecInvalid:
		_ = r.runtime.DisableSession(ctx, tenantID, resourceID, "spec invalid: "+spec.Reason)
		return r.markError(ctx, req, tenantID, resourceID, spec.Reason, now)

	case SpecPending:
		if ra.GetDesiredState() == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_DISABLED {
			_ = r.runtime.DisableSession(ctx, tenantID, resourceID, "desired disabled (pending)")
		}
		return req.Ack()

	case SpecReady:
		if ra.GetDesiredState() == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_DISABLED {
			_ = r.runtime.DisableSession(ctx, tenantID, resourceID, "desired disabled")
			return r.convergeState(ctx, req, tenantID, resourceID, ra, now)
		}

		spec := buildRAPSpec(ra)
		if err := r.runtime.EnsureSession(ctx, tenantID, resourceID, spec); err != nil {
			return r.markError(
				ctx,
				req,
				tenantID,
				resourceID,
				"runtime ensure failed: "+err.Error(),
				now,
			)
		}

		return r.convergeState(ctx, req, tenantID, resourceID, ra, now)

	default:
		return req.Ack()
	}
}

func (r *RAPReconciler) markError(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	tenantID string,
	resourceID string,
	reason string,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {

	patch := &remoteaccessv1.RemoteAccessConfiguration{
		ResourceId:                   resourceID,
		CurrentState:                 remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR,
		ConfigurationStatus:          reason,
		ConfigurationStatusTimestamp: uint64(now.Unix()),
	}

	err := r.netClient.UpdateRemoteAccessConfigState(ctx, tenantID, resourceID, patch)
	if d := HandleInventoryError(err, req); d != nil {
		return d
	}
	return req.Ack()
}

func (r *RAPReconciler) convergeState(
	ctx context.Context,
	req rec_v2.Request[ReconcilerID],
	tenantID string,
	resourceID string,
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
) rec_v2.Directive[ReconcilerID] {

	target := ra.GetDesiredState()
	if target == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_UNSPECIFIED {
		target = remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR
	}

	if ra.GetCurrentState() == target {
		return req.Ack()
	}

	patch := &remoteaccessv1.RemoteAccessConfiguration{
		ResourceId:                   resourceID,
		CurrentState:                 target,
		ConfigurationStatus:          "remote access proxy reconciled",
		ConfigurationStatusTimestamp: uint64(now.Unix()),
	}

	err := r.netClient.UpdateRemoteAccessConfigState(ctx, tenantID, resourceID, patch)
	if d := HandleInventoryError(err, req); d != nil {
		return d
	}
	return req.Ack()
}

func checkIdentity(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	fatal *[]string,
) {
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

func checkDesiredState(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	fatal *[]string,
) {
	if ra.GetDesiredState() ==
		remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_UNSPECIFIED {
		*fatal = append(*fatal, "desired_state is UNSPECIFIED")
	}
}

// Expiration is fatal only when enabling.
func checkExpirationForRAP(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
	fatal *[]string,
	pending *[]string,
) {
	if ra.GetDesiredState() ==
		remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_DISABLED {
		return
	}

	ts := ra.GetExpirationTimestamp()
	switch {
	case ts == 0:
		*pending = append(*pending, "expiration_timestamp not set")
	case int64(ts) <= now.Unix():
		*fatal = append(*fatal, "expiration_timestamp is in the past")
	}
}

// Fields typically allocated by manager/proxy.
func checkRAPBinding(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	pending *[]string,
) {
	if ra.GetLocalPort() == 0 {
		*pending = append(*pending, "local_port not allocated")
	}
	if strings.TrimSpace(ra.GetProxyHost()) == "" {
		*pending = append(*pending, "proxy_host not set")
	}
}

// Without target, agent cannot expose SSH endpoint.
func checkAgentTarget(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	pending *[]string,
) {
	if strings.TrimSpace(ra.GetTargetHost()) == "" {
		*pending = append(*pending, "target_host not set")
	}
	if ra.GetTargetPort() == 0 {
		*pending = append(*pending, "target_port not set")
	}
}

func buildRAPSpec(
	ra *remoteaccessv1.RemoteAccessConfiguration,
) *RAPSpec {
	return &RAPSpec{
		ResourceID:   ra.GetResourceId(),
		TenantID:     ra.GetTenantId(),
		ProxyHost:    ra.GetProxyHost(),
		LocalPort:    ra.GetLocalPort(),
		TargetHost:   ra.GetTargetHost(),
		TargetPort:   ra.GetTargetPort(),
		User:         ra.GetUser(),
		SessionToken: ra.GetSessionToken(),
		DesiredState: ra.GetDesiredState(),
		ExpirationTs: ra.GetExpirationTimestamp(),
	}
}

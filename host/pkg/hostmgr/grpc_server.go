// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package hostmgr implements the Host Manager gRPC service.
package hostmgr

import (
	"context"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	"github.com/open-edge-platform/infra-managers/host/pkg/errors"
	inv_mgr_cli "github.com/open-edge-platform/infra-managers/host/pkg/invclient"
	hmgr_util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

type server struct {
	pb.UnimplementedHostmgrServer
	rbac        *rbac.Policy
	authEnabled bool
}

//nolint:stylecheck,revive // name of this function should be aligned with the one in .pb.go
func (s *server) UpdateHostStatusByHostGuid(ctx context.Context,
	in *pb.UpdateHostStatusByHostGuidRequest,
) (*pb.HostStatusResp, error) {
	zlog.Info().Msgf("Updating Host (%s) status", in.GetHostGuid())
	if s.authEnabled {
		if !s.rbac.IsRequestAuthorized(ctx, rbac.UpdateKey) {
			err := inv_errors.Errorfc(codes.PermissionDenied, "Request is blocked by RBAC")
			zlog.InfraSec().InfraErr(err).Msgf("Request UpdateHostStatusByHostGuid is not authenticated")
			return nil, err
		}
	}
	if err := in.ValidateAll(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Invalid request: %v", in)
		return nil, errors.Wrap(err)
	}

	guid := in.GetHostGuid()
	status := in.GetHostStatus()

	tenantID, present := tenant.GetTenantIDFromContext(ctx)
	if !present {
		// This should never happen! Interceptor should either fail or set it!
		err := inv_errors.Errorfc(codes.Unauthenticated, "Tenant ID is not present in context")
		zlog.InfraSec().InfraErr(err).Msg("Request UpdateHostStatusByHostGuid is not authenticated")
		return nil, err
	}

	hostResc, err := inv_mgr_cli.GetHostResourceByGUID(ctx, invClientInstance, tenantID, guid)
	if err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if hmgr_util.IsHostUntrusted(hostResc) {
		zlog.InfraSec().InfraError("Host tID=%s, UUID=%s is not trusted, the message will not be handled", tenantID, guid).
			Msg("UpdateHostStatusByHostGuid")
		return nil, inv_errors.Errorfc(codes.Unauthenticated,
			"Host tID=%s, UUID=%s is not trusted, the message will not be handled", tenantID, guid)
	}

	err = s.updateHostStatusIfNeeded(ctx, tenantID, hostResc, status)
	if err != nil {
		return nil, err
	}

	return &pb.HostStatusResp{}, nil
}

//nolint:cyclop // cyclomatic complexity is high due to update of various Host components
func (s *server) UpdateHostSystemInfoByGUID(ctx context.Context,
	in *pb.UpdateHostSystemInfoByGUIDRequest,
) (*pb.UpdateHostSystemInfoByGUIDResponse, error) {
	zlog.Info().Msgf("Updating Host (%s) system information", in.GetHostGuid())
	if s.authEnabled {
		if !s.rbac.IsRequestAuthorized(ctx, rbac.UpdateKey) {
			err := inv_errors.Errorfc(codes.PermissionDenied, "Request is blocked by RBAC")
			zlog.InfraSec().InfraErr(err).Msgf("Request UpdateHostSystemInfoByGUID is not authenticated")
			return nil, err
		}
	}
	if err := in.ValidateAll(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Invalid request %v", in)
		return nil, errors.Wrap(err)
	}

	guid := in.GetHostGuid()
	systemInfo := in.GetSystemInfo()

	tenantID, present := tenant.GetTenantIDFromContext(ctx)
	if !present {
		// This should never happen! Interceptor should either fail or set it!
		err := inv_errors.Errorfc(codes.Unauthenticated, "Tenant ID is not present in context")
		zlog.InfraSec().InfraErr(err).Msg("Request UpdateHostSystemInfoByGUID is not authenticated")
		return nil, err
	}

	hostres, err := inv_mgr_cli.GetHostResourceByGUID(ctx, invClientInstance, tenantID, guid)
	if err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if hmgr_util.IsHostUntrusted(hostres) {
		zlog.InfraSec().InfraError("Host tID=%s, UUID=%s is not trusted, the message will not be handled", tenantID, guid).
			Msg("UpdateHostSystemInfoByGUID")
		return nil, inv_errors.Errorfc(codes.Unauthenticated,
			"Host tID=%s, UUID=%s is not trusted, the message will not be handled", tenantID, guid)
	}

	if hmgr_util.IsHostNotProvisioned(hostres) {
		zlog.InfraSec().
			InfraError("Host tID=%s, UUID=%s is not yet provisioned, skipping update", tenantID, hostres.GetUuid()).
			Msg("UpdateHostSystemInfoByGUID")
		return nil, inv_errors.Errorfc(codes.FailedPrecondition, "")
	}

	err = updateHost(ctx, tenantID, hostres, systemInfo)
	if err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if err := updateHoststorage(ctx, tenantID, hostres, systemInfo.HwInfo); err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if err := updateHostnics(ctx, tenantID, hostres, systemInfo.HwInfo); err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if err := updateHostusbs(ctx, tenantID, hostres, systemInfo.HwInfo); err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if err := updateHostgpus(ctx, tenantID, hostres, systemInfo.HwInfo); err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	return &pb.UpdateHostSystemInfoByGUIDResponse{}, nil
}

func (s *server) UpdateInstanceStateStatusByHostGUID(
	ctx context.Context,
	in *pb.UpdateInstanceStateStatusByHostGUIDRequest,
) (*pb.UpdateInstanceStateStatusByHostGUIDResponse, error) {
	if s.authEnabled {
		if !s.rbac.IsRequestAuthorized(ctx, rbac.UpdateKey) {
			err := inv_errors.Errorfc(codes.PermissionDenied, "Request is blocked by RBAC")
			zlog.InfraSec().InfraErr(err).Msgf("Request UpdateInstanceStateStatusByHostGUID is not authenticated")
			return nil, err
		}
	}
	if err := in.ValidateAll(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Invalid request %v", in)
		return nil, errors.Wrap(err)
	}

	tenantID, present := tenant.GetTenantIDFromContext(ctx)
	if !present {
		// This should never happen! Interceptor should either fail or set it!
		err := inv_errors.Errorfc(codes.Unauthenticated, "Tenant ID is not present in context")
		zlog.InfraSec().InfraErr(err).Msg("Request UpdateInstanceStateStatusByHostGUID is not authenticated")
		return nil, err
	}

	zlog.Info().Msgf("Updating an Instance for Host (tID=%s, UUID=%s)", tenantID, in.GetHostGuid())
	// Finding a Host by GUID to get its ResourceID first (needed for querying Instance)
	host, err := inv_mgr_cli.GetHostResourceByGUID(ctx, invClientInstance, tenantID, in.GetHostGuid())
	if err != nil {
		return &pb.UpdateInstanceStateStatusByHostGUIDResponse{}, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	if hmgr_util.IsHostUntrusted(host) {
		zlog.InfraSec().
			InfraError("Skip updating instance state for host tID=%s, UUID=%s (untrusted).", tenantID, host.GetUuid()).
			Msg("UpdateInstanceStateStatusByHostGUID")
		return &pb.UpdateInstanceStateStatusByHostGUIDResponse{}, inv_errors.Errorfc(codes.Unauthenticated, "")
	}

	// Deal first with Host status update
	err = s.updateHostStatusIfNeeded(ctx, tenantID, host, hmgr_util.InstanceStatusToHostStatusMsg(in))
	if err != nil {
		return nil, err
	}

	err = updateInstanceStateStatusByHostGUID(ctx, tenantID, host.GetInstance(), in)
	if err != nil {
		return nil, inv_errors.ErrorToSanitizedGrpcError(err)
	}

	return &pb.UpdateInstanceStateStatusByHostGUIDResponse{}, nil
}

func (s *server) updateHostStatusIfNeeded(
	ctx context.Context, tenantID string, host *computev1.HostResource, status *pb.HostStatus,
) error {
	hostUUID := host.GetUuid()
	// update host heartbeat
	if err := alivemgr.UpdateHostHeartBeat(host); err != nil {
		zlog.Warn().Err(err).Msg("Failed to update host heartbeat")
	}

	// If host under maintenance, skip everything else
	if hmgr_util.IsHostUnderMaintain(host) {
		zlog.Info().Msgf("Skip host status update for host tID=%s, UUID=%s is under maintain.", tenantID, hostUUID)
		return nil
	}

	if hmgr_util.IsHostNotProvisioned(host) {
		zlog.InfraSec().
			InfraError("Skip updating instance state for host tID=%s, UUID=%s (not provisioned)", tenantID, host.GetUuid()).
			Msg("UpdateInstanceStateStatusByHostGUID")
		return inv_errors.Errorfc(codes.FailedPrecondition, "")
	}

	if hmgr_util.IsSameHostStatus(host, status) {
		zlog.Info().Msgf("Skip host tID=%s, UUID=%s status update for no changes.", tenantID, hostUUID)
		return nil
	}

	hostStatusName := pb.HostStatus_HostStatus_name[int32(status.GetHostStatus())]
	zlog.Debug().Msgf("Update host resc (tID=%s, resID=%v) status: %v", tenantID, host.GetResourceId(),
		hostStatusName)

	if err := inv_mgr_cli.SetHostStatus(ctx, invClientInstance, tenantID, host.GetResourceId(),
		hmgr_util.GetHostStatus(status.GetHostStatus()),
	); err != nil {
		zlog.InfraSec().InfraError("Failed to update host resource info").Msg("updateHostStatusIfNeeded")
		return inv_errors.ErrorToSanitizedGrpcError(err)
	}
	return nil
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package maintmgr

import (
	"context"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	mmgr_error "github.com/open-edge-platform/infra-managers/maintenance/pkg/errors"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	maintgmr_util "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

type server struct {
	pb.UnimplementedMaintmgrServiceServer
	rbac        *rbac.Policy
	authEnabled bool
}

//nolint:cyclop,funlen // cyclomatic complexity is 11 due to validation logic
func (s *server) PlatformUpdateStatus(ctx context.Context,
	in *pb.PlatformUpdateStatusRequest,
) (*pb.PlatformUpdateStatusResponse, error) {
	// TODO: refactor to reduce cyclomatic complexity
	zlog.Info().Msgf("PlatformUpdateStatus: GUID=%s", in.GetHostGuid())
	zlog.Debug().Msgf("PlatformUpdateStatus: request=%v", in)
	if s.authEnabled {
		if !s.rbac.IsRequestAuthorized(ctx, rbac.GetKey) {
			err := inv_errors.Errorfc(codes.PermissionDenied, "Request is blocked by RBAC")
			zlog.InfraSec().InfraErr(err).Msgf("Request PlatformUpdateStatus is not authenticated")
			return nil, err
		}
	}
	err := in.ValidateAll()
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, mmgr_error.Wrap(err)
	}
	guid := in.GetHostGuid()

	tenantID, present := tenant.GetTenantIDFromContext(ctx)
	if !present {
		// This should never happen! Interceptor should either fail or set it!
		err = inv_errors.Errorfc(codes.Unauthenticated, "Tenant ID is not present in context")
		zlog.InfraSec().InfraErr(err).Msg("Request PlatformUpdateStatus is not authenticated")
		return nil, err
	}
	zlog.Debug().Msgf("PlatformUpdateStatus: tenantID=%s", tenantID)

	hostRes, instRes, err := getHostAndInstanceFromUUID(ctx, tenantID, guid)
	if err != nil {
		return nil, err
	}

	if maintgmr_util.IsHostUntrusted(hostRes) {
		zlog.InfraSec().InfraError("Host [tID=%s, UUID=%s] is not trusted, the message will not be handled", tenantID, guid).
			Msg("PlatformUpdateStatus")
		return nil, inv_errors.Errorfc(codes.Unauthenticated,
			"Host [tID=%s, UUID=%s] is not trusted, the message will not be handled", tenantID, guid)
	}

	if maintgmr_util.IsInstanceNotProvisioned(instRes) {
		zlog.InfraSec().
			InfraError("Host tID=%s, UUID=%s is not yet provisioned, skipping update", tenantID, hostRes.GetUuid()).
			Msg("PlatformUpdateStatus")
		return nil, inv_errors.Errorfc(codes.FailedPrecondition, "")
	}

	updateInstanceInInv(ctx, invMgrCli.InvClient, tenantID, in.GetUpdateStatus(), instRes)

	handleOSUpdateRun(ctx, invMgrCli.InvClient, tenantID, in.GetUpdateStatus(), instRes)

	ssRes, err := invclient.ListSingleSchedules(ctx, invMgrCli, tenantID, hostRes)
	if err != nil {
		return nil, err
	}
	closestSingleSched := maintgmr_util.GetClosestSingleSchedule(ssRes)

	rsRes, err := invclient.ListRepeatedSchedules(ctx, invMgrCli, tenantID, hostRes)
	if err != nil {
		return nil, err
	}

	scheresp, err := maintgmr_util.PopulateUpdateSchedule(rsRes, closestSingleSched)
	if err != nil {
		return nil, err
	}

	osType := instRes.GetOs().GetOsType()

	response := &pb.PlatformUpdateStatusResponse{
		UpdateSchedule: scheresp,
		OsType:         pb.PlatformUpdateStatusResponse_OSType(osType),
		UpdateSource: &pb.UpdateSource{
			KernelCommand: "",
			CustomRepos:   []string{},
		},
		InstalledPackages:     "",
		OsProfileUpdateSource: &pb.OSProfileUpdateSource{},
	}
	// Return empty OSUpdatePolicy Resource if not found
	osUpdatePolicyRes, err := invclient.GetOSUpdatePolicyByInstanceID(ctx, invMgrCli.InvClient, tenantID, instRes.GetResourceId())
	// Not found is not an error, it means that the instance does not have an OS update policy.
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("PlatformUpdateStatus: tenantID=%s, UUID=%s", tenantID, guid)
		return nil, err
	}
	zlog.Debug().Msgf("OS Update Policy resource from Instance backlink: tenantID=%s, instanceID=%s, updatePolicy=%v",
		tenantID, instRes.GetResourceId(), osUpdatePolicyRes)

	if osUpdatePolicyRes != nil && osUpdatePolicyRes.GetResourceId() != "" {
		err = getOSUpdatePolicyInfo(ctx, response, osType, osUpdatePolicyRes, tenantID,
			instRes.GetOs().GetProfileName(), guid)
		if err != nil {
			return nil, err
		}
	}

	zlog.Debug().Msgf("PlatformUpdateStatus: tenantID%s, response=%v", tenantID, response)
	if err = response.ValidateAll(); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, mmgr_error.Wrap(err)
	}
	return response, nil
}

func getHostAndInstanceFromUUID(ctx context.Context, tenantID, uuid string) (
	*computev1.HostResource, *computev1.InstanceResource, error,
) {
	instRes, err := invclient.GetInstanceResourceByHostGUID(ctx, invMgrCli.InvClient, tenantID, uuid)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to get instance: tenantID=%s, UUID=%s", tenantID, uuid)
		return nil, nil, err
	}

	hostRes := instRes.Host
	if hostRes == nil {
		// this should never happen
		return nil, nil, inv_errors.Errorfc(codes.NotFound, "Instance without host: tenantID=%s, UUID=%s", tenantID, uuid)
	}
	return hostRes, instRes, nil
}

func getOSUpdatePolicyInfo(ctx context.Context, resp *pb.PlatformUpdateStatusResponse, osType osv1.OsType,
	osUpdatePolicyRes *computev1.OSUpdatePolicyResource, tenantID string, profileName string, guid string,
) error {
	switch osType {
	case osv1.OsType_OS_TYPE_MUTABLE:
		// For mutable OS, retrieve info from the Policy and provide them to the EN.
		populateMutableUpdateDetails(resp, osUpdatePolicyRes)
	case osv1.OsType_OS_TYPE_IMMUTABLE:
		// For immutable OS, retrieve the target OS from the OSUpdatePolicy
		err := populateImmutableUpdateDetails(ctx, resp, osUpdatePolicyRes, tenantID, profileName, guid)
		if err != nil {
			return err
		}
	default:
		// We should never reach this point.
		err := inv_errors.Errorfc(codes.InvalidArgument, "Unsupported OS type: %s", osType)
		zlog.InfraSec().InfraErr(err).Msgf("PlatformUpdateStatus: tenantID=%s, UUID=%s", tenantID, guid)
		return err
	}

	return nil
}

func populateMutableUpdateDetails(resp *pb.PlatformUpdateStatusResponse, policy *computev1.OSUpdatePolicyResource) {
	resp.UpdateSource.KernelCommand = policy.GetUpdateKernelCommand()
	resp.UpdateSource.CustomRepos = policy.GetUpdateSources()
	resp.InstalledPackages = policy.GetUpdatePackages()
}

func populateImmutableUpdateDetails(
	ctx context.Context,
	resp *pb.PlatformUpdateStatusResponse,
	policy *computev1.OSUpdatePolicyResource,
	tenantID, profileName, guid string,
) error {
	osRes, err := getUpdateOS(ctx, invMgrCli.InvClient, tenantID, profileName, policy)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("PlatformUpdateStatus: tenantID=%s, UUID=%s", tenantID, guid)
		return err
	}
	zlog.Debug().Msgf("OS resource from Instance backlink: tenantID=%s, OSResource=%v", tenantID, osRes)

	resp.OsProfileUpdateSource, err = maintgmr_util.PopulateOsProfileUpdateSource(osRes)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("PlatformUpdateStatus: tenantID=%s, UUID=%s", tenantID, guid)
		return err
	}
	return nil
}

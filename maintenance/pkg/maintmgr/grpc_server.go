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

//nolint:cyclop // cyclomatic complexity is 11 due to validation logic
func (s *server) PlatformUpdateStatus(ctx context.Context,
	in *pb.PlatformUpdateStatusRequest,
) (*pb.PlatformUpdateStatusResponse, error) {
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

	var osRes *osv1.OperatingSystemResource
	var osType osv1.OsType
	osUpdatePolicyRes := instRes.GetOsUpdatePolicy()

	osType = instRes.GetOs().GetOsType()
	//get OS Resource based on Update policy for immutable.
	// (Mutable has only one scenario )
	if osType == osv1.OsType_OS_TYPE_IMMUTABLE {
		osRes, err = getUpdateOS(ctx, invMgrCli.InvClient, tenantID, instRes.GetOs().GetProfileName(), osUpdatePolicyRes)
	}

	zlog.Debug().Msgf("OS resource from Instance backlink: tenantID=%s, OSResource=%v", tenantID, osRes)

	upSources := maintgmr_util.PopulateUpdateSource(osType, osUpdatePolicyRes)
	installedPackages := maintgmr_util.PopulateInstalledPackages(osType, osUpdatePolicyRes)
	osProfileUpdateSource := maintgmr_util.PopulateOsProfileUpdateSource(osRes) // gets information from  OS Profile Resource for IMMUTABLE only

	response := &pb.PlatformUpdateStatusResponse{
		UpdateSource:          upSources,
		UpdateSchedule:        scheresp,
		InstalledPackages:     installedPackages,
		OsType:                pb.PlatformUpdateStatusResponse_OSType(osType),
		OsProfileUpdateSource: osProfileUpdateSource,
	}

	zlog.Debug().Msgf("PlatformUpdateStatus: tenantID%s, response=%v", tenantID, response)
	// No need to validate, already validated by PopulateUpdateSource and PopulateUpdateSchedule
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

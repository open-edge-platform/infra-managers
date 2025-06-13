// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package maintmgr

import (
	"context"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	maintgmr_util "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

func updateInstanceInInv(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) {
	newInstUpStatus, needed := maintgmr_util.GetUpdatedUpdateStatusIfNeeded(mmUpStatus,
		instRes.GetUpdateStatusIndicator(), instRes.GetUpdateStatus())

	if needed {
		zlog.Debug().Msgf("Updating Instance UpdateStatus: old=%v, new=%v",
			newInstUpStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))

		newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
			newInstUpStatus, mmUpStatus, instRes.GetCurrentOs().GetOsType())

		newOSResID, err := GetNewOSResourceIDIfNeeded(ctx, client, tenantID, mmUpStatus, instRes)
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf("Failed to get new OS Resource")
			return
		}

		err = invclient.UpdateInstance(ctx, client, tenantID, instRes.GetResourceId(),
			*newInstUpStatus, newUpdateStatusDetail, newOSResID)
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf("Failed to update Instance Status")
			return
		}
	} else {
		zlog.Debug().Msgf("No Instance UpdateStatus update needed: old=%v, new=%v",
			newInstUpStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))
	}
}

func getUpdateOS(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, profileName string, policy *computev1.OSUpdatePolicyResource) (*os_v1.OperatingSystemResource, error) {

	var os *os_v1.OperatingSystemResource
	var err error
	switch policy.GetUpdatePolicy() {
	case computev1.UpdatePolicy_UPDATE_POLICY_TARGET:

		os = policy.GetTargetOs()
		if os == nil {
			err := errors.Errorfc(codes.Internal, "missing targetOS in OSUpdatePolicy: ResourceID %s, UpdatePolicy: %s",
				policy.GetResourceId(), policy.GetUpdatePolicy())
			zlog.InfraSec().InfraErr(err).Msg("")
		}

	case computev1.UpdatePolicy_UPDATE_POLICY_LATEST:

		os, err = invclient.GetLatestImmutableOSByProfile(ctx, c, tenantID, profileName)
		if err != nil {
			return nil, err
		}
	default:
		zlog.InfraSec().Warn().Err(err).Msgf("Unsupported scenario")
		err = errors.Errorfc(codes.Internal,
			"unsupported update scenario: ResourceID %s, UpdatePolicy: %s",
			policy.GetResourceId(), policy.GetUpdatePolicy())
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, err
	}

	return os, nil
}

func GetNewOSResourceIDIfNeeded(ctx context.Context, c inv_client.TenantAwareInventoryClient,
	tenantID string, mmUpStatus *pb.UpdateStatus, instance *computev1.InstanceResource,
) (string, error) {
	zlog.Debug().Msgf("GetNewOSResourceIDIfNeeded: Instance's current OS osType=%s, new updateStatus=%s",
		instance.GetCurrentOs().GetOsType(), mmUpStatus.StatusType)

	if mmUpStatus.StatusType != pb.UpdateStatus_STATUS_TYPE_UPDATED ||
		instance.GetCurrentOs().GetOsType() != os_v1.OsType_OS_TYPE_IMMUTABLE {
		zlog.Debug().Msgf("abandoned OS Resource search as not needed")
		return "", nil
	}

	zlog.Debug().Msgf("GetNewOSResourceIDIfNeeded: profileName=%s, profileVersion=%s, osImageId=%s",
		mmUpStatus.ProfileName, mmUpStatus.ProfileVersion, mmUpStatus.OsImageId)

	// TBD add ProfileVersion check
	if mmUpStatus.ProfileName == "" || mmUpStatus.OsImageId == "" {
		err := errors.Errorfc(codes.Internal, "missing information about immutable OS - profileName=%s, osImageId=%s",
			mmUpStatus.ProfileName, mmUpStatus.OsImageId)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}

	if instance.GetCurrentOs().GetProfileName() != mmUpStatus.GetProfileName() {
		err := errors.Errorfc(codes.Internal,
			"current profile name differs from the new profile name: current profileName=%s, new profileName=%s",
			instance.GetCurrentOs().GetProfileName(), mmUpStatus.ProfileName)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}

	if instance.GetCurrentOs().GetImageId() == mmUpStatus.GetOsImageId() {
		err := errors.Errorfc(codes.Internal,
			"current and new OS Image IDs are identical: current OS ImageID=%s, new OS ImageID=%s",
			instance.GetCurrentOs().GetImageId(), mmUpStatus.OsImageId)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}

	newOSResID, err := invclient.GetOSResourceIDByProfileInfo(ctx, c, tenantID, mmUpStatus.ProfileName, mmUpStatus.OsImageId)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("OS Resource retrieval error for profileName=%s  profileVersion=%s",
			mmUpStatus.ProfileName, mmUpStatus.OsImageId)
		return "", err
	}

	if instance.GetCurrentOs().GetResourceId() == newOSResID {
		err := errors.Errorfc(codes.Internal,
			"current and new OS Resource IDs are identical: current OS Resource ID=%s, new OS Resource ID=%s",
			instance.GetCurrentOs().GetResourceId(), newOSResID)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}
	return newOSResID, nil
}

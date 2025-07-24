// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package maintmgr

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
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

		zlog.Debug().Msgf("New OS Resource ID: %s", newOSResID)

		newExistingCVEs, err := GetNewExistingCVEs(ctx, client, tenantID, newOSResID, instRes, newInstUpStatus)
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf("Failed to get new existing CVEs")
			return
		}

		err = invclient.UpdateInstance(ctx, client, tenantID, instRes.GetResourceId(),
			*newInstUpStatus, newUpdateStatusDetail, newOSResID, newExistingCVEs)
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

func getUpdateOS(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID, profileName string,
	policy *computev1.OSUpdatePolicyResource,
) (*os_v1.OperatingSystemResource, error) {
	var os *os_v1.OperatingSystemResource
	var err error
	switch policy.GetUpdatePolicy() {
	case computev1.UpdatePolicy_UPDATE_POLICY_TARGET:
		os = policy.GetTargetOs()
		if os == nil {
			err = errors.Errorfc(codes.NotFound, "missing targetOS in OSUpdatePolicy: ResourceID %s, UpdatePolicy: %s",
				policy.GetResourceId(), policy.GetUpdatePolicy())
			zlog.InfraSec().InfraErr(err).Msg("")
		}
	case computev1.UpdatePolicy_UPDATE_POLICY_LATEST:
		os, err = invclient.GetLatestImmutableOSByProfile(ctx, c, tenantID, profileName)
		if err != nil {
			return nil, err
		}
	default:
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

func handleOSUpdateRun(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) {
	zlog.Debug().Msgf("handleOSUpdateRunResourse: status=%s", mmUpStatus.StatusType.String())

	switch mmUpStatus.StatusType {
	case pb.UpdateStatus_STATUS_TYPE_STARTED:
		createOSUpdateRun(ctx, client, tenantID, mmUpStatus, instRes)
	case pb.UpdateStatus_STATUS_TYPE_UPDATED, pb.UpdateStatus_STATUS_TYPE_FAILED:
		updateOSUpdateRun(ctx, client, tenantID, instRes, mmUpStatus)
	default:
		// Ignore other statuses.
		zlog.Debug().Msgf("No OsUpdateRun status change needed: status=%s", mmUpStatus.StatusType.String())
		return
	}
}

func getOSUpdateRunPerIns(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {
	instID := instRes.GetResourceId()
	return invclient.GetLatestOSUpdateRunByInstanceID(ctx, client, tenantID, instID)
}

func createOSUpdateRun(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, upStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) {
	newUpdateStatus := maintgmr_util.GetUpdatedUpdateStatus(upStatus)
	instanceID := instRes.GetResourceId()
	policy := instRes.GetOsUpdatePolicy()

	timeNow := time.Now().UTC().Format(maintgmr_util.ISO8601Format)
	runRes := &computev1.OSUpdateRunResource{
		Name:            maintgmr_util.GenerateRandomOsUpdateRunName(),
		Instance:        &computev1.InstanceResource{ResourceId: instRes.ResourceId},
		Status:          newUpdateStatus.Status,
		StatusIndicator: newUpdateStatus.StatusIndicator,
		StatusTimestamp: timeNow,
		StartTime:       timeNow,
		TenantId:        tenantID,
		AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
	}

	err := invclient.CreateOSUpdateRun(ctx, client, tenantID, runRes)
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf(
			"Creation of a new OSUpdateRun resource failed, instanceID: %s, OSUpdateRun: %v, OsUpdatePolicy: %v",
			instanceID, runRes, policy)
		return
	}
}

func updateOSUpdateRun(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID string,
	instRes *computev1.InstanceResource,
	upStatus *pb.UpdateStatus,
) {
	runRes, err := getOSUpdateRunPerIns(ctx, c, tenantID, instRes)
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to find OSUpdateRun Resource")
		return
	}

	newUpdateStatus, needed := maintgmr_util.GetUpdatedUpdateStatusIfNeeded(upStatus,
		runRes.GetStatusIndicator(), runRes.GetStatus())

	if needed {
		newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
			newUpdateStatus, upStatus, instRes.GetOs().GetOsType())

		err = invclient.UpdateOSUpdateRun(
			ctx, c, tenantID, instRes.GetResourceId(), newUpdateStatus, newUpdateStatusDetail, runRes.GetResourceId())
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf(
				"Failed to update OSUpdateRun status: tenantID=%s, instance=%s", tenantID, instRes.GetResourceId())
			return
		}
	} else {
		zlog.Debug().Msgf("No UpdateStatus change needed: old=%v, new=%v",
			newUpdateStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))
	}
}

// GetNewExistingCVEs retrieves the new existing CVEs based on the new OS Resource ID.
// or from the current OS resource if no new OS Resource ID is provided.
func GetNewExistingCVEs(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	newOSResID string,
	instRes *computev1.InstanceResource,
	newInstUpStatus *inv_status.ResourceStatus,
) (string, error) {
	var newExistingCVEs string

	if newOSResID == "" {
		// If newOSResID is os zero length, it means there is no new OS Resource update and
		// also if Instance Status is getting updated from UnSpecified to Idle then only
		// copy existing CVEs from existing OS resource
		if instRes.GetUpdateStatusIndicator() == statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED &&
			newInstUpStatus.StatusIndicator == statusv1.StatusIndication_STATUS_INDICATION_IDLE {
			newExistingCVEs = instRes.GetOs().GetExistingCves()
		}
	} else {
		// Fetch the new existing CVEs after fetching the new OS Resource based on the newOSResID
		newOSResource, osErr := invclient.GetOSResourceByID(ctx, client, tenantID, newOSResID)
		if osErr != nil {
			return "", osErr
		}
		newExistingCVEs = newOSResource.GetExistingCves()
	}

	return newExistingCVEs, nil
}

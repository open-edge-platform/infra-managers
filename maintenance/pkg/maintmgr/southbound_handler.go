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
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
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
		instance.GetOs().GetOsType(), mmUpStatus.StatusType)

	if mmUpStatus.StatusType != pb.UpdateStatus_STATUS_TYPE_UPDATED ||
		instance.GetOs().GetOsType() != os_v1.OsType_OS_TYPE_IMMUTABLE {
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

	if instance.GetOs().GetProfileName() != mmUpStatus.GetProfileName() {
		err := errors.Errorfc(codes.Internal,
			"current profile name differs from the new profile name: current profileName=%s, new profileName=%s",
			instance.GetOs().GetProfileName(), mmUpStatus.ProfileName)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}

	if instance.GetOs().GetImageId() == mmUpStatus.GetOsImageId() {
		err := errors.Errorfc(codes.Internal,
			"current and new OS Image IDs are identical: current OS ImageID=%s, new OS ImageID=%s",
			instance.GetOs().GetImageId(), mmUpStatus.OsImageId)
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", err
	}

	newOSResID, err := invclient.GetOSResourceIDByProfileInfo(ctx, c, tenantID, mmUpStatus.ProfileName, mmUpStatus.OsImageId)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("OS Resource retrieval error for profileName=%s  profileVersion=%s",
			mmUpStatus.ProfileName, mmUpStatus.OsImageId)
		return "", err
	}

	if instance.GetOs().GetResourceId() == newOSResID {
		err := errors.Errorfc(codes.Internal,
			"current and new OS Resource IDs are identical: current OS Resource ID=%s, new OS Resource ID=%s",
			instance.GetOs().GetResourceId(), newOSResID)
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
	instanceID := instRes.GetResourceId()
	newStatus := mmUpStatus.StatusType.String()
	zlog.Debug().Msgf("Handle OSUpdateRun")

	// Map pb.UpdateStatus -> local status
	targetStatuses := map[pb.UpdateStatus_StatusType]string{
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADING: status.StatusDownloading,
		pb.UpdateStatus_STATUS_TYPE_DOWNLOADED:  status.StatusDownloaded,
		pb.UpdateStatus_STATUS_TYPE_STARTED:     status.StatusUpdating,
		pb.UpdateStatus_STATUS_TYPE_UPDATED:     status.StatusCompleted,
		pb.UpdateStatus_STATUS_TYPE_FAILED:      status.StatusFailed,
	}

	targetStatus, ok := targetStatuses[mmUpStatus.StatusType]
	if !ok {
		zlog.Debug().Msgf("OSUpdateRun status ignored, instanceID: %s, status: %s", instanceID, newStatus)
		return
	}

	runRes, err := getLatestOSUpdateRunPerIns(ctx, client, tenantID, instRes)
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf("OSUpdateRun not found for instanceID: %s", instanceID)
		runRes = nil
	}

	// If no run exists, always create
	if runRes == nil {
		zlog.Debug().
			Msgf("Creating new OSUpdateRun (no existing run found), instanceID: %s, update status: %s", instanceID, newStatus)
		if _, err = createOSUpdateRun(ctx, client, tenantID, mmUpStatus, instRes); err != nil {
			zlog.Error().Err(err).Msgf("Failed to create OSUpdateRun for instanceId: %s", instanceID)
		}
		return
	}

	// Update only if new status differs
	if runRes.GetStatus() != targetStatus {
		zlog.Debug().
			Msgf("Updating OSUpdateRun status, instanceID: %s, old status: %s, new status: %s",
				instanceID, runRes.GetStatus(), targetStatus)
		if err := updateOSUpdateRun(ctx, client, tenantID, instRes, mmUpStatus, runRes); err != nil {
			zlog.Error().Err(err).Msgf("Failed to update OSUpdateRun, instanceId: %s, OSUpdateRunId: %s",
				instanceID, runRes.GetResourceId())
		}
	} else {
		zlog.Debug().
			Msgf("OSUpdateRun already up-to-date, skipping update, instanceID: %s, update status: %s, OSUpdateRunId: %s",
				instanceID, targetStatus, runRes.GetResourceId())
	}
}

func getLatestOSUpdateRunPerIns(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {
	instID := instRes.GetResourceId()
	return invclient.GetLatestOSUpdateRunByInstanceID(ctx, client, tenantID, instID, invclient.OSUpdateRunUncompleted)
}

func createOSUpdateRun(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, upStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {
	newUpdateStatus := maintgmr_util.GetUpdatedUpdateStatus(upStatus)
	instanceID := instRes.GetResourceId()
	policy := instRes.GetOsUpdatePolicy()

	timeNow, err := maintgmr_util.SafeInt64ToUint64((time.Now().Unix()))
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
		return nil, err
	}

	var endTime uint64

	if newUpdateStatus.Status == status.StatusCompleted ||
		newUpdateStatus.Status == status.StatusFailed {
		endTime = timeNow
	} else {
		endTime = invclient.SentinelEndTimeUnset // Not completed yet
	}
	runRes := &computev1.OSUpdateRunResource{
		Name:            "OS Update Run for " + instanceID, // TODO Generate unique name
		Description:     "OS Update Run for " + instanceID,
		Instance:        &computev1.InstanceResource{ResourceId: instanceID},
		Status:          newUpdateStatus.Status,
		StatusIndicator: newUpdateStatus.StatusIndicator,
		StatusTimestamp: timeNow,
		StartTime:       timeNow,
		EndTime:         endTime,
		TenantId:        tenantID,
		AppliedPolicy:   &computev1.OSUpdatePolicyResource{ResourceId: policy.GetResourceId()},
	}

	run, err := invclient.CreateOSUpdateRun(ctx, client, tenantID, runRes)
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf(
			"Creation of a new OSUpdateRun resource failed, instanceID: %s, OSUpdateRun: %v, OsUpdatePolicy: %v",
			instanceID, runRes, policy)
		return nil, err
	}

	return run, nil
}

func updateOSUpdateRun(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID string,
	instRes *computev1.InstanceResource,
	upStatus *pb.UpdateStatus,
	runRes *computev1.OSUpdateRunResource,
) error {
	newUpdateStatus, needed := maintgmr_util.GetUpdatedUpdateStatusIfNeeded(upStatus,
		runRes.GetStatusIndicator(), runRes.GetStatus())

	if needed {
		newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
			newUpdateStatus, upStatus, instRes.GetOs().GetOsType())

		err := invclient.UpdateOSUpdateRun(
			ctx, c, tenantID, instRes.GetResourceId(), newUpdateStatus, newUpdateStatusDetail, runRes.GetResourceId())
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf(
				"Failed to update OSUpdateRun status: tenantID=%s, instance=%s", tenantID, instRes.GetResourceId())
			return err
		}
	} else {
		zlog.Debug().Msgf("No UpdateStatus change needed: old=%v, new=%v",
			newUpdateStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))
	}
	return nil
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
		// If newOSResID is of zero length, it means there is no new OS Resource update and
		// also if Instance Status is getting updated from Unspecified to Idle then only
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

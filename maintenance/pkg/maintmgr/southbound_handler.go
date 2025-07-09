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
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/invclient"
	inv_utils "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
	maintgmr_util "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

func handleOSUpdateRunResourse(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) {

	switch mmUpStatus.StatusType {
	case pb.UpdateStatus_STATUS_TYPE_STARTED:
		createOSUpdateRun(ctx, client, tenantID, mmUpStatus, instRes)
	case pb.UpdateStatus_STATUS_TYPE_UPDATED, pb.UpdateStatus_STATUS_TYPE_FAILED:
		updateOSUpdateRun(ctx, client, tenantID, instRes, mmUpStatus)
	default:
		// Ignore.
		zlog.Debug().Msgf("No UpdateStatus change needed: status=%s", mmUpStatus.StatusType.String())
		return
	}
}

func getOSUpdateRunPerIns(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {

	return invclient.GetLatestOSUpdateRunByInstanceID(ctx, client, tenantID, instRes.GetResourceId())

}

func createOSUpdateRun(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, upStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) error {

	newUpdateStatus := maintgmr_util.GetUpdatedUpdateStatus(upStatus)

	timeStr := time.Now().UTC().Format(inv_utils.ISO8601Format)
	runRes := &computev1.OSUpdateRunResource{
		Name:            "",
		Instance:        &computev1.InstanceResource{ResourceId: instRes.ResourceId},
		Status:          newUpdateStatus.Status,
		StatusIndicator: newUpdateStatus.StatusIndicator,
		StatusTimestamp: timeStr,
		StartTime:       timeStr,
		TenantId:        tenantID,
		CreatedAt:       timeStr,
		AppliedPolicy:   instRes.OsUpdatePolicy,
		//UpdatedAt:       timeStr,
	}

	return invclient.CreateOSUpdateRun(ctx, client, tenantID, runRes)
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
		//updateInstance(ctx, c, tenantID, upStatus, newUpdateStatus, instRes)

		newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
			newUpdateStatus, upStatus, instRes.GetOs().GetOsType())

		err = invclient.UpdateOSUpdateRun(ctx, c, tenantID, instRes.GetResourceId(), newUpdateStatus, newUpdateStatusDetail, runRes.GetResourceId())
		if err != nil {
			// Return and continue in case of errors
			zlog.InfraSec().Warn().Err(err).Msgf("Failed to update OSUpdateRun status: tenantID=%s, instance=%s", tenantID, instRes.GetResourceId())
			return
		}

	} else {
		zlog.Debug().Msgf("No UpdateStatus change needed: old=%v, new=%v",
			newUpdateStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))
	}

}

/*func updateInstance(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	newUpdateStatus *inv_status.ResourceStatus,
	instRes *computev1.InstanceResource,
) {
	zlog.Debug().Msgf("Updating Instance UpdateStatus: old=%v, new=%v",
		newUpdateStatus, maintgmr_util.GetUpdateStatusFromInstance(instRes))

	newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
		newUpdateStatus, mmUpStatus, instRes.GetCurrentOs().GetOsType())

	newOSResID, err := GetNewOSResourceIDIfNeeded(ctx, client, tenantID, mmUpStatus, instRes)
	if err != nil {
		// Return and continue in case of errors
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to get new OS Resource")
		return
	}

	err = invclient.UpdateInstance(ctx, client, tenantID, instRes.GetResourceId(),
		*newUpdateStatus, newUpdateStatusDetail, newOSResID)
	if err != nil {
		// Return and continue in case of errors
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to update Instance Status")
		return
	}
}*/

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

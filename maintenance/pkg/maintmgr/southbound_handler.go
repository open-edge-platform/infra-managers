// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package maintmgr

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	grpc_status "google.golang.org/grpc/status"

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

func resolveOsResAndCVEsIfNeeded(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
	status *inv_status.ResourceStatus,
	needed bool,
) (osResID, existingCVEs string, err error) {
	if !needed {
		return "", "", nil
	}
	osResID, err = GetNewOSResourceIDIfNeeded(ctx, client, tenantID, mmUpStatus, instRes)
	if err != nil {
		return "", "", err
	}
	existingCVEs, err = GetNewExistingCVEs(ctx, client, tenantID, osResID, instRes, status)
	if err != nil {
		return "", "", err
	}
	return osResID, existingCVEs, nil
}

func evalOsUpdateAvailable(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	instRes *computev1.InstanceResource,
	mmUpStatus *pb.UpdateStatus,
) (osUpdateAvailable string, needed bool, err error) {
	switch instRes.GetOs().GetOsType() {
	case os_v1.OsType_OS_TYPE_IMMUTABLE:
		availableUpdateOS, err := getAvailableUpdateOS(ctx, client, tenantID, instRes)
		if err != nil {
			if grpc_status.Code(err) == codes.NotFound {
				// No newer immutable OS is available; not an error.
				zlog.InfraSec().Debug().Err(err).Msgf("Failed to get new OS Resource")
				return "", false, nil
			}
			return "", false, err
		}
		if availableUpdateOS != nil {
			return availableUpdateOS.GetName(), true, nil
		}
		return "", false, nil

	case os_v1.OsType_OS_TYPE_MUTABLE:
		if instRes.GetOsUpdateAvailable() != mmUpStatus.OsUpdateAvailable {
			return mmUpStatus.OsUpdateAvailable, true, nil
		}
		return "", false, nil

	default:
		// Includes OS_TYPE_UNSPECIFIED - should never happen.
		zlog.Debug().Msg("OS type unspecified; skipping OS update availability evaluation")
		return "", false, nil
	}
}

func buildInstanceUpdatePlan(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) (invclient.InstanceUpdatePlan, error) {
	var (
		err    error
		update invclient.InstanceUpdatePlan
	)

	// Compute update status and whether we need to update it.
	update.Status, update.Needed = maintgmr_util.GetUpdatedUpdateStatusIfNeeded(
		mmUpStatus, instRes.GetUpdateStatusIndicator(), instRes.GetUpdateStatus(),
	)

	// Only set update detail, OS and CVEs when status update is needed.
	if update.Needed {
		update.OsResID, update.ExistingCVEs, err = resolveOsResAndCVEsIfNeeded(
			ctx, client, tenantID, mmUpStatus, instRes, update.Status, update.Needed,
		)
		if err != nil {
			return update, err
		}
	}

	// Evaluate available updates for mutable/immutable OS.
	var osAvailNeeded bool
	update.OsUpdateAvailable, osAvailNeeded, err = evalOsUpdateAvailable(
		ctx, client, tenantID, instRes, mmUpStatus,
	)
	if err != nil {
		return update, err
	}
	update.Needed = update.Needed || osAvailNeeded

	return update, nil
}

func updateInventory(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	mmUpStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) {
	update, err := buildInstanceUpdatePlan(ctx, client, tenantID, mmUpStatus, instRes)
	if err != nil {
		// Return and continue in case of errors
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to build update plan for Instance")
		return
	}

	if !update.Needed {
		zlog.Debug().Msgf(
			"No Instance update needed: status=%v newOSResID=%s existingCVEs=%s osUpdateAvailable=%s",
			update.Status, update.OsResID, update.ExistingCVEs, update.OsUpdateAvailable,
		)
		return
	}

	zlog.Debug().Msgf(
		"Updating Instance UpdateStatus=%s, osUpdateAvailable=%s, newOSResID=%s, existingCVEs=%s",
		update.Status.Status, update.OsUpdateAvailable, update.OsResID, update.ExistingCVEs,
	)

	err = invclient.UpdateInstance(
		ctx,
		client,
		tenantID,
		instRes.GetResourceId(),
		update,
	)
	if err != nil {
		// Return and continue in case of errors
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to update Instance Status")
		return
	}

	zlog.Debug().Msgf(
		"Updated Instance: status=%v newOSResID=%s existingCVEs=%s osUpdateAvailable=%s",
		update.Status, update.OsResID, update.ExistingCVEs, update.OsUpdateAvailable,
	)

	// Create or update OSUpdateRun resource
	handleOSUpdateRun(ctx, invMgrCli.InvClient, tenantID, mmUpStatus, instRes)
}

func getAvailableUpdateOS(
	ctx context.Context,
	client inv_client.TenantAwareInventoryClient,
	tenantID string,
	instRes *computev1.InstanceResource,
) (*os_v1.OperatingSystemResource, error) {
	var availableUpdateOS *os_v1.OperatingSystemResource
	var err error

	availableUpdateOS, err = invclient.GetLatestImmutableOSByProfile(ctx, client, tenantID, instRes.GetOs().GetProfileName())
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf("Failed to get latest OS")
		return nil, err
	}
	if availableUpdateOS != nil {
		if availableUpdateOS.GetResourceId() != instRes.GetOs().GetResourceId() {
			if maintgmr_util.CompareImageVersions(availableUpdateOS.GetImageId(), instRes.GetOs().GetImageId()) {
				zlog.Debug().Msgf("Found available OS update: current OS ResourceID=%s, available OS ResourceID=%s",
					instRes.GetOs().GetResourceId(), availableUpdateOS.GetResourceId())
			}
		} else {
			// No update is available; return a sentinel NotFound error.
			return nil, errors.Errorfc(codes.NotFound, "no newer immutable OS available for current OS resourceID=%s",
				instRes.GetOs().GetResourceId())
		}
	}

	return availableUpdateOS, nil
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
	zlog.Info().Msgf("[handleOSUpdateRun]: instanceID=%s, newStatus=%s", instanceID, newStatus)

	// Log the policy info from the instance
	if instRes.GetOsUpdatePolicy() != nil {
		zlog.Info().Msgf("[handleOSUpdateRun]: Instance HAS OsUpdatePolicy - policyID=%s policyName=%s, ",
			instRes.GetOsUpdatePolicy().GetResourceId(), instRes.GetOsUpdatePolicy().GetName())
	}

	// Map pb.UpdateStatus to local status
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

	runRes, err := getLatestUncompletedOSUpdateRunPerIns(ctx, client, tenantID, instRes)
	if err != nil {
		zlog.InfraSec().Warn().Err(err).Msgf("OSUpdateRun not found for instanceID: %s", instanceID)
		runRes = nil
	}

	// If no uncompleted run exists, then create a new one only in case of Downloading or Started status otherwise ignore
	if runRes == nil {
		if mmUpStatus.StatusType == pb.UpdateStatus_STATUS_TYPE_DOWNLOADING ||
			mmUpStatus.StatusType == pb.UpdateStatus_STATUS_TYPE_STARTED {
			zlog.Info().Msgf("Creating new OSUpdateRun (no existing run found)")
			if _, err = createOSUpdateRun(ctx, client, tenantID, mmUpStatus, instRes); err != nil {
				zlog.Error().Err(err).Msgf("Failed to create OSUpdateRun for instanceId: %s", instanceID)
			}
		} else {
			zlog.Info().Msgf("Not creating OSUpdateRun and ignoring this event")
		}
		return
	}

	zlog.Info().Msgf("[handleOSUpdateRun]: runRes=%s, CurrentStatus=%s, NewStatus=%s",
		runRes.GetName(), runRes.GetStatus(), targetStatus)

	// Update only if new status differs
	if runRes.GetStatus() != targetStatus {
		zlog.Info().Msgf("[handleOSUpdateRun]: Updating OSUpdateRun status")
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

func getLatestUncompletedOSUpdateRunPerIns(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {
	instID := instRes.GetResourceId()
	return invclient.GetLatestOSUpdateRunByInstanceID(ctx, client, tenantID, instID, invclient.OSUpdateRunUncompleted)
}

func createOSUpdateRun(ctx context.Context, client inv_client.TenantAwareInventoryClient,
	tenantID string, upStatus *pb.UpdateStatus,
	instRes *computev1.InstanceResource,
) (*computev1.OSUpdateRunResource, error) {
	// maxLenHostName is set to 13 to ensure that the generated OS update run name
	// (which includes the hostname and timestamp) does not exceed 40 bytes.
	const maxLenHostName = 13

	newUpdateStatus := maintgmr_util.GetUpdatedUpdateStatus(upStatus)
	instanceID := instRes.GetResourceId()
	policy := instRes.GetOsUpdatePolicy()

	t := time.Now()
	timeNow, err := maintgmr_util.SafeInt64ToUint64(t.Unix())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
		return nil, err
	}

	var endTime uint64
	if newUpdateStatus.Status == status.StatusCompleted ||
		newUpdateStatus.Status == status.StatusFailed {
		endTime = timeNow
	} else {
		endTime = invclient.SentinelEndTimeUnset
	}

	// Generate unique name: <host-name> update <timestamp>
	// Truncate host name to ensure total name length stays within 40 bytes limit
	timestamp := t.Format("20060102-150405")
	hostName := instRes.GetHost().GetName()

	if len(hostName) > maxLenHostName {
		hostName = hostName[:maxLenHostName] // Truncate to max 13 characters
	}
	runName := hostName + " update " + timestamp

	runRes := &computev1.OSUpdateRunResource{
		Name:            runName,
		Description:     fmt.Sprintf("OS update for %s", instRes.GetOs().GetName()),
		Instance:        &computev1.InstanceResource{ResourceId: instanceID},
		Status:          newUpdateStatus.Status,
		StatusDetails:   upStatus.StatusDetail,
		StatusIndicator: newUpdateStatus.StatusIndicator,
		StatusTimestamp: timeNow,
		StartTime:       timeNow,
		EndTime:         endTime,
		TenantId:        tenantID,
	}

	// Allow empty policy to support cases where no policy is required for updating mutable OS
	if policy != nil {
		policyID := policy.GetResourceId()
		runRes.AppliedPolicy = &computev1.OSUpdatePolicyResource{ResourceId: policyID}
	} else {
		zlog.Info().Msgf("Creating OSUpdateRun with no applied policy, instanceID: %s", instanceID)
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

	zlog.Info().Msgf("[updateOSUpdateRun]: newStatus=%s, needed=%v", newUpdateStatus.Status, needed)

	if needed {
		newUpdateStatusDetail := maintgmr_util.GetUpdateStatusDetailIfNeeded(
			newUpdateStatus, upStatus, instRes.GetOs().GetOsType())

		zlog.Info().Msgf("[updateOSUpdateRun]: newUpdateStatusDetail=%s", newUpdateStatusDetail)

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

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package invclient

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedules_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	schedule_cache "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client/cache/schedule"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	inv_utils "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

const (
	DefaultInventoryTimeout = 5 * time.Second
	batchSize               = 1000
)

var (
	zlog = logging.GetLogger("InvClient")

	inventoryTimeout = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
)

type InvGrpcClient struct {
	InvClient            inv_client.TenantAwareInventoryClient
	HScheduleCacheClient *schedule_cache.HScheduleCacheClient
}

func NewInvGrpcClient(
	invClient inv_client.TenantAwareInventoryClient,
	hScheduleCache *schedule_cache.HScheduleCacheClient,
) InvGrpcClient {
	return InvGrpcClient{
		InvClient:            invClient,
		HScheduleCacheClient: hScheduleCache,
	}
}

func ListSingleSchedules(ctx context.Context, cli InvGrpcClient, tenantID string, hostRes *computev1.HostResource,
) ([]*schedules_v1.SingleScheduleResource, error) {
	hostID := hostRes.GetResourceId()
	return loadSingleSchedulesFromCache(ctx, cli, tenantID, hostID)
}

func loadSingleSchedulesFromCache(ctx context.Context, cli InvGrpcClient, tenantID, hostID string,
) ([]*schedules_v1.SingleScheduleResource, error) {
	allSingleSchedules := make([]*schedules_v1.SingleScheduleResource, 0)
	hasNext := true
	offset := 0
	limit := batchSize

	for hasNext {
		filters := new(schedule_cache.Filters).
			Add(schedule_cache.HasHostID(&hostID))
		sScheds, respHasNext, _, err := cli.HScheduleCacheClient.GetSingleSchedules(
			ctx, tenantID, offset, limit, filters)
		if err != nil {
			zlog.InfraErr(err).Msg("Failed to get single schedules from inventory.")
			return nil, err
		}

		if len(sScheds) == 0 {
			zlog.Debug().Msg("No more single schedules in Inventory.")
			break
		}

		allSingleSchedules = append(allSingleSchedules, sScheds...)
		hasNext = respHasNext
		offset += limit
	}

	return allSingleSchedules, nil
}

func ListRepeatedSchedules(ctx context.Context, cli InvGrpcClient, tenantID string, hostRes *computev1.HostResource,
) ([]*schedules_v1.RepeatedScheduleResource, error) {
	hostID := hostRes.GetResourceId()
	return loadRepeatedSchedulesFromCache(ctx, cli, tenantID, hostID)
}

func loadRepeatedSchedulesFromCache(ctx context.Context, cli InvGrpcClient, tenantID, hostID string,
) ([]*schedules_v1.RepeatedScheduleResource, error) {
	hasNext := true
	offset := 0
	limit := batchSize
	allRepeatedSchedules := make([]*schedules_v1.RepeatedScheduleResource, 0)

	for hasNext {
		filters := new(schedule_cache.Filters).
			Add(schedule_cache.HasHostID(&hostID))

		rScheds, respHasNext, _, err := cli.HScheduleCacheClient.GetRepeatedSchedules(
			ctx, tenantID, offset, limit, filters)
		if err != nil {
			zlog.InfraErr(err).Msg("Failed to get repeated schedules from inventory.")
			return nil, nil
		}

		if len(rScheds) == 0 {
			zlog.Debug().Msg("No more repeated schedules in Inventory.")
			break
		}

		allRepeatedSchedules = append(allRepeatedSchedules, rScheds...)

		hasNext = respHasNext
		offset += limit
	}

	return allRepeatedSchedules, nil
}

func GetInstanceResourceByHostGUID(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostGUID string,
) (
	*computev1.InstanceResource,
	error,
) {
	zlog.Debug().Msgf("GetInstanceResourceByHostGUID: tenantID=%s, HostGUID=%s", tenantID, hostGUID)
	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	host, err := c.GetHostByUUID(childCtx, tenantID, hostGUID)
	if err != nil {
		return nil, err
	}

	if err = validator.ValidateMessage(host); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, errors.Wrap(err)
	}
	if host.GetInstance() == nil {
		return nil, errors.Errorfc(codes.NotFound, "Instance not found: tenantID=%s, hostID=%s", tenantID, host.GetResourceId())
	}

	// Flip around the eager loading
	instance := host.GetInstance()
	host.Instance = nil
	instance.Host = host
	return instance, nil
}

func GetOSUpdatePolicyByInstanceID(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, instanceID string,
) (*computev1.OSUpdatePolicyResource, error) {
	// TODO: Optimize and use caches, we could use ResourceID based caches.
	zlog.Debug().Msgf("GetOSUpdatePolicyByInstanceID: tenantID=%s, InstanceID=%s", tenantID, instanceID)
	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	// First retrieve the Instance resource.
	instanceResp, err := c.Get(childCtx, tenantID, instanceID)
	if err != nil {
		return nil, err
	}
	if err = validator.ValidateMessage(instanceResp); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, errors.Wrap(err)
	}
	instance, err := util.UnwrapResource[*computev1.InstanceResource](instanceResp.GetResource())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to unwrap resource: %s", instanceResp.GetResource())
		return nil, err
	}
	if instance.GetOsUpdatePolicy() == nil {
		zlog.InfraSec().Warn().Msgf("OSUpdatePolicy not present in the Instance Resource: %s", instanceResp.GetResource())
		return &computev1.OSUpdatePolicyResource{}, nil
	}

	// Now retrieve the OSUpdatePolicy resource, so we get all eager loaded fields.
	osPolicyUpdateResp, err := c.Get(childCtx, tenantID, instance.GetOsUpdatePolicy().GetResourceId())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf(
			"Failed to get OSUpdatePolicy: tenantID=%s, instanceID=%s", tenantID, instanceID)
		return nil, err
	}
	if err = validator.ValidateMessage(osPolicyUpdateResp); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, errors.Wrap(err)
	}
	osUpdatePolicy, err := util.UnwrapResource[*computev1.OSUpdatePolicyResource](osPolicyUpdateResp.GetResource())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to unwrap resource: %s", instanceResp.GetResource())
		return nil, err
	}

	zlog.Debug().Msgf("GetOSUpdatePolicyByInstanceID: tenantID=%s, InstanceID=%s, OSUpdatePolicyID=%s",
		tenantID, instanceID, osUpdatePolicy.GetResourceId())

	return osUpdatePolicy, nil
}

func UpdateInstance(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID string,
	instanceID string,
	updateStatus inv_status.ResourceStatus,
	updateStatusDetail string,
	newOSResID string,
) error {
	zlog.Debug().Msgf("UpdateInstanceStatus: tenantID=%s, InstanceID=%s, NewUpdateStatus=%v, LastUpdateDetail=%s",
		tenantID, instanceID, updateStatus, updateStatusDetail)

	timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
		return err
	}

	instRes := &computev1.InstanceResource{
		UpdateStatus:          updateStatus.Status,
		UpdateStatusIndicator: updateStatus.StatusIndicator,
		UpdateStatusTimestamp: timeNow,
	}

	fields := []string{
		computev1.InstanceResourceFieldUpdateStatus,
		computev1.InstanceResourceFieldUpdateStatusIndicator,
		computev1.InstanceResourceFieldUpdateStatusTimestamp,
	}

	if updateStatusDetail != "" {
		instRes.UpdateStatusDetail = updateStatusDetail
		fields = append(fields, computev1.InstanceResourceFieldUpdateStatusDetail)
	}

	if newOSResID != "" {
		instRes.CurrentOs = &os_v1.OperatingSystemResource{ResourceId: newOSResID}
		fields = append(fields, computev1.InstanceResourceEdgeCurrentOs)
	}

	fieldMask, err := fieldmaskpb.New(instRes, fields...)
	if err != nil {
		// This should never happen
		zlog.InfraSec().InfraErr(err).Msg("should never happen")
		return err
	}

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	_, err = c.Update(childCtx, tenantID, instanceID, fieldMask, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Instance{
			Instance: instRes,
		},
	})
	return err
}

func GetOSResourceIDByProfileInfo(ctx context.Context, c inv_client.TenantAwareInventoryClient,
	tenantID, profileName, osImageID string) (
	string, error,
) {
	zlog.Debug().Msgf("GetOSResourceIDByProfileInfo: ProfileName=%s, OSImageID:%s", profileName, osImageID)
	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	filter := fmt.Sprintf("%s = %q AND %s = %q AND %s = %q",
		os_v1.OperatingSystemResourceFieldProfileName, profileName,
		os_v1.OperatingSystemResourceFieldImageId, osImageID,
		os_v1.OperatingSystemResourceFieldTenantId, tenantID,
	)

	findResp, err := c.Find(childCtx, &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Os{}},
		Filter:   filter,
	})
	if err != nil {
		return "", err
	}

	if err = util.CheckListOutputIsSingular(findResp.GetResources()); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Expected one resource, received multiple: %s", findResp)
		return "", err
	}

	if err = validator.ValidateMessage(findResp); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", errors.Wrap(err)
	}
	osResID := findResp.GetResources()[0].GetResourceId()
	zlog.Debug().Msgf("Found resource ID: %s", osResID)

	return osResID, nil
}

func GetLatestImmutableOSByProfile(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID, profileName string,
) (*os_v1.OperatingSystemResource, error) {
	// TODO: Add caching layer
	zlog.Debug().Msgf("GetLatestImmutableOSByProfile: tenantID=%s, profileName=%s", tenantID, profileName)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	filter := fmt.Sprintf("%s=%q AND %s=%s AND %s=%q",
		os_v1.OperatingSystemResourceFieldTenantId, tenantID,
		os_v1.OperatingSystemResourceFieldOsType, os_v1.OsType_OS_TYPE_IMMUTABLE.String(),
		os_v1.OperatingSystemResourceFieldProfileName, profileName,
	)

	resp, err := c.List(childCtx, &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Os{}},
		Filter:   filter,
	})
	if err != nil {
		return nil, err
	}

	if len(resp.GetResources()) == 0 {
		return nil, errors.Errorfc(
			codes.NotFound, "OS resource not found: tenantID=%s, profile_name=%s", tenantID, profileName)
	}

	// Find the OS profile with the highest semantic version using Masterminds/semver
	var latestOS *os_v1.OperatingSystemResource
	var latestVersion *semver.Version

	for _, resource := range resp.GetResources() {
		os := resource.GetResource().GetOs()
		if err := validator.ValidateMessage(os); err != nil {
			zlog.Warn().Err(err).Msgf("Invalid OS resource: %s", os.GetResourceId())
			continue // Skip invalid OS resources
		}

		currentVersionStr := os.GetProfileVersion()
		currentVersion, err := semver.NewVersion(currentVersionStr)
		if err != nil {
			zlog.Warn().Err(err).Msgf("Failed to parse semantic version: %s", currentVersionStr)
			continue // Skip this OS if the version is invalid
		}

		// If this is the first valid version we've seen, or if it's higher than our current latest
		if latestVersion == nil || currentVersion.GreaterThan(latestVersion) {
			latestOS = os
			latestVersion = currentVersion
		}
	}

	if latestOS == nil {
		return nil, errors.Errorfc(
			codes.NotFound, "No valid OS resource found: tenantID=%s, profile_name=%s", tenantID, profileName)
	}

	zlog.Debug().Msgf("Found OS resource with resourceID: %s, version: %s",
		latestOS.GetResourceId(), latestVersion.String())

	return latestOS, nil
}

func CreateOSUpdateRun(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, osUpRun *computev1.OSUpdateRunResource,
) error {
	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	zlog.Info().Msgf("Create a new OSUpdateRun resource: %v", osUpRun)
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_OsUpdateRun{
			OsUpdateRun: osUpRun,
		},
	}
	runRes, err := c.Create(childCtx, tenantID, res)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to create OSUpdateRun resource. OSUpdateRun: %v", res.GetOsUpdateRun())
		return err
	}

	zlog.Info().Msgf("New OSUpdateRun resource created. OSUpdateRun: %v", runRes)

	return err
}

func DeleteOSUpdateRun(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, osUpRun *computev1.OSUpdateRunResource,
) error {
	zlog.Info().Msgf("Delete OSUpdateRun resource: %v", osUpRun)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	_, err := c.Delete(childCtx, tenantID, osUpRun.GetResourceId())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to delete OSUpdateRun resource, resourceID: %s",
			osUpRun.GetResourceId())
		return err
	}
	zlog.Debug().Msgf("Deleted OSUpdateRun resource, resourseID: %s", osUpRun.GetResourceId())

	return err
}

func UpdateOSUpdateRun(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID string,
	instanceID string,
	updateStatus *inv_status.ResourceStatus,
	updateStatusDetail string,
	runResID string,
) error {
	zlog.Debug().Msgf(
		"UpdateInstanceStatus: tenantID=%s, InstanceID=%s, OSUpdateRunID=%s, NewUpdateStatus=%v, LastUpdateDetail=%s",
		tenantID, instanceID, runResID, &updateStatus, updateStatusDetail)

	now := time.Now().UTC().Format(inv_utils.ISO8601Format)
	run := &computev1.OSUpdateRunResource{
		Status:          updateStatus.Status,
		StatusIndicator: updateStatus.StatusIndicator,
		StatusTimestamp: now,
	}

	fields := []string{
		computev1.OSUpdateRunResourceFieldStatus,
		computev1.OSUpdateRunResourceFieldStatusIndicator,
		computev1.OSUpdateRunResourceFieldStatusTimestamp,
	}

	if updateStatusDetail != "" {
		run.StatusDetails = updateStatusDetail
		fields = append(fields, computev1.OSUpdateRunResourceFieldStatusDetails)
	}

	if updateStatus.Status == status.StatusCompleted ||
		updateStatus.Status == status.StatusFailed {
		run.EndTime = now
		fields = append(fields, computev1.OSUpdateRunResourceFieldEndTime)
	}

	fieldMask, err := fieldmaskpb.New(run, fields...)
	if err != nil {
		// This should never happen
		zlog.InfraSec().InfraErr(err).Msg("should never happen")
		return err
	}

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	_, err = c.Update(childCtx, tenantID, runResID, fieldMask, &inv_v1.Resource{
		Resource: &inv_v1.Resource_OsUpdateRun{
			OsUpdateRun: run,
		},
	})
	return err
}

func GetLatestOSUpdateRunByInstanceID(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID, instID string,
) (*computev1.OSUpdateRunResource, error) {
	// TODO: Add caching layer
	zlog.Debug().Msgf("GetLatestOSUpdateRunByInstanceID: tenantID=%s, instance=%s", tenantID, instID)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	filter := fmt.Sprintf("%s=%q AND %s.%s=%q",
		computev1.OSUpdateRunResourceFieldTenantId, tenantID,
		computev1.OSUpdateRunResourceEdgeInstance,
		computev1.InstanceResourceFieldResourceId, instID,
		// TODO: unset computev1.OSUpdateRunResourceFieldEndTime,
	)

	resp, err := c.List(childCtx, &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_OsUpdateRun{}},
		Filter:   filter,
		OrderBy:  "start_time desc",
		Limit:    1,
	})
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("GetLatestOSUpdateRunByInstanceID: tenanatID=%s, instance=%s", tenantID, instID)
		return nil, err
	}

	if len(resp.GetResources()) == 0 {
		return nil, errors.Errorfc(
			codes.NotFound, "OSUpdateRun resource not found: tenantID=%s, instance=%s", tenantID, instID)
	}

	run := resp.GetResources()[0].GetResource().GetOsUpdateRun()
	if err := validator.ValidateMessage(run); err != nil {
		return nil, errors.Wrap(err)
	}

	zlog.Debug().Msgf("Found OSUpdateRun resource with resourceID: %s", run.GetResourceId())

	return run, nil
}

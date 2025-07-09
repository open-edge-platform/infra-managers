// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package invclient

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"time"

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

	filter := fmt.Sprintf("%s = %q AND %s = %q AND %s = %q", os_v1.OperatingSystemResourceFieldProfileName, profileName,
		os_v1.OperatingSystemResourceFieldImageId, osImageID, os_v1.OperatingSystemResourceFieldTenantId, tenantID)

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

func CreateOSUpdateRun(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, osUpRun *computev1.OSUpdateRunResource,
) error {
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_OsUpdateRun{
			OsUpdateRun: osUpRun,
		},
	}

	res, err := c.Create(ctx, tenantID, res)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to create OSUpdateRun resourse: instance=%s", res.GetInstance().GetResourceId())
		return err
	}

	zlog.Debug().Msgf("New OSUpdateRun resourse created: OSUpdateRun=%v", res)

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
	zlog.Debug().Msgf("UpdateInstanceStatus: tenantID=%s, InstanceID=%s, OSUpdateRunID=%s, NewUpdateStatus=%v, LastUpdateDetail=%s",
		tenantID, instanceID, runResID, &updateStatus, updateStatusDetail)

	timeNow, err := inv_utils.SafeInt64ToUint64(time.Now().Unix())
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
		return err
	}
	timeStr := strconv.FormatUint(timeNow, 10)
	run := &computev1.OSUpdateRunResource{
		Status:          updateStatus.Status,
		StatusIndicator: updateStatus.StatusIndicator,
		StatusTimestamp: timeStr,
		UpdatedAt:       timeStr,
		EndTime:         timeStr,
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

	filter := fmt.Sprintf("%s=%q AND %s.%s=%q AND %s=%q",
		computev1.OSUpdateRunResourceFieldTenantId, tenantID,
		computev1.OSUpdateRunResourceEdgeInstance,
		computev1.InstanceResourceFieldResourceId, instID,
		computev1.OSUpdateRunResourceFieldEndTime, "",
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

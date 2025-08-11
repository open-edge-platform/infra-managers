// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedule_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	mmgr_error "github.com/open-edge-platform/infra-managers/maintenance/pkg/errors"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	"github.com/open-edge-platform/infra-managers/maintenance/pkg/statusdetail"
	om_status "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/status"
)

var zlog = logging.GetLogger("MaintenanceManagerUtils")

const (
	EnableSanitizeGrpcErr            = "enableSanitizeGrpcErr"
	EnableSanitizeGrpcErrDescription = "enable to sanitize grpc error of each RPC call"
	ISO8601Format                    = "2006-01-02T15:04:05.999Z"
)

func IsInstanceNotProvisioned(instance *computev1.InstanceResource) bool {
	if instance == nil {
		// If a host has no Instance, it's not provisioned yet
		return true
	}
	return instance.ProvisioningStatusIndicator != om_status.ProvisioningStatusDone.StatusIndicator &&
		instance.ProvisioningStatus != om_status.ProvisioningStatusDone.Status
}

func IsHostUntrusted(hostres *computev1.HostResource) bool {
	// this can mean a state inconsistency if desired state != current state, but for safety we check both.
	// eventually we should only check the desired state,
	// if we're confident that upper layers will handle state inconsistency properly.
	return hostres.GetCurrentState() == computev1.HostState_HOST_STATE_UNTRUSTED ||
		hostres.GetDesiredState() == computev1.HostState_HOST_STATE_UNTRUSTED
}

// PopulateUpdateSchedule returns a populated SB UpdateSchedule given the provided repeated schedules and single schedule.
func PopulateUpdateSchedule(rsResources []*schedule_v1.RepeatedScheduleResource,
	ssRes *schedule_v1.SingleScheduleResource,
) (*pb.UpdateSchedule, error) {
	var sche pb.UpdateSchedule
	if len(rsResources) > 0 {
		// Populate the deprecated field
		singleRsRes := rsResources[0]
		//nolint:staticcheck // deprecated RPC will be removed in future
		sche.RepeatedSchedule = &pb.RepeatedSchedule{
			DurationSeconds: singleRsRes.DurationSeconds,
			CronMinutes:     singleRsRes.CronMinutes,
			CronHours:       singleRsRes.CronHours,
			CronDayMonth:    singleRsRes.CronDayMonth,
			CronMonth:       singleRsRes.CronMonth,
			CronDayWeek:     singleRsRes.CronDayWeek,
		}

		// Populate the newly added field (not deprecated)
		for _, rsrsp := range rsResources {
			repeatedsche := pb.RepeatedSchedule{}
			repeatedsche.DurationSeconds = rsrsp.DurationSeconds
			repeatedsche.CronMinutes = rsrsp.CronMinutes
			repeatedsche.CronHours = rsrsp.CronHours
			repeatedsche.CronDayMonth = rsrsp.CronDayMonth
			repeatedsche.CronMonth = rsrsp.CronMonth
			repeatedsche.CronDayWeek = rsrsp.CronDayWeek
			// append to repeated schedule
			sche.RepeatedSchedules = append(sche.RepeatedSchedules, &repeatedsche)
		}
		zlog.Debug().Msgf("Returning repeated schedule: repeatedSched=%v", rsResources)
	}
	if ssRes != nil {
		sche.SingleSchedule = &pb.SingleSchedule{
			StartSeconds: ssRes.StartSeconds,
			EndSeconds:   ssRes.EndSeconds,
		}
	}
	if err := sche.ValidateAll(); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, mmgr_error.Wrap(err)
	}
	zlog.Debug().Msgf("Returning updated schedule: single and repeated sched=%v", &sche)
	return &sche, nil
}

func PopulateOsProfileUpdateSource(os *os_v1.OperatingSystemResource) (*pb.OSProfileUpdateSource, error) {
	osProfileUpdateSource := &pb.OSProfileUpdateSource{}

	if os == nil {
		err := inv_errors.Errorfc(codes.Internal, "missing OSUpdatePolicy resource")
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, err
	}

	if os.GetOsType() == os_v1.OsType_OS_TYPE_IMMUTABLE {
		osProfileUpdateSource.ProfileName = os.ProfileName
		osProfileUpdateSource.ProfileVersion = os.ProfileVersion
		osProfileUpdateSource.OsImageUrl = os.ImageUrl
		osProfileUpdateSource.OsImageId = os.ImageId
		osProfileUpdateSource.OsImageSha = os.Sha256
	} else {
		err := inv_errors.Errorfc(codes.Internal, "unsupported OS type: %s", os.GetOsType())
		zlog.InfraSec().InfraErr(err).Msgf("Wrong OS type, we expect IMMUTABLE")
		return nil, err
	}
	return osProfileUpdateSource, nil
}

// GetClosestSingleSchedule Returns the closest single schedule from time.Now.
func GetClosestSingleSchedule(sScheds []*schedule_v1.SingleScheduleResource) *schedule_v1.SingleScheduleResource {
	var ssRet *schedule_v1.SingleScheduleResource
	currentTime := time.Now().In(time.UTC)
	durationMin := time.Duration(1<<63 - 1)
	duration := time.Duration(1<<63 - 1)

	for _, ssRes := range sScheds {
		// the single schedule is valid only when the current time is before end window.
		endTimeSec, err := SafeUint64ToInt64(ssRes.GetEndSeconds())
		if err != nil {
			zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
			return ssRet
		}

		startTimeSec, err := SafeUint64ToInt64(ssRes.GetStartSeconds())
		if err != nil {
			zlog.InfraSec().InfraErr(err).Msg("Conversion Overflow Error")
			return ssRet
		}

		if ssRes.GetEndSeconds() != 0 && currentTime.Compare(time.Unix(endTimeSec, 0)) == -1 {
			duration = currentTime.Sub(time.Unix(startTimeSec, 0)).Abs()
			// end time is not set.
		} else if ssRes.GetEndSeconds() == 0 {
			duration = currentTime.Sub(time.Unix(startTimeSec, 0)).Abs()
		}

		if duration < durationMin {
			durationMin = duration
			ssRet = ssRes
		}
	}

	zlog.Debug().Msgf("Closest Single Schedule: singleSched=%v", ssRet)
	return ssRet
}

func GetUpdateStatusFromInstance(inst *computev1.InstanceResource) inv_status.ResourceStatus {
	return inv_status.ResourceStatus{
		StatusIndicator: inst.UpdateStatusIndicator,
		Status:          inst.UpdateStatus,
	}
}

func compareUpdateStatuses(instanceUpdateStatusInd statusv1.StatusIndication, instanceUpdateStatusMessage string,
	status *inv_status.ResourceStatus,
) bool {
	return (instanceUpdateStatusInd == status.StatusIndicator) && (strings.Contains(instanceUpdateStatusMessage, status.Status))
}

func returnUpdateStatusNeed(newStatus *inv_status.ResourceStatus,
	statIndicator statusv1.StatusIndication, statMessage string,
) (*inv_status.ResourceStatus, bool) {
	if compareUpdateStatuses(statIndicator, statMessage, newStatus) {
		return newStatus, false
	}
	return newStatus, true
}

func GetUpdatedUpdateStatusIfNeeded(newUpdateStatus *pb.UpdateStatus,
	instStatusInd statusv1.StatusIndication, instUpdateMessage string) (
	*inv_status.ResourceStatus, bool,
) {
	switch newUpdateStatus.StatusType {
	case pb.UpdateStatus_STATUS_TYPE_STARTED:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusInProgress, instStatusInd, instUpdateMessage)
	case pb.UpdateStatus_STATUS_TYPE_UPDATED:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusDone, instStatusInd, instUpdateMessage)
	case pb.UpdateStatus_STATUS_TYPE_FAILED:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusFailed, instStatusInd, instUpdateMessage)
	case pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusUpToDate, instStatusInd, instUpdateMessage)
	case pb.UpdateStatus_STATUS_TYPE_DOWNLOADING:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusDownloading, instStatusInd, instUpdateMessage)
	case pb.UpdateStatus_STATUS_TYPE_DOWNLOADED:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusDownloaded, instStatusInd, instUpdateMessage)
	default:
		return returnUpdateStatusNeed(
			&mm_status.UpdateStatusUnknown, instStatusInd, instUpdateMessage)
	}
}

func validateJSONSchema(jsonStr string) (int, error) {
	var detail statusdetail.DetailLog
	if err := json.Unmarshal([]byte(jsonStr), &detail); err != nil {
		return 0, err
	}

	if detail.UpdateLog == nil {
		err := inv_errors.Errorfc(codes.Internal, "Missing required key: update_log")
		return 0, err
	}
	return len(detail.UpdateLog), nil
}

func GetUpdateStatusDetailIfNeeded(invUpStatus *inv_status.ResourceStatus,
	mmUpStatus *pb.UpdateStatus, osType os_v1.OsType,
) string {
	// Check if field is not empty for PUA backward compatibility
	if mmUpStatus.StatusDetail == "" {
		return ""
	}

	switch *invUpStatus {
	case mm_status.UpdateStatusDone:
		logSize, err := validateJSONSchema(mmUpStatus.StatusDetail)
		if err != nil {
			zlog.InfraSec().InfraErr(err).Msg("status detail validation error")
			return ""
		}
		if osType == os_v1.OsType_OS_TYPE_MUTABLE {
			invUpStatus.Status = fmt.Sprintf("%s %s %d %s", invUpStatus.Status, "-", logSize, "package(s) updated/installed")
		}
		return mmUpStatus.StatusDetail

	case mm_status.UpdateStatusFailed:
		_, err := validateJSONSchema(mmUpStatus.StatusDetail)
		if err != nil {
			zlog.InfraSec().InfraErr(err).Msg("status detail validation error")
			return ""
		}
		return mmUpStatus.StatusDetail
	default:
		return ""
	}
}

func SafeUint64ToInt64(u uint64) (int64, error) {
	if u > math.MaxInt64 {
		return 0, inv_errors.Errorfc(codes.InvalidArgument, "uint64 value exceeds int64 range")
	}
	return int64(u), nil
}

func SafeInt64ToUint64(i int64) (uint64, error) {
	if i < 0 {
		return 0, inv_errors.Errorfc(codes.InvalidArgument, "int64 value is negative and cannot be converted to uint64")
	}
	return uint64(i), nil
}

func GetUpdatedUpdateStatus(newUpdateStatus *pb.UpdateStatus) *inv_status.ResourceStatus {
	switch newUpdateStatus.StatusType {
	case pb.UpdateStatus_STATUS_TYPE_STARTED:
		return &mm_status.UpdateStatusInProgress
	case pb.UpdateStatus_STATUS_TYPE_UPDATED:
		return &mm_status.UpdateStatusDone
	case pb.UpdateStatus_STATUS_TYPE_FAILED:
		return &mm_status.UpdateStatusFailed
	case pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE:
		return &mm_status.UpdateStatusUpToDate
	case pb.UpdateStatus_STATUS_TYPE_DOWNLOADING:
		return &mm_status.UpdateStatusDownloading
	case pb.UpdateStatus_STATUS_TYPE_DOWNLOADED:
		return &mm_status.UpdateStatusDownloaded
	default:
		return &mm_status.UpdateStatusUnknown
	}
}

func ConvertToComparableSemVer(s string) (string, error) {
	// convert image version string to a comparable semantic version format
	// Example: "3.0.20250717.0732" -> "3.0.20250717-build0732"
	parts := strings.Split(s, ".")
	if len(parts) < 3 {
		return "", fmt.Errorf("invalid version string: %s", s)
	}

	// Core: Major.Minor.Patch
	core := make([]string, 3)
	copy(core, parts[:3])
	for i := range core {
		if core[i] == "" {
			return "", fmt.Errorf("invalid version segment: empty")
		}
		n, err := strconv.Atoi(core[i])
		if err != nil {
			return "", fmt.Errorf("invalid number segment: %s", core[i])
		}
		core[i] = strconv.Itoa(n) // remove leading zeros
	}

	// Prerelease — add prefix "build" to segment to avoid starting with 0
	var prerelease string
	if len(parts) > 3 {
		preParts := parts[3:]
		for i, p := range preParts {
			if len(p) > 1 && strings.HasPrefix(p, "0") {
				preParts[i] = "build" + p
			}
		}
		prerelease = strings.Join(preParts, ".")
	}

	if prerelease != "" {
		return fmt.Sprintf("%s-%s", strings.Join(core, "."), prerelease), nil
	}
	return strings.Join(core, "."), nil
}

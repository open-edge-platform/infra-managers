// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package utils_test

import (
	"sort"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	os_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	schedule_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	mm_testing "github.com/open-edge-platform/infra-managers/maintenance/internal/testing"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	util "github.com/open-edge-platform/infra-managers/maintenance/pkg/utils"
)

func TestGetHostSchedule(t *testing.T) {
	tests := []struct {
		name     string
		rSched   []*schedule_v1.RepeatedScheduleResource
		sSched   *schedule_v1.SingleScheduleResource
		wantSche *pb.UpdateSchedule
		valid    bool
	}{
		{
			name:   "InvalidRepeatedSched",
			rSched: []*schedule_v1.RepeatedScheduleResource{{CronMinutes: "100"}},
			valid:  false,
		},
		{
			name:     "No_Sched",
			wantSche: &pb.UpdateSchedule{},
			valid:    true,
		},
		{
			name:     "OnlySingleSched1",
			sSched:   &mm_testing.SingleSchedule0,
			wantSche: &pb.UpdateSchedule{SingleSchedule: &mm_testing.MmSingleSchedule0},
			valid:    true,
		},
		{
			name:     "OnlySingleSched2",
			sSched:   &mm_testing.SingleSchedule3,
			wantSche: &pb.UpdateSchedule{SingleSchedule: &mm_testing.MmSingleSchedule3},
			valid:    true,
		},
		{
			name:   "OnlyRepeatedSched",
			rSched: []*schedule_v1.RepeatedScheduleResource{&mm_testing.RepeatedSchedule1},
			wantSche: &pb.UpdateSchedule{
				RepeatedSchedule:  mm_testing.MmRepeatedSchedule1[0],
				RepeatedSchedules: mm_testing.MmRepeatedSchedule1,
			},
			valid: true,
		},
		{
			name:   "MultipleRepeatedSched",
			rSched: []*schedule_v1.RepeatedScheduleResource{&mm_testing.RepeatedSchedule2, &mm_testing.RepeatedSchedule1},
			wantSche: &pb.UpdateSchedule{
				RepeatedSchedule:  mm_testing.MmRepeatedSchedule2[0],
				RepeatedSchedules: mm_testing.MmRepeatedSchedule2,
			},
			valid: true,
		},
		{
			name:   "BothSingleAndRepeatedSched",
			sSched: &mm_testing.SingleSchedule3,
			rSched: []*schedule_v1.RepeatedScheduleResource{&mm_testing.RepeatedSchedule1},
			wantSche: &pb.UpdateSchedule{
				SingleSchedule:    &mm_testing.MmSingleSchedule3,
				RepeatedSchedule:  mm_testing.MmRepeatedSchedule1[0],
				RepeatedSchedules: mm_testing.MmRepeatedSchedule1,
			},
			valid: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := util.PopulateUpdateSchedule(tt.rSched, tt.sSched)
			if !tt.valid {
				require.Error(t, err)
				require.Nil(t, sched)
			} else {
				require.NoError(t, err, errors.ErrorToStringWithDetails(err))
				if eq, diff := inv_testing.ProtoEqualOrDiff(tt.wantSche, sched); !eq {
					t.Errorf("Wrong host schedule: %v", diff)
				}
			}
		})
	}
}

func TestGetClosestSingleSchedule(t *testing.T) {
	tests := []struct {
		name string
		args []*schedule_v1.SingleScheduleResource
		want *schedule_v1.SingleScheduleResource
	}{
		{
			name: "MissingSingleSched1",
			want: nil,
		},
		{
			name: "MissingSingleSched2",
			args: []*schedule_v1.SingleScheduleResource{},
			want: nil,
		},
		{
			name: "SingleSchedInPast",
			args: []*schedule_v1.SingleScheduleResource{&mm_testing.SingleSchedulePast},
		},
		{
			name: "SingleSchedInPastContinuing",
			args: []*schedule_v1.SingleScheduleResource{&mm_testing.SingleSchedulePastContinuing},
			want: &mm_testing.SingleSchedulePastContinuing,
		},
		{
			name: "ValidSingleSchedsWithPast",
			args: []*schedule_v1.SingleScheduleResource{
				&mm_testing.SingleSchedule2,
				&mm_testing.SingleSchedulePastContinuing,
				&mm_testing.SingleSchedule3,
				&mm_testing.SingleSchedule1,
			},
			want: &mm_testing.SingleSchedule1,
		},
		{
			name: "ValidSingleScheds1",
			args: []*schedule_v1.SingleScheduleResource{
				&mm_testing.SingleSchedule2,
				&mm_testing.SingleSchedulePast,
				&mm_testing.SingleSchedule3,
				&mm_testing.SingleSchedule1,
			},
			want: &mm_testing.SingleSchedule1,
		},
		{
			name: "ValidSingleScheds2",
			args: []*schedule_v1.SingleScheduleResource{
				&mm_testing.SingleSchedule2,
				&mm_testing.SingleSchedule3,
				&mm_testing.SingleSchedule0,
				&mm_testing.SingleSchedule1,
			},
			want: &mm_testing.SingleSchedule0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			closest := util.GetClosestSingleSchedule(tt.args)
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, closest); !eq {
				t.Errorf("Wrong closest single schedule: %v", diff)
			}
		})
	}
}

var (
	// invalid cron fields in repeated schedule.
	rsche1 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-1",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "/5", // invalid
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	rsche2 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-2",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "*",
			CronHours:       "4-5", // invalid
			CronDayMonth:    "5",
			CronMonth:       "*",
			CronDayWeek:     "*",
		},
	}
	rsche3 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-3",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "60", // invalid
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	rsche4 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-4",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "24", // invalid
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	rsche5 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-5",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "4",
			CronDayMonth:    "32", // invalid
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	rsche6 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-6",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "13", // invalid
			CronDayWeek:     "0",
		},
	}
	rsche7 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-7",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "7", // invalid
		},
	}
	rsche8 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-8",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "1,2,3", // valid
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	rsche9 = []*schedule_v1.RepeatedScheduleResource{
		{
			Name:            "repeatedSchedule-9",
			ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
			DurationSeconds: uint32(10),
			CronMinutes:     "5",
			CronHours:       "1,2",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "?", // invalid
		},
	}
)

func TestValidateCronFieldsRepeatedSchedule(t *testing.T) {
	tests := []struct {
		name   string
		rSched []*schedule_v1.RepeatedScheduleResource
		sSched *schedule_v1.SingleScheduleResource
		valid  bool
	}{
		{
			name:   "InvalidRepeatedSched1",
			rSched: rsche1,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched2",
			rSched: rsche2,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched3",
			rSched: rsche3,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched4",
			rSched: rsche4,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched5",
			rSched: rsche5,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched6",
			rSched: rsche6,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched7",
			rSched: rsche7,
			valid:  false,
		},
		{
			name:   "InvalidRepeatedSched8",
			rSched: rsche8,
			valid:  true,
		},
		{
			name:   "InvalidRepeatedSched9",
			rSched: rsche9,
			valid:  false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := util.PopulateUpdateSchedule(tc.rSched, tc.sSched)
			if err != nil {
				if !tc.valid {
					assert.Error(t, err)
				}
			}
		})
	}
}

func TestIsHostUntrusted(t *testing.T) {
	type args struct {
		hostres *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "IsTrusted",
			args: args{
				hostres: &computev1.HostResource{
					DesiredState: computev1.HostState_HOST_STATE_DELETED,
					CurrentState: computev1.HostState_HOST_STATE_ONBOARDED,
				},
			},
			want: false,
		},
		{
			name: "Untrusted from desired state",
			args: args{
				hostres: &computev1.HostResource{
					DesiredState: computev1.HostState_HOST_STATE_UNTRUSTED,
					CurrentState: computev1.HostState_HOST_STATE_ONBOARDED,
				},
			},
			want: true,
		},
		{
			name: "Untrusted from current state",
			args: args{
				hostres: &computev1.HostResource{
					DesiredState: computev1.HostState_HOST_STATE_DELETED,
					CurrentState: computev1.HostState_HOST_STATE_UNTRUSTED,
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, util.IsHostUntrusted(tt.args.hostres), "IsHostUntrusted(%v)", tt.args.hostres)
		})
	}
}

//nolint:funlen // it's a test
func TestGetUpdateStatusDetailIfNeeded(t *testing.T) {
	validDetailJSON1 := `{
		"update_log": [
			{
				"update_type": "application",
				"package_name": "package-abc",
				"update_time": "2024-05-08T12:34:56Z",
				"action": "install",
				"status": "installed",
				"version": "1.0.1"
			}
		]
	}`
	validDetailJSON2 := `{
		"update_log": [
			{
				"update_type": "application",
				"package_name": "package-abc",
				"update_time": "2024-05-08T12:34:56Z",
				"action": "install",
				"status": "installed",
				"version": "1.0.1",
				"failure_reason": "nofailure"

			},
			{
				"update_type": "application",
				"package_name": "package-def",
				"update_time": "2024-05-08T12:34:56Z",
				"action": "install",
				"status": "installed",
				"version": "1.0.5",
				"failure_reason": "nofailure"
			}
		]
	}`

	validDetailJSON3 := `{
		"update_log": [
			{
				"update_type": "OS",
				"package_name": "edge-microvisor-toolkit-image",
				"update_time": "2024-05-08T12:34:56Z",
				"action": "upgrade",
				"status": "rolledback",
				"version": "1.0.5",
				"failure_reason": "bootloader",
				"failure_log": "test failure log string"
			}
		]
	}`
	invalidDetailJSON := "invalid json string format"

	invalidDetailJSON2 := `{
		"UpdateLog": [
			{
				"update_type": "OS",
				"package_name": "edge-microvisor-toolkit-image",
				"update_time": "2024-05-08T12:34:56Z",
				"action": "upgrade",
				"status": "rolledback",
				"version": "1.0.5",
				"failure_reason": "bootloader",
				"failure_log": "test failure log string"
			}
		]
	}`

	emptyDetailJSON := `{
		"update_log": []
	}`

	tests := []struct {
		name             string
		upDetail         string
		upStatus         inv_status.ResourceStatus
		wantStatusDetail string
		wantStatusMsg    string
		osType           os_v1.OsType
	}{
		{
			name:             "ReturnStatusDetailsFor1Package",
			upDetail:         validDetailJSON1,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: validDetailJSON1,
			wantStatusMsg:    "Update completed - 1 package(s) updated/installed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnStatusDetailsFor2Packages",
			upDetail:         validDetailJSON2,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: validDetailJSON2,
			wantStatusMsg:    "Update completed - 2 package(s) updated/installed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnStatusDetailsFor0Packages",
			upDetail:         emptyDetailJSON,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: emptyDetailJSON,
			wantStatusMsg:    "Update completed - 0 package(s) updated/installed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnEmptyStringAsJSONInvalid",
			upDetail:         invalidDetailJSON,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: "",
			wantStatusMsg:    "Update completed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnStatusDetailsForFailedUpdate",
			upDetail:         validDetailJSON1,
			upStatus:         mm_status.UpdateStatusFailed,
			wantStatusDetail: validDetailJSON1,
			wantStatusMsg:    "Update failed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnEmptyString",
			upDetail:         validDetailJSON1,
			upStatus:         mm_status.UpdateStatusInProgress,
			wantStatusDetail: "",
			wantStatusMsg:    "Updating",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnEmptyString",
			upDetail:         validDetailJSON1,
			upStatus:         mm_status.UpdateStatusUpToDate,
			wantStatusDetail: "",
			wantStatusMsg:    "No new updates available",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},

		{
			name:             "ReturnEmptyStringWhenJSONInvalid",
			upDetail:         invalidDetailJSON,
			upStatus:         mm_status.UpdateStatusFailed,
			wantStatusDetail: "",
			wantStatusMsg:    "Update failed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnEmptyStringWhenDetailsEmpty",
			upDetail:         "",
			upStatus:         mm_status.UpdateStatusFailed,
			wantStatusDetail: "",
			wantStatusMsg:    "Update failed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnStatusDetailsForRolledbackOS",
			upDetail:         validDetailJSON3,
			upStatus:         mm_status.UpdateStatusFailed,
			wantStatusDetail: validDetailJSON3,
			wantStatusMsg:    "Update failed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
		{
			name:             "ReturnStatusDetailsForImmutableOS",
			upDetail:         validDetailJSON2,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: validDetailJSON2,
			wantStatusMsg:    "Update completed",
			osType:           os_v1.OsType_OS_TYPE_IMMUTABLE,
		},
		{
			name:             "ReturnEmptyStringAsJSONInvalid",
			upDetail:         invalidDetailJSON2,
			upStatus:         mm_status.UpdateStatusDone,
			wantStatusDetail: "",
			wantStatusMsg:    "Update completed",
			osType:           os_v1.OsType_OS_TYPE_MUTABLE,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateStatus := &pb.UpdateStatus{StatusDetail: tt.upDetail}
			s := tt.upStatus
			statusDetailOut := util.GetUpdateStatusDetailIfNeeded(&s, updateStatus, tt.osType)
			assert.Equal(t, tt.wantStatusDetail, statusDetailOut)
			assert.Equal(t, tt.wantStatusMsg, s.Status)
		})
	}
}

//nolint:funlen // it's a test
func TestGetUpdatedUpdateStatusIfNeeded(t *testing.T) {
	type args struct {
		status                   *pb.UpdateStatus
		instanceStatusIndication statusv1.StatusIndication
		instanceStatusMessage    string
	}
	tests := []struct {
		name  string
		args  args
		want1 *inv_status.ResourceStatus
		want2 bool
	}{
		{
			name: "UpdateUnspecifiedToUpdating",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_STARTED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_UNSPECIFIED,
				instanceStatusMessage:    mm_status.StatusUnknown,
			},
			want1: &mm_status.UpdateStatusInProgress,
			want2: true,
		},
		{
			name: "NotUpdateToUpdating",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_STARTED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS,
				instanceStatusMessage:    mm_status.StatusUpdating,
			},
			want1: &mm_status.UpdateStatusInProgress,
			want2: false,
		},
		{
			name: "UpdateUpdatingToUpdated",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS,
				instanceStatusMessage:    mm_status.StatusUpdating,
			},
			want1: &mm_status.UpdateStatusDone,
			want2: true,
		},
		{
			name: "NotUpdateToUpdated",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UPDATED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusCompleted,
			},
			want1: &mm_status.UpdateStatusDone,
			want2: false,
		},
		{
			name: "UpdateToFailed",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_FAILED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IN_PROGRESS,
				instanceStatusMessage:    mm_status.StatusUpdating,
			},
			want1: &mm_status.UpdateStatusFailed,
			want2: true,
		},
		{
			name: "NoUpdateToFailed",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_FAILED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_ERROR,
				instanceStatusMessage:    mm_status.StatusFailed,
			},
			want1: &mm_status.UpdateStatusFailed,
			want2: false,
		},
		{
			name: "UpdateToUpToDate",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_UP_TO_DATE,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusCompleted,
			},
			want1: &mm_status.UpdateStatusUpToDate,
			want2: true,
		},
		{
			name: "UpdateToDownloading",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_DOWNLOADING,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusUpToDate,
			},
			want1: &mm_status.UpdateStatusDownloading,
			want2: true,
		},
		{
			name: "NoUpdateToDownloading",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_DOWNLOADING,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusDownloading,
			},
			want1: &mm_status.UpdateStatusDownloading,
			want2: false,
		},
		{
			name: "UpdateToDownloaded",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_DOWNLOADED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusDownloading,
			},
			want1: &mm_status.UpdateStatusDownloaded,
			want2: true,
		},
		{
			name: "NoUpdateToDownloaded",
			args: args{
				status: &pb.UpdateStatus{
					StatusType: pb.UpdateStatus_STATUS_TYPE_DOWNLOADED,
				},
				instanceStatusIndication: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
				instanceStatusMessage:    mm_status.StatusDownloaded,
			},
			want1: &mm_status.UpdateStatusDownloaded,
			want2: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newStatus, ifNeeded := util.GetUpdatedUpdateStatusIfNeeded(
				tt.args.status, tt.args.instanceStatusIndication, tt.args.instanceStatusMessage)
			assert.Equal(t, tt.want1, newStatus)
			assert.Equal(t, tt.want2, ifNeeded)
		})
	}
}

func TestPopulateOsProfileUpdateSource(t *testing.T) {
	immutableOS := &os_v1.OperatingSystemResource{
		OsType:         os_v1.OsType_OS_TYPE_IMMUTABLE,
		ProfileName:    "test-profile",
		ProfileVersion: "1.2.3",
		ImageUrl:       "http://example.com/image",
		ImageId:        "img-123",
		Sha256:         "sha256sum",
	}
	mutableOS := &os_v1.OperatingSystemResource{
		OsType: os_v1.OsType_OS_TYPE_MUTABLE,
	}
	t.Run("nil input", func(t *testing.T) {
		res, err := util.PopulateOsProfileUpdateSource(nil)
		assert.Nil(t, res)
		assert.Error(t, err)
	})
	t.Run("immutable os", func(t *testing.T) {
		res, err := util.PopulateOsProfileUpdateSource(immutableOS)
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.Equal(t, immutableOS.ProfileName, res.ProfileName)
		assert.Equal(t, immutableOS.ProfileVersion, res.ProfileVersion)
		assert.Equal(t, immutableOS.ImageUrl, res.OsImageUrl)
		assert.Equal(t, immutableOS.ImageId, res.OsImageId)
		assert.Equal(t, immutableOS.Sha256, res.OsImageSha)
	})
	t.Run("mutable os", func(t *testing.T) {
		res, err := util.PopulateOsProfileUpdateSource(mutableOS)
		assert.Nil(t, res)
		assert.Error(t, err)
	})
}

func TestPopulateOsProfileUpdateSource_Matrix(t *testing.T) {
	immutableOS := &os_v1.OperatingSystemResource{
		OsType:         os_v1.OsType_OS_TYPE_IMMUTABLE,
		ProfileName:    "test-profile",
		ProfileVersion: "1.2.3",
		ImageUrl:       "http://example.com/image",
		ImageId:        "img-123",
		Sha256:         "sha256sum",
	}
	mutableOS := &os_v1.OperatingSystemResource{
		OsType: os_v1.OsType_OS_TYPE_MUTABLE,
	}
	tests := []struct {
		name    string
		input   *os_v1.OperatingSystemResource
		want    *pb.OSProfileUpdateSource
		wantErr bool
	}{
		{
			name:    "nil input",
			input:   nil,
			want:    nil,
			wantErr: true,
		},
		{
			name:  "immutable os",
			input: immutableOS,
			want: &pb.OSProfileUpdateSource{
				ProfileName:    "test-profile",
				ProfileVersion: "1.2.3",
				OsImageUrl:     "http://example.com/image",
				OsImageId:      "img-123",
				OsImageSha:     "sha256sum",
			},
			wantErr: false,
		},
		{
			name:    "mutable os",
			input:   mutableOS,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateOsProfileUpdateSource(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestConvertToComparableSemVer(t *testing.T) {
	tests := []struct {
		input       string
		expected    string
		expectError bool
	}{
		// Valid image versions
		{"3.0.20250717.0732", "3.0.20250717-build0732", false},
		{"1.2.3.4.5", "1.2.3-4.5", false},
		{"10.0.1", "10.0.1", false},
		{"2025.07.11.0415", "2025.7.11-build0415", false}, // removed leading zeros in core + build in prerelease
		{"0.0.1", "0.0.1", false},

		// Invalid image versions
		{"", "", true},
		{"1", "", true},
		{"2.5.", "", true},
		{"a.b.c", "", true},
	}

	for _, tc := range tests {
		got, err := util.ConvertToComparableSemVer(tc.input)
		if tc.expectError {
			if err == nil {
				t.Errorf("ConvertToComparableSemVer(%q) expected error, got none", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ConvertToComparableSemVer(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.expected {
			t.Errorf("ConvertToComparableSemVer(%q) = %q; want %q", tc.input, got, tc.expected)
		}
		// Check if the result can be parsed as a semver
		if _, err := semver.NewVersion(got); err != nil {
			t.Errorf("semver.NewVersion(%q) failed: %v", got, err)
		}
	}
}

func TestConvertToComparableSemVer_Sorting(t *testing.T) {
	rawVersions := []string{
		"3.0.20250717.0732",
		"3.0.20250711.0415",
		"3.0.20250719.1000",
		"3.0.20240719.1000",
	}

	parsed := make([]*semver.Version, len(rawVersions))
	for i, rv := range rawVersions {
		vStr, err := util.ConvertToComparableSemVer(rv)
		if err != nil {
			t.Fatalf("convert error for %q: %v", rv, err)
		}
		v, err := semver.NewVersion(vStr)
		if err != nil {
			t.Fatalf("parse error for %q: %v", vStr, err)
		}
		parsed[i] = v
	}

	sort.Sort(semver.Collection(parsed))

	got := make([]string, len(parsed))
	for i, v := range parsed {
		got[i] = v.String()
	}

	expected := []string{
		"3.0.20240719-1000",
		"3.0.20250711-build0415",
		"3.0.20250717-build0732",
		"3.0.20250719-1000",
	}

	for i := range expected {
		if got[i] != expected[i] {
			t.Errorf("sorting mismatch at %d: got %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestCompareImageVersions(t *testing.T) {
	tests := []struct {
		name     string
		version1 string
		version2 string
		expected bool
	}{
		{
			name:     "version with many leading zeros",
			version1: "3.0.20250717.0001",
			version2: "3.0.20250717.0000",
			expected: true,
		},
		{
			name:     "versions identical except for leading zeros",
			version1: "3.0.20250717.0",
			version2: "3.0.20250717.0000",
			expected: false, // Should be treated as equal
		},
		{
			name:     "very long build numbers",
			version1: "3.0.20250717.123456789",
			version2: "3.0.20250717.123456788",
			expected: true,
		},
		{
			name:     "version1 has higher patch version",
			version1: "3.0.20250817.1234",
			version2: "3.0.20250717.1234",
			expected: true,
		},
		{
			name:     "version1 has higher minor version",
			version1: "3.1.20250717.1234",
			version2: "3.0.20250717.1234",
			expected: true,
		},
		{
			name:     "version1 has higher minor version",
			version1: "4.0.20250717.1234",
			version2: "3.0.20250717.1234",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := util.CompareImageVersions(tt.version1, tt.version2)
			assert.Equal(t, tt.expected, result,
				"CompareImageVersions(%q, %q) = %v, expected %v",
				tt.version1, tt.version2, result, tt.expected)
		})
	}
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package testing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	location_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/location/v1"
	schedule_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/schedule/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	pb "github.com/open-edge-platform/infra-managers/maintenance/pkg/api/maintmgr/v1"
)

// Contains helper functions and variables to be shared across different test packages of maintenance manager

const (
	SafeTimeDelay  = 600
	DelayStart5    = 5
	DelayStart10   = 10
	DelayStart20   = 20
	DelayStart50   = 50
	DurationTest   = 10
	TestMemory     = 64 * util.Gigabyte
	TestCPUCores   = 14
	TestCPUThreads = 10
)

const (
	Tenant1 = "11111111-1111-1111-1111-111111111111"
	Tenant2 = "22222222-2222-2222-2222-222222222222"
	// DefaultTenantID is the default tenantID set when no TenantID is provided in SBI.
	DefaultTenantID = "10000000-0000-0000-0000-000000000000"
)

var (
	TimeNow            = uint64(time.Now().UTC().Unix())
	SingleSchedulePast = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow - SafeTimeDelay - DelayStart20,
		EndSeconds:     TimeNow - SafeTimeDelay,
	}
	SingleSchedulePastContinuing = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow - SafeTimeDelay - DelayStart20,
	}
	SingleSchedule0 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart10,
	}
	MmSingleSchedule0 = pb.SingleSchedule{
		StartSeconds: TimeNow + SafeTimeDelay + DelayStart10,
	}
	SingleSchedule1 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart20,
	}
	SingleSchedule2 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart50,
	}
	SingleSchedule3 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart5 + DelayStart50,
		EndSeconds:     TimeNow + SafeTimeDelay + DelayStart50 + DelayStart50 + DelayStart5,
	}
	MmSingleSchedule3 = pb.SingleSchedule{
		StartSeconds: TimeNow + SafeTimeDelay + DelayStart5 + DelayStart50,
		EndSeconds:   TimeNow + SafeTimeDelay + DelayStart50 + DelayStart50 + DelayStart5,
	}
	SingleSchedule4 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_MAINTENANCE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart10,
	}
	SingleSchedule5 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_SHIPPING,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart10,
	}
	SingleSchedule6 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_CLUSTER_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart10,
	}
	SingleSchedule7 = schedule_v1.SingleScheduleResource{
		TenantId:       Tenant1,
		ScheduleStatus: schedule_v1.ScheduleStatus_SCHEDULE_STATUS_FIRMWARE_UPDATE,
		StartSeconds:   TimeNow + SafeTimeDelay + DelayStart10,
	}
	RepeatedSchedule1 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "3",
		CronHours:       "4",
		CronDayMonth:    "5",
		CronMonth:       "6",
		CronDayWeek:     "0",
	}
	MmRepeatedSchedule1 = []*pb.RepeatedSchedule{
		{
			DurationSeconds: uint32(DurationTest),
			CronMinutes:     "3",
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	RepeatedSchedule2 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_OS_UPDATE,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "*",
		CronHours:       "*",
		CronDayMonth:    "*",
		CronMonth:       "*",
		CronDayWeek:     "*",
	}
	MmRepeatedSchedule2 = []*pb.RepeatedSchedule{
		{
			DurationSeconds: uint32(DurationTest),
			CronMinutes:     "*",
			CronHours:       "*",
			CronDayMonth:    "*",
			CronMonth:       "*",
			CronDayWeek:     "*",
		},
		{
			DurationSeconds: uint32(DurationTest),
			CronMinutes:     "3",
			CronHours:       "4",
			CronDayMonth:    "5",
			CronMonth:       "6",
			CronDayWeek:     "0",
		},
	}
	RepeatedSchedule3 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_MAINTENANCE,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "*",
		CronHours:       "*",
		CronDayMonth:    "*",
		CronMonth:       "*",
		CronDayWeek:     "*",
	}
	RepeatedSchedule4 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_SHIPPING,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "*",
		CronHours:       "*",
		CronDayMonth:    "*",
		CronMonth:       "*",
		CronDayWeek:     "*",
	}
	RepeatedSchedule5 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_FIRMWARE_UPDATE,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "*",
		CronHours:       "*",
		CronDayMonth:    "*",
		CronMonth:       "*",
		CronDayWeek:     "*",
	}
	RepeatedSchedule6 = schedule_v1.RepeatedScheduleResource{
		TenantId:        Tenant1,
		Name:            "repeatedSchedule test 1",
		ScheduleStatus:  schedule_v1.ScheduleStatus_SCHEDULE_STATUS_CLUSTER_UPDATE,
		DurationSeconds: uint32(DurationTest),
		CronMinutes:     "*",
		CronHours:       "*",
		CronDayMonth:    "*",
		CronMonth:       "*",
		CronDayWeek:     "*",
	}

	HostResource1 = computev1.HostResource{
		TenantId: Tenant1,
		Name:     "for unit testing purposes",

		HardwareKind: "XDgen2",
		SerialNumber: "12345678",
		MemoryBytes:  TestMemory,

		CpuModel:        "12th Gen Intel(R) Core(TM) i9-12900",
		CpuSockets:      1,
		CpuCores:        TestCPUCores,
		CpuCapabilities: "",
		CpuArchitecture: "x86_64",
		CpuThreads:      TestCPUThreads,

		MgmtIp: "192.168.10.10",

		BmcKind:     computev1.BaremetalControllerKind_BAREMETAL_CONTROLLER_KIND_PDU,
		BmcIp:       "10.0.0.10",
		BmcUsername: "user",
		BmcPassword: "pass",
		PxeMac:      "90:49:fa:ff:ff:ff",

		Hostname: "testhost1",
	}
)

func CreateHost(t *testing.T, tenantID string, host *computev1.HostResource) *computev1.HostResource {
	t.Helper()
	dao := inv_testing.NewInvResourceDAOOrFail(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient().Create(ctx,
		tenantID,
		&inv_v1.Resource{
			Resource: &inv_v1.Resource_Host{Host: host},
		})
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(resp)
	require.NoError(t, err)
	host.ResourceId = rID
	t.Cleanup(func() { dao.HardDeleteHost(t, tenantID, rID) })

	return host
}

//nolint:dupl // Helper testing function very similar to CreateAndBindRepeatedSchedule, but kept separate for easier consumption
func CreateAndBindSingleSchedule(
	t *testing.T,
	tenantID string,
	sSched *schedule_v1.SingleScheduleResource,
	host *computev1.HostResource,
	site *location_v1.SiteResource,
) {
	t.Helper()
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	switch {
	case host != nil:
		sSched.Relation = &schedule_v1.SingleScheduleResource_TargetHost{
			TargetHost: host,
		}
	case site != nil:
		sSched.Relation = &schedule_v1.SingleScheduleResource_TargetSite{
			TargetSite: site,
		}
	default:
		sSched.Relation = nil
	}
	resp, err := client.Create(ctx, tenantID, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Singleschedule{
			Singleschedule: sSched,
		},
	})
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(resp)
	require.NoError(t, err)
	sSched.ResourceId = rID
	t.Cleanup(func() { dao.DeleteResource(t, tenantID, rID) })
}

//nolint:dupl // Helper testing function very similar to CreateAndBindSingleSchedule, but kept separate for easier consumption
func CreateAndBindRepeatedSchedule(
	t *testing.T,
	tenantID string,
	rSched *schedule_v1.RepeatedScheduleResource,
	host *computev1.HostResource,
	site *location_v1.SiteResource,
) {
	t.Helper()
	dao := inv_testing.NewInvResourceDAOOrFail(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	client := inv_testing.TestClients[inv_testing.RMClient].GetTenantAwareInventoryClient()
	switch {
	case host != nil:
		rSched.Relation = &schedule_v1.RepeatedScheduleResource_TargetHost{
			TargetHost: host,
		}
	case site != nil:
		rSched.Relation = &schedule_v1.RepeatedScheduleResource_TargetSite{
			TargetSite: site,
		}
	default:
		rSched.Relation = nil
	}
	resp, err := client.Create(ctx, tenantID, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Repeatedschedule{
			Repeatedschedule: rSched,
		},
	})
	require.NoError(t, err)
	_, rID, err := util.GetResourceKeyFromResource(resp)
	require.NoError(t, err)
	rSched.ResourceId = rID
	t.Cleanup(func() { dao.DeleteResource(t, tenantID, rID) })
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package test_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	test_utils "github.com/open-edge-platform/infra-managers/host/test/utils"
)

const (
	TestGUID = "BFD3B398-9A4B-480D-AB53-4050ED108F5D"
	TestSN   = "12345678"
	tenant1  = "11111111-1111-1111-1111-111111111111"
	tenant2  = "22222222-2222-2222-2222-222222222222"
)

var (
	HostManagerTestClient     pb.HostmgrClient
	HostManagerTestClientConn *grpc.ClientConn
	zlog                      = logging.GetLogger("Host-Manager-Validation-Test")

	hwInfo = &pb.HWInfo{
		Cpu: &pb.SystemCPU{
			Arch:    "x86",
			Vendor:  "GenuineIntel",
			Model:   "12th Gen Intel(R) Core(TM) i9-12900H",
			Sockets: 1,
			Cores:   14,
			Threads: 20,
		},
		Memory: &pb.SystemMemory{
			Size: 30965874138,
		},
		Network: []*pb.SystemNetwork{
			{
				Name:         "eth0",
				PciId:        "0000:00:1f.6",
				Mac:          "90:49:fa:07:6c:fd",
				Sriovenabled: false,
				IpAddresses: []*pb.IPAddress{
					{
						IpAddress:         "192.168.0.11",
						NetworkPrefixBits: 24,
						ConfigMode:        1,
					},
					{
						IpAddress:         "2345:0425:2CA1::0567:5673:23b5",
						NetworkPrefixBits: 64,
						ConfigMode:        0,
					},
				},
				Mtu:    1500,
				BmcNet: false,
			},
			{
				Name:          "ens3",
				PciId:         "0000:00:1f.7",
				Mac:           "90:49:fa:07:6c:ff",
				Sriovenabled:  true,
				Sriovnumvfs:   8,
				SriovVfsTotal: 128,
				IpAddresses: []*pb.IPAddress{
					{
						IpAddress:         "192.168.0.12",
						NetworkPrefixBits: 24,
						ConfigMode:        1,
					},
					{
						IpAddress:         "2345:0425:2CA1::0567:5673:2231",
						NetworkPrefixBits: 64,
						ConfigMode:        0,
					},
				},
				Mtu:    1500,
				BmcNet: false,
			},
		},
		Storage: &pb.Storage{
			Disk: []*pb.SystemDisk{
				{
					SerialNumber: "21101S803132",
					Name:         "nvme0n1",
					Vendor:       "unknown",
					Model:        "WDS500G3X0C-00SJG0",
					Size:         500107862016,
				},
			},
		},
		Usb: []*pb.SystemUSB{
			{
				Class:      "aaaa",
				Idvendor:   "1233",
				Idproduct:  "2233",
				Bus:        0,
				Addr:       1,
				Serial:     "aaaabbbbcc",
				Interfaces: nil,
			},
		},
		Gpu: []*pb.SystemGPU{
			{
				PciId:       "0000:00:dd.1",
				Product:     "some product name",
				Vendor:      "Intel",
				Description: "GPU card - Model XYZ",
			},
			{
				PciId:       "0000:00:dd.2",
				Product:     "some product name",
				Vendor:      "Intel",
				Description: "GPU card - Model XYZ",
			},
		},
	}
)

// validation_test.go runs basic workflows from the gRPC client perspective (bare-metal agents).
// It is used to validate basic scenarios for communication between agents and HRM.
func TestMain(m *testing.M) {
	cli, conn, err := test_utils.CreateHostManagerClient("localhost:50001", nil)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Cannot create Host Manager client")
	}
	HostManagerTestClient = cli
	HostManagerTestClientConn = conn

	run := m.Run() // run all tests

	if err := HostManagerTestClientConn.Close(); err != nil {
		zlog.Warn().Err(err).Msg("Failed to close host manager test client")
	}
	os.Exit(run)
}

func TestHeartbeatAndHWInfoUpdate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	hostInv := inv_testing.CreateHost(t, nil, nil)
	TestGUID := hostInv.GetUuid()

	inUp := &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid: TestGUID,
		HostStatus: &pb.HostStatus{
			HostStatus: pb.HostStatus_RUNNING,
		},
	}
	respUp, err := HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.NoError(t, err)
	require.NotNil(t, respUp)

	cancel()
	t.Log("Waiting for host heartbeat timeout..")
	// wait for host heartbeat timeout
	time.Sleep(50 * time.Second)

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// refresh status
	inUp.HostStatus.HostStatus = pb.HostStatus_PROVISIONED
	respUp, err = HostManagerTestClient.UpdateHostStatusByHostGuid(ctx, inUp)
	require.NoError(t, err)
	require.NotNil(t, respUp)

	cancel()

	time.Sleep(10 * time.Second)

	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// update BIOS info with new API
	upSystemInfo := &pb.UpdateHostSystemInfoByGUIDRequest{
		HostGuid: TestGUID,
		SystemInfo: &pb.SystemInfo{
			HwInfo: hwInfo,
			BiosInfo: &pb.BiosInfo{
				Version:     "1.0.0",
				ReleaseDate: "",
				Vendor:      "Dell Inc.",
			},
		},
	}
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, upSystemInfo)
	require.NoError(t, err)

	// add a new USB device
	upSystemInfo.SystemInfo.HwInfo.Usb = append(upSystemInfo.SystemInfo.HwInfo.Usb, &pb.SystemUSB{
		Class:     "bbbb",
		Idvendor:  "999",
		Idproduct: "6666",
	})
	_, err = HostManagerTestClient.UpdateHostSystemInfoByGUID(ctx, upSystemInfo)
	require.NoError(t, err)
}

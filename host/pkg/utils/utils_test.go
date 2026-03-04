// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package util_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	statusv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/status/v1"
	inv_testing "github.com/open-edge-platform/infra-core/inventory/v2/pkg/testing"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
	util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
)

var (
	hostStorageResource = &computev1.HoststorageResource{
		Serial:        "diskSerialNumber",
		DeviceName:    "diskName",
		Vendor:        "diskVendor",
		Model:         "diskModel",
		CapacityBytes: 64,
	}
	hostNicResource = &computev1.HostnicResource{
		DeviceName:          "nicName",
		PciIdentifier:       "nicPciId",
		MacAddr:             "nicMac",
		SriovEnabled:        true,
		SriovVfsNum:         1,
		SriovVfsTotal:       2,
		PeerName:            "nicPeerName",
		PeerDescription:     "nicPeerDescription",
		PeerMac:             "nicPeerMac",
		PeerMgmtIp:          "nicPeerMgmtIp",
		PeerPort:            "nicPeerPort",
		SupportedLinkMode:   "linkMode1,linkMode2,linkMode3",
		AdvertisingLinkMode: "linkMode1,LinkMode3",
		CurrentSpeedBps:     21,
		CurrentDuplex:       "nicCurrentDuplex",
		Features:            "feature1,feature2,feature3",
		Mtu:                 1,
		LinkState:           computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
		BmcInterface:        true,
	}
	hostUsbResource = &computev1.HostusbResource{
		Idvendor:   "usbIdvendor",
		Idproduct:  "usbIdproduct",
		Bus:        0,
		Addr:       0,
		Class:      "usbClass",
		DeviceName: "usbDescription",
		Serial:     "usbSerial",
	}
	hostGpuResource = &computev1.HostgpuResource{
		PciId:       "gpuPciId",
		Product:     "gpuProductName",
		Vendor:      "gpuVendor",
		Description: "some desc",
		DeviceName:  "gpu0",
	}
	disk = &pb.SystemDisk{
		SerialNumber: "diskSerialNumber",
		Name:         "diskName",
		Vendor:       "diskVendor",
		Model:        "diskModel",
		Size:         64,
	}
	nic = &pb.SystemNetwork{
		Name:                "nicName",
		PciId:               "nicPciId",
		Mac:                 "nicMac",
		Sriovenabled:        true,
		Sriovnumvfs:         1,
		SriovVfsTotal:       2,
		PeerName:            "nicPeerName",
		PeerDescription:     "nicPeerDescription",
		PeerMac:             "nicPeerMac",
		PeerMgmtIp:          "nicPeerMgmtIp",
		PeerPort:            "nicPeerPort",
		SupportedLinkMode:   []string{"linkMode1", "linkMode2", "linkMode3"},
		AdvertisingLinkMode: []string{"linkMode1", "LinkMode3"},
		CurrentSpeed:        21,
		CurrentDuplex:       "nicCurrentDuplex",
		Features:            []string{"feature1", "feature2", "feature3"},
		Mtu:                 1,
		LinkState:           true,
		BmcNet:              true,
	}
	usb = &pb.SystemUSB{
		Idvendor:    "usbIdvendor",
		Idproduct:   "usbIdproduct",
		Bus:         0,
		Addr:        0,
		Class:       "usbClass",
		Description: "usbDescription",
		Serial:      "usbSerial",
	}
	gpu = &pb.SystemGPU{
		Name:        "gpu0",
		PciId:       "gpuPciId",
		Product:     "gpuProductName",
		Vendor:      "gpuVendor",
		Description: "some desc",
		Features:    []string{"abc", "xyz", "q"},
	}
)

func TestPopulateHostusbWithUsbInfo(t *testing.T) {
	host := &computev1.HostResource{}
	type args struct {
		usb  *pb.SystemUSB
		host *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want *computev1.HostusbResource
		fail bool
	}{
		{
			name: "succ1",
			args: args{
				usb:  &pb.SystemUSB{},
				host: host,
			},
			want: &computev1.HostusbResource{
				Host: host,
			},
			fail: false,
		},
		{
			name: "succ2",
			args: args{
				usb: &pb.SystemUSB{
					Bus:      1,
					Addr:     1,
					Class:    "HighFooBar",
					Idvendor: "Foo",
				},
				host: host,
			},
			want: &computev1.HostusbResource{
				Bus:      1,
				Addr:     1,
				Class:    "HighFooBar",
				Idvendor: "Foo",
				Host:     host,
			},
			fail: false,
		},
		{
			name: "Fail_NoUsb",
			args: args{
				host: host,
			},
			want: &computev1.HostusbResource{
				Bus:      1,
				Addr:     1,
				Class:    "HighFooBar",
				Idvendor: "Foo",
				Host:     host,
			},
			fail: true,
		},
		{
			name: "Fail_NoHost",
			args: args{
				usb: &pb.SystemUSB{},
			},
			want: &computev1.HostusbResource{
				Bus:      1,
				Addr:     1,
				Class:    "HighFooBar",
				Idvendor: "Foo",
				Host:     host,
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateHostusbWithUsbInfo(tt.args.usb, tt.args.host)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateHostusbWithUsbInfo() should fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, got); !eq {
				t.Errorf("PopulateHostusbWithUsbInfo() data not equal: %v", diff)
			}
		})
	}
}

//nolint:funlen // this is a table-driven test
func TestPopulateHostResourceWithNewSystemInfo(t *testing.T) {
	type args struct {
		info *pb.SystemInfo
	}
	tests := []struct {
		name string
		args args
		want *computev1.HostResource
		fail bool
	}{
		{
			name: "Success",
			args: args{
				&pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum:   "test",
						ProductName: "test-product",
						Memory: &pb.SystemMemory{
							Size: 1,
						},
						Cpu: &pb.SystemCPU{
							Sockets:  1,
							Arch:     "x86",
							Model:    "Intel Core i9-14900K",
							Cores:    24,
							Threads:  32,
							Features: []string{"capability1", "capability2", "capability3"},
						},
						Gpu: []*pb.SystemGPU{
							{
								Name:        "gpu0",
								PciId:       "00:01",
								Product:     "XYZ",
								Vendor:      "Intel",
								Description: "some desc",
							},
						},
					},
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: false,
		},
		{
			name: "ResetGPU_Success",
			args: args{
				&pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum:   "test",
						ProductName: "test-product",
						Memory: &pb.SystemMemory{
							Size: 1,
						},
						Cpu: &pb.SystemCPU{
							Sockets:  1,
							Arch:     "x86",
							Model:    "Intel Core i9-14900K",
							Cores:    24,
							Threads:  32,
							Features: []string{"capability1", "capability2", "capability3"},
						},
					},
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: false,
		},
		{
			name: "NoCPU_Success",
			args: args{
				&pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum:   "test",
						ProductName: "test-product",
						Memory: &pb.SystemMemory{
							Size: 1,
						},
						Gpu: []*pb.SystemGPU{
							{
								Name:        "gpu0",
								PciId:       "00:01",
								Product:     "XYZ",
								Vendor:      "Intel",
								Description: "some desc",
							},
						},
					},
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: false,
		},
		{
			name: "NoMemory_Success",
			args: args{
				&pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum:   "test",
						ProductName: "test-product",
						Cpu: &pb.SystemCPU{
							Sockets:  1,
							Arch:     "x86",
							Model:    "Intel Core i9-14900K",
							Cores:    24,
							Threads:  32,
							Features: []string{"capability1", "capability2", "capability3"},
						},
						Gpu: []*pb.SystemGPU{
							{
								Name:        "gpu0",
								PciId:       "00:01",
								Product:     "XYZ",
								Vendor:      "Intel",
								Description: "some desc",
							},
						},
					},
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: false,
		},
		{
			name: "NoBIOS_Success",
			args: args{
				&pb.SystemInfo{
					HwInfo: &pb.HWInfo{
						SerialNum:   "test",
						ProductName: "test-product",
						Memory: &pb.SystemMemory{
							Size: 1,
						},
						Cpu: &pb.SystemCPU{
							Sockets:  1,
							Arch:     "x86",
							Model:    "Intel Core i9-14900K",
							Cores:    24,
							Threads:  32,
							Features: []string{"capability1", "capability2", "capability3"},
						},
						Gpu: []*pb.SystemGPU{
							{
								Name:        "gpu0",
								PciId:       "00:01",
								Product:     "XYZ",
								Vendor:      "Intel",
								Description: "some desc",
							},
						},
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
			},
			fail: false,
		},
		{
			name: "NoHWInfo_Success",
			args: args{
				&pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: false,
		},
		{
			name: "NoHWInfo_Fail",
			args: args{
				&pb.SystemInfo{
					BiosInfo: &pb.BiosInfo{
						Version:     "1.0.0",
						ReleaseDate: "12/09/2022",
						Vendor:      "Dell Inc.",
					},
				},
			},
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			fail: true,
		},
		{
			name: "Failed_NoSystemInfo",
			args: args{
				nil,
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updatedHost, _, err := util.PopulateHostResourceWithNewSystemInfo(tt.args.info)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateHostResourceWithNewSystemInfo() should NOT fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, updatedHost); !eq && !tt.fail {
				t.Errorf("PopulateHostResourceWithNewSystemInfo() data not equal, but should be: %v", diff)
			}
		})
	}
}

func TestPopulateHoststorageWithDiskInfo(t *testing.T) {
	host := &computev1.HostResource{}
	type args struct {
		disk *pb.SystemDisk
		host *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want *computev1.HoststorageResource
		fail bool
	}{
		{
			name: "succ1",
			args: args{
				disk: &pb.SystemDisk{},
				host: host,
			},
			want: &computev1.HoststorageResource{
				Host: host,
			},
			fail: false,
		},
		{
			name: "succ2",
			args: args{
				disk: &pb.SystemDisk{
					Name:   "sda1",
					Vendor: "FooVendor",
					Model:  "BarModel",
					Size:   1,
					Wwid:   "0x1234567890",
				},
				host: host,
			},
			want: &computev1.HoststorageResource{
				DeviceName:    "sda1",
				Vendor:        "FooVendor",
				Model:         "BarModel",
				CapacityBytes: 1,
				Host:          host,
				Wwid:          "0x1234567890",
			},
			fail: false,
		},
		{
			name: "Fail_NoStorage",
			args: args{
				host: host,
			},
			want: &computev1.HoststorageResource{
				DeviceName:    "sda1",
				Vendor:        "FooVendor",
				Model:         "BarModel",
				CapacityBytes: 1,
			},
			fail: true,
		},
		{
			name: "Fail_NoDisk",
			args: args{
				disk: &pb.SystemDisk{
					Name:   "sda1",
					Vendor: "FooVendor",
					Model:  "BarModel",
					Size:   1,
				},
			},
			want: &computev1.HoststorageResource{
				DeviceName:    "sda1",
				Vendor:        "FooVendor",
				Model:         "BarModel",
				CapacityBytes: 1,
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateHoststorageWithDiskInfo(tt.args.disk, tt.args.host)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateHoststorageWithDiskInfo() should fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, got); !eq {
				t.Errorf("PopulateHoststorageWithDiskInfo() data not equal: %v", diff)
			}
		})
	}
}

func TestPopulateHostnicWithNetworkInfo(t *testing.T) { //nolint:funlen // it is a table-driven test
	host := &computev1.HostResource{}
	type args struct {
		nic  *pb.SystemNetwork
		host *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want *computev1.HostnicResource
		fail bool
	}{
		{
			name: "succ1",
			args: args{
				nic: &pb.SystemNetwork{
					Name:                "eth0",
					PciId:               "0000:b1:00.0",
					Mac:                 "ee:ee:ee:ee:ee:ee",
					LinkState:           false,
					CurrentSpeed:        10000,
					CurrentDuplex:       "",
					SupportedLinkMode:   []string{""},
					AdvertisingLinkMode: []string{""},
					Features:            []string{""},
					Sriovenabled:        true,
					Sriovnumvfs:         8,
					SriovVfsTotal:       128,
					PeerName:            "test_peer",
					PeerDescription:     "some desc",
					PeerMac:             "ee:ee:ee:ee:ee:ff",
					PeerMgmtIp:          "10.0.0.1",
					PeerPort:            "80",
					IpAddresses:         []*pb.IPAddress{},
					Mtu:                 1500,
					BmcNet:              false,
				},
				host: host,
			},
			want: &computev1.HostnicResource{
				Host:                host,
				DeviceName:          "eth0",
				MacAddr:             "ee:ee:ee:ee:ee:ee",
				PciIdentifier:       "0000:b1:00.0",
				SriovEnabled:        true,
				SriovVfsNum:         8,
				SriovVfsTotal:       128,
				PeerName:            "test_peer",
				PeerDescription:     "some desc",
				PeerMac:             "ee:ee:ee:ee:ee:ff",
				PeerMgmtIp:          "10.0.0.1",
				PeerPort:            "80",
				SupportedLinkMode:   "",
				AdvertisingLinkMode: "",
				CurrentSpeedBps:     10000,
				CurrentDuplex:       "",
				Features:            "",
				Mtu:                 1500,
				LinkState:           computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_DOWN,
				BmcInterface:        false,
			},
			fail: false,
		},
		{
			name: "succ2",
			args: args{
				nic: &pb.SystemNetwork{
					Name:                "eth1",
					PciId:               "0000:b1:00.0",
					Mac:                 "ee:ee:ee:ee:ee:ee",
					LinkState:           true,
					CurrentSpeed:        10000,
					CurrentDuplex:       "",
					SupportedLinkMode:   []string{""},
					AdvertisingLinkMode: []string{""},
					Features:            []string{""},
					Sriovenabled:        true,
					Sriovnumvfs:         8,
					SriovVfsTotal:       128,
					PeerName:            "test_peer",
					PeerDescription:     "some desc",
					PeerMac:             "ee:ee:ee:ee:ee:ff",
					PeerMgmtIp:          "10.0.0.1",
					PeerPort:            "80",
					IpAddresses:         []*pb.IPAddress{},
					Mtu:                 1500,
					BmcNet:              false,
				},
				host: host,
			},
			want: &computev1.HostnicResource{
				Host:                host,
				DeviceName:          "eth1",
				MacAddr:             "ee:ee:ee:ee:ee:ee",
				PciIdentifier:       "0000:b1:00.0",
				SriovEnabled:        true,
				SriovVfsNum:         8,
				SriovVfsTotal:       128,
				PeerName:            "test_peer",
				PeerDescription:     "some desc",
				PeerMac:             "ee:ee:ee:ee:ee:ff",
				PeerMgmtIp:          "10.0.0.1",
				PeerPort:            "80",
				SupportedLinkMode:   "",
				AdvertisingLinkMode: "",
				CurrentSpeedBps:     10000,
				CurrentDuplex:       "",
				Features:            "",
				Mtu:                 1500,
				LinkState:           computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
				BmcInterface:        false,
			},
			fail: false,
		},
		{
			name: "Fail_NoHost",
			args: args{
				nic: &pb.SystemNetwork{
					Name:                "eth1",
					PciId:               "0000:b1:00.0",
					Mac:                 "ee:ee:ee:ee:ee:ee",
					LinkState:           true,
					CurrentSpeed:        10000,
					CurrentDuplex:       "",
					SupportedLinkMode:   []string{""},
					AdvertisingLinkMode: []string{""},
					Features:            []string{""},
					Sriovenabled:        true,
					Sriovnumvfs:         8,
					SriovVfsTotal:       128,
					PeerName:            "test_peer",
					PeerDescription:     "some desc",
					PeerMac:             "ee:ee:ee:ee:ee:ff",
					PeerMgmtIp:          "10.0.0.1",
					PeerPort:            "80",
					IpAddresses:         []*pb.IPAddress{},
					Mtu:                 1500,
					BmcNet:              false,
				},
			},
			want: &computev1.HostnicResource{
				Host:                host,
				DeviceName:          "eth1",
				MacAddr:             "ee:ee:ee:ee:ee:ee",
				PciIdentifier:       "0000:b1:00.0",
				SriovEnabled:        true,
				SriovVfsNum:         8,
				SriovVfsTotal:       128,
				PeerName:            "test_peer",
				PeerDescription:     "some desc",
				PeerMac:             "ee:ee:ee:ee:ee:ff",
				PeerMgmtIp:          "10.0.0.1",
				PeerPort:            "80",
				SupportedLinkMode:   "",
				AdvertisingLinkMode: "",
				CurrentSpeedBps:     10000,
				CurrentDuplex:       "",
				Features:            "",
				Mtu:                 1500,
				LinkState:           computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
				BmcInterface:        false,
			},
			fail: true,
		},
		{
			name: "Fail_NoNic",
			args: args{
				host: host,
			},
			want: &computev1.HostnicResource{
				Host:                host,
				DeviceName:          "eth1",
				MacAddr:             "ee:ee:ee:ee:ee:ee",
				PciIdentifier:       "0000:b1:00.0",
				SriovEnabled:        true,
				SriovVfsNum:         8,
				SriovVfsTotal:       128,
				PeerName:            "test_peer",
				PeerDescription:     "some desc",
				PeerMac:             "ee:ee:ee:ee:ee:ff",
				PeerMgmtIp:          "10.0.0.1",
				PeerPort:            "80",
				SupportedLinkMode:   "",
				AdvertisingLinkMode: "",
				CurrentSpeedBps:     10000,
				CurrentDuplex:       "",
				Features:            "",
				Mtu:                 1500,
				LinkState:           computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP,
				BmcInterface:        false,
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateHostnicWithNetworkInfo(tt.args.nic, tt.args.host)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateHostnicWithNetworkInfo() should fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, got); !eq {
				t.Errorf("PopulateHostnicWithNetworkInfo() data not equal: %v", diff)
			}
		})
	}
}

func TestPopulateHostgpuWithGpuInfo(t *testing.T) {
	host := &computev1.HostResource{}
	type args struct {
		gpu  *pb.SystemGPU
		host *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want *computev1.HostgpuResource
		fail bool
	}{
		{
			name: "Success",
			args: args{
				gpu:  gpu,
				host: host,
			},
			want: &computev1.HostgpuResource{
				Host:        host,
				PciId:       "gpuPciId",
				Product:     "gpuProductName",
				Vendor:      "gpuVendor",
				Description: "some desc",
				DeviceName:  "gpu0",
				Features:    "abc,xyz,q",
			},
			fail: false,
		},
		{
			name: "Fail_NoHost",
			args: args{
				gpu: &pb.SystemGPU{
					Name:  "gpu0",
					PciId: "0000:b1:00.0",
				},
			},
			fail: true,
		},
		{
			name: "Fail_NoGPU",
			args: args{
				host: host,
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateHostgpuWithGpuInfo(tt.args.gpu, tt.args.host)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateHostgpuWithGpuInfo() should fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, got); !eq {
				t.Errorf("PopulateHostgpuWithGpuInfo() data not equal: %v", diff)
			}
		})
	}
}

func TestPopulateIPAddressWithIPAddressInfo(t *testing.T) { //nolint:funlen // it is a table-driven test
	hostNic := &computev1.HostnicResource{}
	type args struct {
		ip      *pb.IPAddress
		hostNic *computev1.HostnicResource
	}
	tests := []struct {
		name string
		args args
		want *network_v1.IPAddressResource
		fail bool
	}{
		{
			name: "succ1",
			args: args{
				ip: &pb.IPAddress{
					IpAddress:         "10.0.0.1",
					NetworkPrefixBits: 32,
					ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
				},
				hostNic: hostNic,
			},
			want: &network_v1.IPAddressResource{
				Nic:          hostNic,
				Address:      "10.0.0.1/32",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
			fail: false,
		},
		{
			name: "succ2",
			args: args{
				ip: &pb.IPAddress{
					IpAddress:         "10.0.0.1",
					NetworkPrefixBits: 32,
					ConfigMode:        3,
				},
				hostNic: hostNic,
			},
			want: &network_v1.IPAddressResource{
				Nic:          hostNic,
				Address:      "10.0.0.1/32",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_UNSPECIFIED,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
			fail: false,
		},
		{
			name: "succ3",
			args: args{
				ip: &pb.IPAddress{
					IpAddress:         "fe80::0204:61ff:fe9d:f156",
					NetworkPrefixBits: 32,
					ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
				},
				hostNic: hostNic,
			},
			want: &network_v1.IPAddressResource{
				Nic:          hostNic,
				Address:      "fe80::0204:61ff:fe9d:f156/32",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
			fail: false,
		},
		{
			name: "Fail_NoIP",
			args: args{
				hostNic: hostNic,
			},
			want: &network_v1.IPAddressResource{
				Nic:          hostNic,
				Address:      "fe80::0204:61ff:fe9d:f156/32",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
			fail: true,
		},
		{
			name: "Fail_NoNic",
			args: args{
				ip: &pb.IPAddress{
					IpAddress:         "fe80::0204:61ff:fe9d:f156",
					NetworkPrefixBits: 32,
					ConfigMode:        pb.ConfigMode_CONFIG_MODE_DYNAMIC,
				},
			},
			want: &network_v1.IPAddressResource{
				Nic:          hostNic,
				Address:      "fe80::0204:61ff:fe9d:f156/32",
				CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
				ConfigMethod: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
				Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
				StatusDetail: "IPAddress is configured",
			},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.PopulateIPAddressWithIPAddressInfo(tt.args.ip, tt.args.hostNic)
			if err != nil {
				if !tt.fail {
					t.Errorf("PopulateIPAddressWithIPAddressInfo() should fail %s", err)
					t.FailNow()
				}
				return
			}
			if eq, diff := inv_testing.ProtoEqualOrDiff(tt.want, got); !eq {
				t.Errorf("PopulateIPAddressWithIPAddressInfo() data not equal: %v", diff)
			}
		})
	}
}

func TestIsHostUnderMaintain(t *testing.T) {
	type args struct {
		hostres *computev1.HostResource
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Success",
			args: args{&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					UpdateStatusIndicator: mm_status.UpdateStatusInProgress.StatusIndicator,
					UpdateStatus:          mm_status.UpdateStatusInProgress.Status,
				},
			}},
			want: true,
		},
		{
			name: "Failed",
			args: args{&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					UpdateStatusIndicator: statusv1.StatusIndication_STATUS_INDICATION_IDLE,
					UpdateStatus:          mm_status.UpdateStatusInProgress.Status,
				},
			}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.IsHostUnderMaintain(tt.args.hostres); got != tt.want {
				t.Errorf("IsHostUnderMaintain() = %v, want %v", got, tt.want)
			}
		})
	}
}

//nolint:funlen // this is a table-driven test, length is expected to be big
func TestIsSameHostSystemInfo(t *testing.T) {
	hostRes := &computev1.HostResource{
		Name:         "for unit testing purposes",
		DesiredState: computev1.HostState_HOST_STATE_ONBOARDED,

		HardwareKind: "XDgen2",
		SerialNumber: "12345678",
		Uuid:         "E5E53D99-708D-4AF5-8378-63880FF62712",
		MemoryBytes:  64,

		CpuModel:        "12th Gen Intel(R) Core(TM) i9-12900",
		CpuSockets:      1,
		CpuCores:        14,
		CpuCapabilities: "",
		CpuArchitecture: "x86_64",
		CpuThreads:      10,

		MgmtIp: "192.168.10.10",

		BmcKind:     computev1.BaremetalControllerKind_BAREMETAL_CONTROLLER_KIND_PDU,
		BmcIp:       "10.0.0.10",
		BmcUsername: "user",
		BmcPassword: "pass",
		PxeMac:      "90:49:fa:ff:ff:ff",

		Hostname: "testhost1",

		DesiredPowerState: computev1.PowerState_POWER_STATE_ON,

		Site:     nil,
		Provider: nil,
		Instance: nil,
	}

	hostStorages := make([]*computev1.HoststorageResource, 0, 1)
	hostStorages = append(hostStorages, hostStorageResource)
	hostRes.HostStorages = hostStorages

	hostNics := make([]*computev1.HostnicResource, 0, 1)
	hostNics = append(hostNics, hostNicResource)
	hostRes.HostNics = hostNics

	hostUsbs := make([]*computev1.HostusbResource, 0, 1)
	hostUsbs = append(hostUsbs, hostUsbResource)
	hostRes.HostUsbs = hostUsbs

	hostGpus := make([]*computev1.HostgpuResource, 0, 1)
	hostGpus = append(hostGpus, hostGpuResource)
	hostRes.HostGpus = hostGpus

	tests := []struct {
		name string
		in   *pb.SystemInfo
		want *computev1.HostResource
		same bool
	}{
		{
			name: "FullHWConversion_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "01/31/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
					Storage: &pb.Storage{
						Disk: []*pb.SystemDisk{
							disk,
						},
					},
					Network: []*pb.SystemNetwork{
						nic,
					},
					Usb: []*pb.SystemUSB{
						usb,
					},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "01/31/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoUSB_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
					Storage: &pb.Storage{
						Disk: []*pb.SystemDisk{
							disk,
						},
					},
					Network: []*pb.SystemNetwork{
						nic,
					},
					Usb: nil,
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoStorage_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
					Storage: nil,
					Network: []*pb.SystemNetwork{
						nic,
					},
					Usb: []*pb.SystemUSB{
						usb,
					},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoNICs_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
					Storage: &pb.Storage{
						Disk: []*pb.SystemDisk{
							disk,
						},
					},
					Network: nil,
					Usb: []*pb.SystemUSB{
						usb,
					},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoGPUInfo_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: nil,
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoCPUInfo_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Features: []string{""},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoMemoryInfo_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory:      &pb.SystemMemory{},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoHWInfo_Success",
			want: &computev1.HostResource{
				BiosVendor:      "Dell Inc.",
				BiosReleaseDate: "12/09/2022",
				BiosVersion:     "1.0.0",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					Cpu: &pb.SystemCPU{
						Features: []string{""},
					},
					Memory: &pb.SystemMemory{},
				},
				BiosInfo: &pb.BiosInfo{
					Version:     "1.0.0",
					ReleaseDate: "12/09/2022",
					Vendor:      "Dell Inc.",
				},
			},
			same: false,
		},
		{
			name: "NoBios_Success",
			want: &computev1.HostResource{
				SerialNumber:    "test",
				ProductName:     "test-product",
				MemoryBytes:     1,
				CpuSockets:      1,
				CpuArchitecture: "x86",
				CpuModel:        "Intel Core i9-14900K",
				CpuCores:        24,
				CpuThreads:      32,
				CpuCapabilities: "capability1,capability2,capability3",
			},
			in: &pb.SystemInfo{
				HwInfo: &pb.HWInfo{
					SerialNum:   "test",
					ProductName: "test-product",
					Memory: &pb.SystemMemory{
						Size: 1,
					},
					Cpu: &pb.SystemCPU{
						Sockets:  1,
						Arch:     "x86",
						Model:    "Intel Core i9-14900K",
						Cores:    24,
						Threads:  32,
						Features: []string{"capability1", "capability2", "capability3"},
					},
					Gpu: []*pb.SystemGPU{
						gpu,
					},
				},
				BiosInfo: &pb.BiosInfo{},
			},
			same: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			updatedHost, fieldmask, err := util.PopulateHostResourceWithNewSystemInfo(tc.in)
			require.NoError(t, err)

			isSame, err := util.IsSameHost(tc.want, updatedHost, fieldmask)
			require.NoError(t, err)
			// TC should not fail (!tc.fail)
			assert.Equal(t, !tc.same, isSame,
				"Assertion failed, TC should be %v, but comparison returned %v", tc.same, isSame)
		})
	}
}

func TestIsSameInstanceStateStatusDetail(t *testing.T) {
	tests := []struct {
		name  string
		in    *pb.UpdateInstanceStateStatusByHostGUIDRequest
		inRes *computev1.InstanceResource
		same  bool
	}{
		{
			name: "IdenticalInstances",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
			},
			inRes: &computev1.InstanceResource{
				CurrentState:   computev1.InstanceState_INSTANCE_STATE_RUNNING,
				InstanceStatus: hrm_status.InstanceStatusRunning.Status,
			},
			same: true,
		},
		{
			name: "DifferentState",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
				InstanceState:  pb.InstanceState_INSTANCE_STATE_UNSPECIFIED,
			},
			inRes: &computev1.InstanceResource{
				CurrentState: computev1.InstanceState_INSTANCE_STATE_RUNNING,
			},
			same: false,
		},
		{
			name: "DifferentStatus",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus: pb.InstanceStatus_INSTANCE_STATUS_PROVISIONING,
				InstanceState:  pb.InstanceState_INSTANCE_STATE_RUNNING,
			},
			inRes: &computev1.InstanceResource{
				CurrentState: computev1.InstanceState_INSTANCE_STATE_RUNNING,
			},
			same: false,
		},
		{
			name: "IdenticalInstances",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				ProviderStatusDetail: "5 of 5 components are running",
			},
			inRes: &computev1.InstanceResource{
				InstanceStatusDetail: "5 of 5 components are running",
			},
			same: true,
		},
		{
			name: "DifferentInstanceStatusDetail",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				ProviderStatusDetail: "2 of 5 components are running",
			},
			inRes: &computev1.InstanceResource{
				InstanceStatusDetail: "0 of 5 components are running",
			},
			same: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isSame := util.IsSameInstanceStateStatusDetail(tc.in, tc.inRes)
			assert.Equal(t, tc.same, isSame,
				"Assertion failed, TC should be %v, but comparison returned %v", tc.same, isSame)
		})
	}
}

func TestUpdateInstanceStateStatusToUpdateHostStatus(t *testing.T) {
	tests := []struct {
		name  string
		in    *pb.UpdateInstanceStateStatusByHostGUIDRequest
		inRes *computev1.InstanceResource
		out   *pb.HostStatus
	}{
		{
			name: "Running state",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus:       pb.InstanceStatus_INSTANCE_STATUS_RUNNING,
				ProviderStatusDetail: "details",
			},
			out: &pb.HostStatus{
				HostStatus:          pb.HostStatus_RUNNING,
				HumanReadableStatus: "details",
				Details:             "",
			},
		},
		{
			name: "Error state",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus:       pb.InstanceStatus_INSTANCE_STATUS_BOOTING,
				ProviderStatusDetail: "booting details",
			},
			out: &pb.HostStatus{
				HostStatus:          pb.HostStatus_BOOTING,
				HumanReadableStatus: "booting details",
				Details:             "",
			},
		},
		{
			name: "Updating state",
			in: &pb.UpdateInstanceStateStatusByHostGUIDRequest{
				InstanceStatus:       pb.InstanceStatus_INSTANCE_STATUS_UPDATING,
				ProviderStatusDetail: "updating details",
			},
			out: &pb.HostStatus{
				HostStatus:          pb.HostStatus_UPDATING,
				HumanReadableStatus: "updating details",
				Details:             "",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			returned := util.InstanceStatusToHostStatusMsg(tc.in)
			assert.Equal(t, tc.out.GetHumanReadableStatus(), returned.GetHumanReadableStatus())
			assert.Equal(t, tc.out.GetDetails(), returned.GetDetails())
			assert.Equal(t, tc.out.GetHostStatus(), returned.GetHostStatus())
		})
	}
}

func TestMarshalHostCPUTopology(t *testing.T) {
	type args struct {
		hostCPUTopology *pb.CPUTopology
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Success",
			args: args{
				&pb.CPUTopology{
					Sockets: []*pb.Socket{
						{
							SocketId: 0,
							CoreGroups: []*pb.CoreGroup{
								{
									CoreType: "Type A",
									CoreList: []uint32{1, 2, 3},
								},
								{
									CoreType: "Type B",
									CoreList: []uint32{4, 5, 6},
								},
							},
						},
						{
							SocketId: 1,
							CoreGroups: []*pb.CoreGroup{
								{
									CoreType: "Type A",
									CoreList: []uint32{1, 2, 3},
								},
								{
									CoreType: "Type B",
									CoreList: []uint32{4, 5, 6},
								},
							},
						},
					},
				},
			},
			//nolint:lll // it's easier to read a one-liner JSON field
			want:    "{\"sockets\":[{\"socket_id\":0,\"core_groups\":[{\"core_type\":\"Type A\",\"core_list\":[1,2,3]},{\"core_type\":\"Type B\",\"core_list\":[4,5,6]}]},{\"socket_id\":1,\"core_groups\":[{\"core_type\":\"Type A\",\"core_list\":[1,2,3]},{\"core_type\":\"Type B\",\"core_list\":[4,5,6]}]}]}",
			wantErr: false,
		},
		{
			name: "Success_NilCpuTopology",
			args: args{
				hostCPUTopology: nil,
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Success_NilSockets",
			args: args{
				hostCPUTopology: &pb.CPUTopology{
					Sockets: nil,
				},
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "Success_EmptySockets",
			args: args{
				hostCPUTopology: &pb.CPUTopology{
					Sockets: []*pb.Socket{},
				},
			},
			want:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := util.MarshalHostCPUTopology(tt.args.hostCPUTopology)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equalf(t, tt.want, got, "MarshalHostCPUTopology(%v)", tt.args.hostCPUTopology)
		})
	}
}

//nolint:funlen // this is a table-driven test
func TestProtoEqualSubset(t *testing.T) {
	tests := []struct {
		name     string
		res1     *computev1.HostResource
		res2     *computev1.HostResource
		included []string
		equal    bool
	}{
		{
			"IdenticalFieldsIncluded",
			&computev1.HostResource{
				SerialNumber: "ABC123",
				ProductName:  "TestProduct",
			},
			&computev1.HostResource{
				SerialNumber: "ABC123",
				ProductName:  "TestProduct",
				MemoryBytes:  1024,
			},
			[]string{
				computev1.HostResourceFieldSerialNumber,
				computev1.HostResourceFieldProductName,
			},
			true,
		},
		{
			"IdenticalSubresourceFieldIncluded",
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name: "InstanceName1",
				},
			},
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name:          "InstanceName1",
					VmMemoryBytes: 100000,
				},
			},
			[]string{computev1.HostResourceEdgeInstance + "." + computev1.InstanceResourceFieldName},
			true,
		},
		{
			"IdenticalSubresourceIncluded",
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name: "InstanceName1",
				},
			},
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name: "InstanceName1",
				},
			},
			[]string{computev1.HostResourceEdgeInstance},
			true,
		},
		{
			"DifferentSubresourceIncluded",
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name: "InstanceName1",
				},
			},
			&computev1.HostResource{
				Instance: &computev1.InstanceResource{
					Name: "InstanceName2",
				},
			},
			[]string{computev1.HostResourceEdgeInstance},
			false,
		},
		{
			"IdenticalFieldsIncludedZero",
			&computev1.HostResource{
				SerialNumber: "ABC123",
				ProductName:  "TestProduct",
			},
			&computev1.HostResource{
				SerialNumber: "ABC123",
				ProductName:  "TestProduct",
				MemoryBytes:  1024, // Different field not included in comparison
			},
			[]string{
				computev1.HostResourceFieldSerialNumber,
				computev1.HostResourceFieldProductName,
				computev1.HostResourceFieldBiosVersion, // Compare also on an unset field
			},
			true,
		},
		{
			"DifferentIncludedField",
			&computev1.HostResource{
				SerialNumber: "ABC123",
				ProductName:  "TestProduct",
			},
			&computev1.HostResource{
				SerialNumber: "XYZ789",
				ProductName:  "TestProduct",
			},
			[]string{
				computev1.HostResourceFieldSerialNumber,
				computev1.HostResourceFieldProductName,
			},
			false,
		},
		{
			"NoOverlapInIncludedFields",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductB",
			},
			[]string{
				computev1.HostResourceFieldSerialNumber, // both empty
			},
			true,
		},
		{
			"DifferentWithEmptyIncludedField",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductB",
			},
			[]string{},
			false,
		},
		{
			"SameWithEmptyIncludedField",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			[]string{},
			true,
		},
		{
			"DifferentInvalidIncludedField",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductB",
			},
			[]string{"NON_EXISTENT_FIELD"},
			false,
		},
		{
			"SameInvalidIncludedField",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			[]string{"NON_EXISTENT_FIELD"},
			true,
		},
		{
			"DifferentResourcesKind",
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			&computev1.HostResource{
				ProductName: "TestProductA",
			},
			[]string{"NON_EXISTENT_FIELD"},
			true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := util.ProtoEqualSubset(tc.res1, tc.res2, tc.included...)
			if result != tc.equal {
				t.Errorf("Expected %v, got %v", tc.equal, result)
			}
		})
	}
}

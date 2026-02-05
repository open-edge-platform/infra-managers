// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package hostmgr

import (
	"context"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	inv_mgr_cli "github.com/open-edge-platform/infra-managers/host/pkg/invclient"
	hmgr_util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

// updateHost retrieves a current host resource from Inventory by GUID, overwrites its info with
// the SystemInfo provided and inserts the updated host resource to Inventory.
func updateHost(ctx context.Context, tenantID string, hostResc *computev1.HostResource, info *pb.SystemInfo) error {
	zlog.Debug().Msgf("Updating Host (tID=%s, UUID=%s) in Inventory: %v", tenantID, hostResc.GetUuid(), hostResc)

	updatedHostres, fieldmask, err := hmgr_util.PopulateHostResourceWithNewSystemInfo(info)
	if err != nil {
		return err
	}

	isSame, err := hmgr_util.IsSameHost(hostResc, updatedHostres, fieldmask)
	if err != nil {
		return err
	}

	if isSame {
		zlog.Debug().Msgf("Skipping HostSystemInfo update for Host (tID=%s, UUID=%s) - no changes: %v",
			tenantID, hostResc.GetUuid(), hostResc)
		return nil
	}

	updatedHostres.ResourceId = hostResc.GetResourceId()
	err = inv_mgr_cli.UpdateInvResourceFields(ctx, invClientInstance, tenantID, updatedHostres, fieldmask.GetPaths())
	if err != nil {
		return err
	}

	return nil
}

// Logic is the following - use the storage device name (as reported by bare metal'\; agent) as unique identifier.
// FIXME: ITEP-20558 Use one of the unique identifier(wwid/SN) together with the name.
func findStorageInList(storageToFind *computev1.HoststorageResource, listOfStorages []*computev1.HoststorageResource) (
	*computev1.HoststorageResource, bool,
) {
	for _, storage := range listOfStorages {
		if storageToFind.GetDeviceName() == storage.GetDeviceName() {
			return storage, true
		}
	}
	return nil, false
}

// Helper function to reduce cyclomatic complexity.
//
//nolint:dupl // Protobuf oneOf-driven separation
func hostStorageToAddOrUpdate(ctx context.Context, tenantID string, update bool,
	hostStorage, invStorage *computev1.HoststorageResource,
) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host storage: tenantID=%s, hostStorage=%v", update, tenantID, hostStorage)
	if update {
		hostStorage.ResourceId = invStorage.GetResourceId()
		// Nop or update the storage
		if !hmgr_util.ProtoEqualSubset(hostStorage, invStorage, inv_mgr_cli.UpdateHoststorageFieldMask...) {
			if err := inv_mgr_cli.UpdateHoststorage(ctx, invClientInstance, tenantID, hostStorage); err != nil {
				return err
			}
		} else {
			// this is here just to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostStorage update: tenantID=%s, hostStorage=%v", tenantID, hostStorage)
		}
	} else {
		// Add the storage
		id, err := inv_mgr_cli.CreateHoststorage(ctx, invClientInstance, tenantID, hostStorage)
		if err != nil {
			return err
		}
		hostStorage.ResourceId = id
	}
	return nil
}

// Helper function to reduce cyclomatic complexity.
func hostStorageToRemove(ctx context.Context, tenantID string, remove bool, invStorage *computev1.HoststorageResource,
) error {
	if remove {
		zlog.Debug().Msgf("Delete host storage: tenantID=%s, hostStorage=%v", tenantID, invStorage)
		if err := inv_mgr_cli.DeleteHoststorage(ctx, invClientInstance, tenantID, invStorage.GetResourceId()); err != nil {
			return err
		}
	}
	return nil
}

// This function update Host storages resources in Inventory if needed.
func updateHoststorage(ctx context.Context, tenantID string, hostRes *computev1.HostResource, hwInfo *pb.HWInfo) error {
	// Storages are always eager loaded. No need to query Inventory again
	invStorages := hostRes.GetHostStorages()
	hostStorages := make([]*computev1.HoststorageResource, 0)

	zlog.Debug().Msgf("Update host storages. tenantID=%s, Inventory Storagess=%v, reported storage info=%v",
		tenantID, invStorages, hwInfo.GetStorage())

	// Find storages to add or update
	for _, storage := range hwInfo.GetStorage().GetDisk() {
		hostStorage, err := hmgr_util.PopulateHoststorageWithDiskInfo(storage, hostRes)
		if err != nil {
			return err
		}
		hostStorages = append(hostStorages, hostStorage)
		invStorage, exists := findStorageInList(hostStorage, invStorages)
		err = hostStorageToAddOrUpdate(ctx, tenantID, exists, hostStorage, invStorage)
		if err != nil {
			return err
		}
	}
	// Then the ones to remove
	for _, invStorage := range invStorages {
		_, exists := findStorageInList(invStorage, hostStorages)
		err := hostStorageToRemove(ctx, tenantID, !exists, invStorage)
		if err != nil {
			return err
		}
	}
	return nil
}

// Helper function to reduce cyclomatic complexity.
//
//nolint:dupl // Protobuf oneOf-driven separation
func hostDeviceToAddOrUpdate(ctx context.Context, tenantID string, update bool,
	hostDevice, invDevice *computev1.HostdeviceResource,
) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host device: tenantID=%s, hostDevice=%v", update, tenantID, hostDevice)
	if update {
		hostDevice.ResourceId = invDevice.GetResourceId()
		// Nop or update the device
		if !hmgr_util.ProtoEqualSubset(hostDevice, invDevice, inv_mgr_cli.UpdateHostdeviceFieldMask...) {
			if err := inv_mgr_cli.UpdateHostdevice(ctx, invClientInstance, tenantID, hostDevice); err != nil {
				return err
			}
		} else {
			// this is here to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostDevice update: tenantID=%s, hostDevice=%v", tenantID, hostDevice)
		}
	} else {
		// Add the device
		id, err := inv_mgr_cli.CreateHostdevice(ctx, invClientInstance, tenantID, hostDevice)
		if err != nil {
			return err
		}
		hostDevice.ResourceId = id
	}
	return nil
}

// Helper function to reduce cyclomatic complexity.
func hostDeviceToRemove(ctx context.Context, tenantID string, invDevice *computev1.HostdeviceResource,
) error {
	zlog.Debug().Msgf("Delete host device: tenantID=%s, hostDevice=%v", tenantID, invDevice)
	if err := inv_mgr_cli.DeleteHostdevice(ctx, invClientInstance, tenantID, invDevice.GetResourceId()); err != nil {
		return err
	}
	return nil
}

// This function updates Host device resources in Inventory if needed.
func updateHostdevice(ctx context.Context, tenantID string, hostRes *computev1.HostResource, deviceInfo *pb.DeviceInfo) error {
	// Devices are always eager loaded. No need to query Inventory again
	invDevice := hostRes.GetHostDevice()

	zlog.Debug().Msgf("Update host device info. tenantID=%s, Inventory Device Info=%v, reported device info=%v",
		tenantID, invDevice, deviceInfo)

	// Populate device info
	hostDevice, err := hmgr_util.PopulateHostdeviceWithDeviceInfo(deviceInfo, hostRes)
	if err != nil {
		return err
	}
	// Check if device info matches with info in invDevice, compare version
	exists := invDevice.GetVersion() == hostDevice.GetVersion()
	if exists {
		// Perform an update of the info in inventory
		err = hostDeviceToAddOrUpdate(ctx, tenantID, exists, hostDevice, invDevice)
		if err != nil {
			return err
		}
	} else {
		// Check if received device info is empty
		hostnameReceived := hostDevice.GetHostname() == ""
		if hostnameReceived {
			// New device info to be added to the inventory
			err = hostDeviceToAddOrUpdate(ctx, tenantID, hostnameReceived, hostDevice, invDevice)
			if err != nil {
				return err
			}
		} else {
			// Empty device info received, delete old info from inventory
			err = hostDeviceToRemove(ctx, tenantID, invDevice)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Logic is the following - use the interface name (as reported by bare metal agent) as unique identifier.
func findNicInList(nicToFind *computev1.HostnicResource, listOfNics []*computev1.HostnicResource) (
	*computev1.HostnicResource, bool,
) {
	for _, nic := range listOfNics {
		if nicToFind.GetDeviceName() == nic.GetDeviceName() {
			return nic, true
		}
	}
	return nil, false
}

// Helper function to reduce cyclomatic complexity.
func hostNicToAddOrUpdate(ctx context.Context, tenantID string, update bool,
	hostNic, invNic *computev1.HostnicResource, network *pb.SystemNetwork,
) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host NIC with tenantID=%s, ID=%s with system network: %v",
		update, tenantID, hostNic.GetResourceId(), network)
	if update {
		hostNic.ResourceId = invNic.GetResourceId()
		// Nop or update the nic
		if !hmgr_util.ProtoEqualSubset(hostNic, invNic, inv_mgr_cli.UpdateHostnicFieldMask...) {
			if err := inv_mgr_cli.UpdateHostnic(ctx, invClientInstance, tenantID, hostNic); err != nil {
				return err
			}
		} else {
			// this is here just to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostNic update: tenantID=%s, hostNIC=%v", tenantID, hostNic)
		}
	} else {
		// Add the nic
		id, err := inv_mgr_cli.CreateHostnic(ctx, invClientInstance, tenantID, hostNic)
		if err != nil {
			return err
		}
		hostNic.ResourceId = id
	}
	// IPAddresses are reconciled now. They are not part of the comparison
	return updateIPAddresses(ctx, tenantID, hostNic, network)
}

// Helper function to reduce cyclomatic complexity.
func hostNicToRemove(ctx context.Context, tenantID string, remove bool, invNic *computev1.HostnicResource,
) error {
	if remove {
		zlog.Debug().Msgf("Delete host nic: tenantID=%s, hostNIC=%v", tenantID, invNic)
		// Firstly the IPAddresses due to the strong relation with nic
		if err := updateIPAddresses(ctx, tenantID, invNic, &pb.SystemNetwork{}); err != nil {
			return err
		}
		if err := inv_mgr_cli.DeleteHostnic(ctx, invClientInstance, tenantID, invNic.GetResourceId()); err != nil {
			return err
		}
	}
	return nil
}

// This function reconciles Host nic resources with Inventory if needed.
func updateHostnics(ctx context.Context, tenantID string, hostRes *computev1.HostResource, hwInfo *pb.HWInfo) error {
	// Nics are always eager loaded. No need to query Inventory again
	invNics := hostRes.GetHostNics()
	hostNics := make([]*computev1.HostnicResource, 0)

	zlog.Debug().Msgf("Updating host NICs. tenantID=%s, Inventory NICs=%v, reported network info=%v",
		tenantID, invNics, hwInfo.GetNetwork())

	// Find nics to add or update
	for _, network := range hwInfo.GetNetwork() {
		hostNic, err := hmgr_util.PopulateHostnicWithNetworkInfo(network, hostRes)
		if err != nil {
			return err
		}
		hostNics = append(hostNics, hostNic)
		invNic, exists := findNicInList(hostNic, invNics)
		err = hostNicToAddOrUpdate(ctx, tenantID, exists, hostNic, invNic, network)
		if err != nil {
			return err
		}
	}
	// Then the ones to remove
	for _, invNic := range invNics {
		_, exists := findNicInList(invNic, hostNics)
		err := hostNicToRemove(ctx, tenantID, !exists, invNic)
		if err != nil {
			return err
		}
	}
	return nil
}

// Logic is merely based on the CIDR notation.
func findIPInList(ipToFind *network_v1.IPAddressResource, listOfIPs []*network_v1.IPAddressResource) (
	*network_v1.IPAddressResource, bool,
) {
	for _, ip := range listOfIPs {
		if ipToFind.GetAddress() == ip.GetAddress() {
			return ip, true
		}
	}
	return nil, false
}

// Helper function to reduce cyclomatic complexity.
func hostIPToAddOrUpdate(ctx context.Context, tenantID string, update bool,
	hostIP, invIP *network_v1.IPAddressResource,
) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host IP with tenantID=%s ID=%s",
		update, tenantID, hostIP.GetResourceId())
	if update {
		hostIP.ResourceId = invIP.GetResourceId()
		if !hmgr_util.ProtoEqualSubset(hostIP, invIP, inv_mgr_cli.UpdateIPAddressFieldMask...) {
			if err := inv_mgr_cli.UpdateIPAddress(ctx, invClientInstance, tenantID, hostIP); err != nil {
				return err
			}
		} else {
			// this is here just to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostIP update: tenantID=%s, IPAddress=%v", tenantID, hostIP)
		}
	} else {
		// Add the nic
		_, err := inv_mgr_cli.CreateIPAddress(ctx, invClientInstance, tenantID, hostIP)
		if err != nil {
			return err
		}
	}
	return nil
}

// Helper function to reduce cyclomatic complexity.
func hostIPToRemove(ctx context.Context, tenantID string, remove bool,
	invIP *network_v1.IPAddressResource,
) error {
	if remove {
		zlog.Debug().Msgf("Delete host IP: tenantID=%s, IPAddress=%v", tenantID, invIP)
		if err := inv_mgr_cli.DeleteIPAddress(ctx, invClientInstance, tenantID, invIP.GetResourceId()); err != nil {
			return err
		}
	}
	return nil
}

func updateIPAddresses(ctx context.Context, tenantID string, hostNic *computev1.HostnicResource, sysNet *pb.SystemNetwork) error {
	// IPs are not eager loaded
	invIPs, err := inv_mgr_cli.ListIPAddresses(ctx, invClientInstance, tenantID, hostNic)
	if err != nil {
		return err
	}

	zlog.Debug().Msgf("Updating IP addresses for Host tenantID=%s, NIC ID=%s. Inventory IPs=%v, reported IP addresses=%v",
		tenantID, hostNic.GetResourceId(), invIPs, sysNet.GetIpAddresses())
	hostIPs := []*network_v1.IPAddressResource{}
	// Find ips to add or update
	for _, ip := range sysNet.GetIpAddresses() {
		hostIP, err := hmgr_util.PopulateIPAddressWithIPAddressInfo(ip, hostNic)
		if err != nil {
			return err
		}
		hostIPs = append(hostIPs, hostIP)
		invIP, exists := findIPInList(hostIP, invIPs)
		err = hostIPToAddOrUpdate(ctx, tenantID, exists, hostIP, invIP)
		if err != nil {
			return err
		}
	}
	// Then the ones to remove
	for _, invIP := range invIPs {
		_, exists := findIPInList(invIP, hostIPs)
		err := hostIPToRemove(ctx, tenantID, !exists, invIP)
		if err != nil {
			return err
		}
	}
	return nil
}

// Logic is the following - use the bus and the addr as unique identifier.
func findUsbInList(usbToFind *computev1.HostusbResource, listOfUsbs []*computev1.HostusbResource) (
	*computev1.HostusbResource, bool,
) {
	for _, usb := range listOfUsbs {
		if usbToFind.Bus == usb.GetBus() && usbToFind.GetAddr() == usb.GetAddr() {
			return usb, true
		}
	}
	return nil, false
}

// Helper function to reduce cyclomatic complexity.
//
//nolint:dupl // Protobuf oneOf-driven separation
func hostUsbToAddOrUpdate(ctx context.Context, tenantID string, update bool, hostUsb, invUsb *computev1.HostusbResource) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host usb: tenantID=%s, hostUSB=%v", update, tenantID, hostUsb)
	if update {
		hostUsb.ResourceId = invUsb.GetResourceId()
		// Nop or update the usb
		if !hmgr_util.ProtoEqualSubset(hostUsb, invUsb, inv_mgr_cli.UpdateHostusbFieldMask...) {
			if err := inv_mgr_cli.UpdateHostusb(ctx, invClientInstance, tenantID, hostUsb); err != nil {
				return err
			}
		} else {
			// this is here just to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostUsb update: tenantID=%s, hostUSB=%v", tenantID, hostUsb)
		}
	} else {
		// Add the usb
		id, err := inv_mgr_cli.CreateHostusb(ctx, invClientInstance, tenantID, hostUsb)
		if err != nil {
			return err
		}
		hostUsb.ResourceId = id
	}
	return nil
}

// Helper function to reduce cyclomatic complexity.
func hostUsbToRemove(ctx context.Context, tenantID string, remove bool, invUsb *computev1.HostusbResource,
) error {
	if remove {
		zlog.Debug().Msgf("Delete host usb: tenantID=%s, hostUSB=%v", tenantID, invUsb)
		if err := inv_mgr_cli.DeleteHostusb(ctx, invClientInstance, tenantID, invUsb.GetResourceId()); err != nil {
			return err
		}
	}
	return nil
}

// This function reconciles Host usb resources with Inventory if needed.
//
//nolint:dupl // Protobuf oneOf-driven separation
func updateHostusbs(ctx context.Context, tenantID string, hostRes *computev1.HostResource, hwInfo *pb.HWInfo) error {
	// USBs are always eager loaded. No need to query Inventory again
	invUsbs := hostRes.HostUsbs
	hostUsbs := make([]*computev1.HostusbResource, 0)

	zlog.Debug().Msgf("Reconciling host USBs. tenantID=%s, Inventory USBs=%v, reported USBs info=%v",
		tenantID, invUsbs, hwInfo.GetUsb())

	// Find usbs to add or update
	for _, usb := range hwInfo.GetUsb() {
		hostUsb, err := hmgr_util.PopulateHostusbWithUsbInfo(usb, hostRes)
		if err != nil {
			return err
		}
		hostUsbs = append(hostUsbs, hostUsb)
		invUsb, exists := findUsbInList(hostUsb, invUsbs)
		err = hostUsbToAddOrUpdate(ctx, tenantID, exists, hostUsb, invUsb)
		if err != nil {
			return err
		}
	}
	// Then the ones to remove
	for _, invUsb := range invUsbs {
		_, exists := findUsbInList(invUsb, hostUsbs)
		err := hostUsbToRemove(ctx, tenantID, !exists, invUsb)
		if err != nil {
			return err
		}
	}
	return nil
}

// findGpuInList finds the GPU instance in the list. PCI identifier is used as unique ID.
func findGpuInList(gpuToFind *computev1.HostgpuResource, listOfGpus []*computev1.HostgpuResource) (
	*computev1.HostgpuResource, bool,
) {
	for _, gpu := range listOfGpus {
		if gpuToFind.GetPciId() == gpu.GetPciId() {
			return gpu, true
		}
	}
	return nil, false
}

//nolint:dupl // Protobuf oneOf-driven separation
func hostGpuToAddOrUpdate(ctx context.Context, tenantID string, update bool, hostGpu, invGpu *computev1.HostgpuResource) error {
	zlog.Debug().Msgf("AddOrUpdate (update=%v) host GPU: tenantID=%s, hostGPU=%v", update, tenantID, hostGpu)
	if update {
		hostGpu.ResourceId = invGpu.GetResourceId()
		// Nop or update the GPU
		if !hmgr_util.ProtoEqualSubset(hostGpu, invGpu, inv_mgr_cli.UpdateHostgpuFieldMask...) {
			if err := inv_mgr_cli.UpdateHostgpu(ctx, invClientInstance, tenantID, hostGpu); err != nil {
				return err
			}
		} else {
			// this is here just to verify that ut are covering this branch
			zlog.Debug().Msgf("Skip hostGpu update: tenantID=%s, hostGPU=%v", tenantID, hostGpu)
		}
	} else {
		id, err := inv_mgr_cli.CreateHostgpu(ctx, invClientInstance, tenantID, hostGpu)
		if err != nil {
			return err
		}
		hostGpu.ResourceId = id
	}
	return nil
}

func hostGpuToRemove(ctx context.Context, tenantID string, remove bool, invGpu *computev1.HostgpuResource,
) error {
	if remove {
		zlog.Debug().Msgf("Delete host GPU: tenantID=%s, hostGPU=%v", tenantID, invGpu)
		if err := inv_mgr_cli.DeleteHostgpu(ctx, invClientInstance, tenantID, invGpu.GetResourceId()); err != nil {
			return err
		}
	}
	return nil
}

//nolint:dupl // Protobuf oneOf-driven separation
func updateHostgpus(ctx context.Context, tenantID string, host *computev1.HostResource, hwInfo *pb.HWInfo) error {
	// GPUs are always eager loaded. No need to query Inventory again
	invGpus := host.HostGpus
	hostGpus := make([]*computev1.HostgpuResource, 0)

	zlog.Debug().Msgf("Reconciling host GPUs. tenantID=%s, Inventory GPUs=%v, reported GPUs info=%v",
		tenantID, invGpus, hwInfo.GetGpu())

	for _, gpu := range hwInfo.GetGpu() {
		hostGpu, err := hmgr_util.PopulateHostgpuWithGpuInfo(gpu, host)
		if err != nil {
			return err
		}
		hostGpus = append(hostGpus, hostGpu)
		invGpu, exists := findGpuInList(hostGpu, invGpus)
		err = hostGpuToAddOrUpdate(ctx, tenantID, exists, hostGpu, invGpu)
		if err != nil {
			return err
		}
	}
	// Then the ones to remove
	for _, invGpu := range invGpus {
		_, exists := findGpuInList(invGpu, hostGpus)
		err := hostGpuToRemove(ctx, tenantID, !exists, invGpu)
		if err != nil {
			return err
		}
	}
	return nil
}

func updateInstanceStateStatusByHostGUID(ctx context.Context, tenantID string, instRes *computev1.InstanceResource,
	in *pb.UpdateInstanceStateStatusByHostGUIDRequest,
) error {
	if instRes == nil {
		zlog.Warn().Msgf("No instance to update state (tenantID=%s), skip: %v", tenantID, in)
		return nil
	}
	if hmgr_util.IsSameInstanceStateStatusDetail(in, instRes) {
		zlog.Debug().Msgf("Skipping Instance State and Status update for Host (tID=%s, UUID=%s) - no changes",
			tenantID, in.GetHostGuid())
		return nil
	}

	// updating Instance's state and status
	instRes = hmgr_util.UpdateInstanceResourceStateStatusDetails(instRes, in.GetInstanceState(), in.GetInstanceStatus(),
		in.GetProviderStatusDetail(), instRes.GetResourceId())

	// updating an Instance
	err := inv_mgr_cli.UpdateInstanceStateStatusByHostGUID(ctx, invClientInstance, tenantID, instRes.GetResourceId(), instRes)
	if err != nil {
		return inv_errors.ErrorToSanitizedGrpcError(err)
	}
	return nil
}

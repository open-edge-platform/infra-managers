// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package util provides utility functions for host management.
//
//nolint:revive // Package name util is intentional
package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	pb "github.com/open-edge-platform/infra-managers/host/pkg/api/hostmgr/proto"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
	mm_status "github.com/open-edge-platform/infra-managers/maintenance/pkg/status"
	om_status "github.com/open-edge-platform/infra-onboarding/onboarding-manager/pkg/status"
)

var mapNodeStatusToHostStatus = map[pb.HostStatus_HostStatus]inv_status.ResourceStatus{
	pb.HostStatus_UNSPECIFIED: hrm_status.HostStatusUnknown,
	pb.HostStatus_RUNNING:     hrm_status.HostStatusRunning,
	pb.HostStatus_ERROR:       hrm_status.HostStatusError,
	// Other legacy host statuses are not mapped to modern "host status", because they are
	// handled by other modern status (e.g., update_status).
}

var mapNodeStatusToInstanceStatus = map[pb.InstanceStatus]inv_status.ResourceStatus{
	pb.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED:  hrm_status.InstanceStatusEmpty,
	pb.InstanceStatus_INSTANCE_STATUS_INITIALIZING: hrm_status.InstanceStatusInitializing,
	pb.InstanceStatus_INSTANCE_STATUS_RUNNING:      hrm_status.InstanceStatusRunning,
	pb.InstanceStatus_INSTANCE_STATUS_ERROR:        hrm_status.InstanceStatusError,
	// Other legacy instance statuses are not mapped to modern "instance status", because they are
	// handled by other modern status (e.g., update_status).
}

var mapIPConfigMode = map[pb.ConfigMode]network_v1.IPAddressConfigMethod{
	pb.ConfigMode_CONFIG_MODE_UNSPECIFIED: network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_UNSPECIFIED,
	pb.ConfigMode_CONFIG_MODE_STATIC:      network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_STATIC,
	pb.ConfigMode_CONFIG_MODE_DYNAMIC:     network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_DYNAMIC,
}

var instanceStateToInstanceResourceState = map[pb.InstanceState]computev1.InstanceState{
	pb.InstanceState_INSTANCE_STATE_UNSPECIFIED: computev1.InstanceState_INSTANCE_STATE_UNSPECIFIED,
	pb.InstanceState_INSTANCE_STATE_RUNNING:     computev1.InstanceState_INSTANCE_STATE_RUNNING,
	pb.InstanceState_INSTANCE_STATE_DELETED:     computev1.InstanceState_INSTANCE_STATE_DELETED,
	// TODO: Installed and stopped states should be deprecated. Temporary mapping them to Running, but the states are be unused.
	pb.InstanceState_INSTANCE_STATE_INSTALLED: computev1.InstanceState_INSTANCE_STATE_RUNNING,
	pb.InstanceState_INSTANCE_STATE_STOPPED:   computev1.InstanceState_INSTANCE_STATE_RUNNING,
}

var instanceStatusToHostStatus = map[pb.InstanceStatus]pb.HostStatus_HostStatus{
	pb.InstanceStatus_INSTANCE_STATUS_UNSPECIFIED:      pb.HostStatus_UNSPECIFIED,
	pb.InstanceStatus_INSTANCE_STATUS_BOOTING:          pb.HostStatus_BOOTING,
	pb.InstanceStatus_INSTANCE_STATUS_BOOT_FAILED:      pb.HostStatus_BOOTFAILED,
	pb.InstanceStatus_INSTANCE_STATUS_PROVISIONING:     pb.HostStatus_PROVISIONING,
	pb.InstanceStatus_INSTANCE_STATUS_PROVISIONED:      pb.HostStatus_PROVISIONED,
	pb.InstanceStatus_INSTANCE_STATUS_PROVISION_FAILED: pb.HostStatus_PROVISIONFAILED,
	pb.InstanceStatus_INSTANCE_STATUS_RUNNING:          pb.HostStatus_RUNNING,
	pb.InstanceStatus_INSTANCE_STATUS_ERROR:            pb.HostStatus_ERROR,
	pb.InstanceStatus_INSTANCE_STATUS_UPDATING:         pb.HostStatus_UPDATING,
	pb.InstanceStatus_INSTANCE_STATUS_UPDATE_FAILED:    pb.HostStatus_UPDATEFAILED,
	pb.InstanceStatus_INSTANCE_STATUS_INITIALIZING:     pb.HostStatus_RUNNING,
}

var zlog = logging.GetLogger("HostMgrUtils")

// MarshalHostCPUTopology marshals the host CPU topology to JSON.
func MarshalHostCPUTopology(hostCPUTopology *pb.CPUTopology) (string, error) {
	if hostCPUTopology == nil {
		return "", nil
	}
	if len(hostCPUTopology.GetSockets()) == 0 {
		return "", nil
	}

	rawBytes, err := protojson.MarshalOptions{
		UseProtoNames:     true,
		EmitDefaultValues: true,
	}.Marshal(hostCPUTopology)
	if err != nil {
		zlog.InfraErr(err).Msg("marshal CPU topology error")
		return "", errors.Wrap(err)
	}

	// protojson generates randomized JSON string, see https://github.com/golang/protobuf/issues/1082
	// We need to convert whitespaces to make the JSON output consistent.
	data := new(bytes.Buffer)
	err = json.Compact(data, rawBytes)
	if err != nil {
		zlog.InfraErr(err).Msg("marshal CPU topology error")
		return "", errors.Wrap(err)
	}

	invHostCPUTopology := data.String()

	return invHostCPUTopology, nil
}

// GetHostStatus returns the host status from a status resource.
func GetHostStatus(status pb.HostStatus_HostStatus) inv_status.ResourceStatus {
	if s, ok := mapNodeStatusToHostStatus[status]; ok {
		return s
	}
	return hrm_status.HostStatusUnknown
}

// GetInstanceStatus returns the instance status from a status resource.
func GetInstanceStatus(status pb.InstanceStatus) inv_status.ResourceStatus {
	if s, ok := mapNodeStatusToInstanceStatus[status]; ok {
		return s
	}

	return hrm_status.InstanceStatusUnknown
}

// PopulateHostusbWithUsbInfo translates a system usb into an host usb resource.
func PopulateHostusbWithUsbInfo(usb *pb.SystemUSB, hostres *computev1.HostResource) (*computev1.HostusbResource, error) {
	if usb == nil {
		zlog.InfraSec().InfraError("SystemUSB cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "SystemUSB cannot be nil")
	}
	if hostres == nil {
		zlog.InfraSec().InfraError("HostResource cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "HostResource cannot be nil")
	}
	usbres := computev1.HostusbResource{
		TenantId:   hostres.GetTenantId(),
		Idvendor:   usb.GetIdvendor(),
		Idproduct:  usb.GetIdproduct(),
		Bus:        usb.GetBus(),
		Addr:       usb.GetAddr(),
		Class:      usb.GetClass(),
		DeviceName: usb.GetDescription(),
		Serial:     usb.GetSerial(),
		Host:       hostres,
	}
	return &usbres, nil
}

// PopulateHostgpuWithGpuInfo populates host GPU resource with GPU information.
func PopulateHostgpuWithGpuInfo(gpu *pb.SystemGPU, host *computev1.HostResource) (*computev1.HostgpuResource, error) {
	if gpu == nil {
		err := errors.Errorfc(codes.InvalidArgument, "SystemGPU cannot be nil")
		zlog.InfraSec().InfraErr(err).Msgf("")
		return nil, err
	}
	if host == nil {
		err := errors.Errorfc(codes.InvalidArgument, "HostResource cannot be nil")
		zlog.InfraSec().InfraErr(err).Msgf("")
		return nil, err
	}
	if gpu.PciId == "" {
		// We cannot enforce validation in the Protobuf due to backwards compatibility with
		// older HD agents that send an empty SystemGPU. So, we validate the PCI identifier here.
		// The PCI identifier must be provided because it uniquely identifies a GPU card.
		err := errors.Errorfc(codes.InvalidArgument, "PCI identifier must be provided for GPU")
		zlog.InfraSec().InfraErr(err).Msgf("")
		return nil, err
	}
	gpures := computev1.HostgpuResource{
		TenantId:    host.GetTenantId(),
		PciId:       gpu.GetPciId(),
		Product:     gpu.GetProduct(),
		Vendor:      gpu.GetVendor(),
		Description: gpu.GetDescription(),
		DeviceName:  gpu.GetName(),
		Features:    strings.Join(gpu.GetFeatures(), ","),
		Host:        host,
	}
	return &gpures, nil
}

// PopulateHostResourceWithNewSystemInfo function gets on input System Information to be updated.
// It constructs a Host resource structure with an updated System Information. Fields not present in
// the System Information are automatically being set to 'nil'. Fieldmask for System Information is
// being produced for future update of the Host resource.
// NIC/Storage/USBs resources are handled in different functions.
//
//nolint:cyclop // complexity is 11
func PopulateHostResourceWithNewSystemInfo(systemInfo *pb.SystemInfo) (
	*computev1.HostResource, *fieldmaskpb.FieldMask, error,
) {
	zlog.InfraSec().Debug().Msg("Populating Host resource with updated system information")

	if systemInfo == nil {
		zlog.InfraSec().InfraError("invalid input: system info is nil").Msg("")
		return nil, nil, errors.Errorfc(codes.InvalidArgument, "invalid input: system info is nil")
	}

	hr := &computev1.HostResource{}
	fieldmask := make([]string, 0, 13) //nolint:mnd // Max number of host resource fields that can be updated

	if systemInfo.HwInfo != nil {
		if systemInfo.HwInfo.SerialNum != "" {
			hr.SerialNumber = systemInfo.HwInfo.SerialNum
		}
		if systemInfo.HwInfo.ProductName != "" {
			hr.ProductName = systemInfo.HwInfo.ProductName
		}
		if systemInfo.HwInfo.Memory != nil {
			hr.MemoryBytes = systemInfo.HwInfo.Memory.Size
		}
		if systemInfo.HwInfo.Cpu != nil {
			hr.CpuSockets = systemInfo.HwInfo.Cpu.Sockets
			hr.CpuArchitecture = systemInfo.HwInfo.Cpu.Arch
			hr.CpuModel = systemInfo.HwInfo.Cpu.Model
			hr.CpuCores = systemInfo.HwInfo.Cpu.Cores
			hr.CpuThreads = systemInfo.HwInfo.Cpu.Threads
			hr.CpuCapabilities = strings.Join(systemInfo.HwInfo.Cpu.Features, ",")

			if systemInfo.HwInfo.Cpu.CpuTopology != nil {
				if len(systemInfo.HwInfo.Cpu.CpuTopology.GetSockets()) != int(systemInfo.HwInfo.Cpu.Sockets) {
					return nil, nil, errors.Errorfc(codes.InvalidArgument,
						"Number of Socket objects in the CPU topology doesn't equal provided Sockets number (%d != %d)",
						len(systemInfo.HwInfo.Cpu.CpuTopology.GetSockets()),
						int(systemInfo.HwInfo.Cpu.Sockets))
				}

				cpuTopology, err := MarshalHostCPUTopology(systemInfo.HwInfo.Cpu.GetCpuTopology())
				if err != nil {
					return nil, nil, err
				}
				hr.CpuTopology = cpuTopology
			}
		}
	}

	if systemInfo.BiosInfo != nil {
		hr.BiosVendor = systemInfo.BiosInfo.Vendor
		hr.BiosVersion = systemInfo.BiosInfo.Version
		hr.BiosReleaseDate = systemInfo.BiosInfo.ReleaseDate
	}

	// adding all expected fields to get updated by invclient.UpdateHostResource function by default
	fieldmask = append(fieldmask, computev1.HostResourceFieldSerialNumber,
		computev1.HostResourceFieldProductName, computev1.HostResourceFieldMemoryBytes,
		computev1.HostResourceFieldCpuSockets, computev1.HostResourceFieldCpuArchitecture,
		computev1.HostResourceFieldCpuModel, computev1.HostResourceFieldCpuCores,
		computev1.HostResourceFieldCpuThreads, computev1.HostResourceFieldCpuCapabilities,
		computev1.HostResourceFieldCpuTopology,
		computev1.HostResourceFieldBiosVendor, computev1.HostResourceFieldBiosVersion,
		computev1.HostResourceFieldBiosReleaseDate)

	return hr, &fieldmaskpb.FieldMask{
		Paths: fieldmask,
	}, nil
}

// PopulateHoststorageWithDiskInfo translates a system disk into an host storage resource.
func PopulateHoststorageWithDiskInfo(disk *pb.SystemDisk, hostres *computev1.HostResource) (
	*computev1.HoststorageResource, error,
) {
	if disk == nil {
		zlog.InfraSec().InfraError("SystemDisk cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "SystemDisk cannot be nil")
	}
	if hostres == nil {
		zlog.InfraSec().InfraError("HostResource cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "HostResource cannot be nil")
	}
	storageres := &computev1.HoststorageResource{
		TenantId:      hostres.GetTenantId(),
		Serial:        disk.GetSerialNumber(),
		DeviceName:    disk.GetName(),
		Vendor:        disk.GetVendor(),
		Model:         disk.GetModel(),
		CapacityBytes: disk.GetSize(),
		Wwid:          disk.GetWwid(),
		Host:          hostres,
	}
	return storageres, nil
}

func linkStateToNetworkInterfaceLinkState(linkState bool) computev1.NetworkInterfaceLinkState {
	if linkState {
		return computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_UP
	}
	return computev1.NetworkInterfaceLinkState_NETWORK_INTERFACE_LINK_STATE_DOWN
}

// PopulateHostnicWithNetworkInfo translates a system network into an host nic resource.
func PopulateHostnicWithNetworkInfo(nic *pb.SystemNetwork, hostRes *computev1.HostResource) (*computev1.HostnicResource, error) {
	if nic == nil {
		zlog.InfraSec().InfraError("SystemNetwork cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "SystemNetwork cannot be nil")
	}
	if hostRes == nil {
		zlog.InfraSec().InfraError("HostResource cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "HostResource cannot be nil")
	}
	nicres := &computev1.HostnicResource{
		TenantId:            hostRes.GetTenantId(),
		Host:                hostRes,
		DeviceName:          nic.Name,
		PciIdentifier:       nic.PciId,
		MacAddr:             nic.Mac,
		SriovEnabled:        nic.Sriovenabled,
		SriovVfsNum:         nic.Sriovnumvfs,
		SriovVfsTotal:       nic.SriovVfsTotal,
		PeerName:            nic.PeerName,
		PeerDescription:     nic.PeerDescription,
		PeerMac:             nic.PeerMac,
		PeerMgmtIp:          nic.PeerMgmtIp,
		PeerPort:            nic.PeerPort,
		SupportedLinkMode:   strings.Join(nic.SupportedLinkMode, ","),
		AdvertisingLinkMode: strings.Join(nic.AdvertisingLinkMode, ","),
		CurrentSpeedBps:     nic.CurrentSpeed,
		CurrentDuplex:       nic.CurrentDuplex,
		Features:            strings.Join(nic.Features, ","),
		Mtu:                 nic.GetMtu(),
		LinkState:           linkStateToNetworkInterfaceLinkState(nic.GetLinkState()),
		BmcInterface:        nic.GetBmcNet(),
	}
	return nicres, nil
}

// PopulateIPAddressWithIPAddressInfo translates an IPAddress into an IPAddress resource.
func PopulateIPAddressWithIPAddressInfo(ip *pb.IPAddress, hostNic *computev1.HostnicResource) (
	*network_v1.IPAddressResource, error,
) {
	if ip == nil {
		zlog.InfraSec().InfraError("IPAddress cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "IPAddress cannot be nil")
	}
	if hostNic == nil {
		zlog.InfraSec().InfraError("HostNic cannot be nil").Msgf("")
		return nil, errors.Errorfc(codes.InvalidArgument, "HostNic cannot be nil")
	}
	prefix := ip.GetIpAddress() + "/" + strconv.Itoa(int(ip.GetNetworkPrefixBits()))
	_, err := netip.ParsePrefix(prefix)
	if err != nil {
		zlog.InfraSec().InfraError("%s is not a valid CIDR IPAddress", prefix).Msg("")
		return nil, errors.Errorfc(codes.InvalidArgument, "%s is not a valid CIDR IPAddress", prefix)
	}
	configMode, ok := mapIPConfigMode[ip.ConfigMode]
	if !ok {
		configMode = network_v1.IPAddressConfigMethod_IP_ADDRESS_CONFIG_METHOD_UNSPECIFIED
	}
	ipres := &network_v1.IPAddressResource{
		TenantId:     hostNic.GetTenantId(),
		Nic:          hostNic,
		Address:      ip.GetIpAddress() + "/" + strconv.Itoa(int(ip.GetNetworkPrefixBits())),
		Status:       network_v1.IPAddressStatus_IP_ADDRESS_STATUS_CONFIGURED,
		StatusDetail: "IPAddress is configured",
		CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_CONFIGURED,
		ConfigMethod: configMode,
	}
	return ipres, nil
}

// UpdateInstanceResourceStateStatusDetails updates instance resource state status details.
func UpdateInstanceResourceStateStatusDetails(
	in *computev1.InstanceResource,
	state pb.InstanceState,
	status pb.InstanceStatus,
	instanceStatusDetail string,
	instResID string,
) *computev1.InstanceResource {
	in.ResourceId = instResID
	in.CurrentState = instanceStateToInstanceResourceState[state]

	instStatus := GetInstanceStatus(status)
	in.InstanceStatus = instStatus.Status
	in.InstanceStatusIndicator = instStatus.StatusIndicator
	// instance status timestamp updated later by inv client
	in.InstanceStatusDetail = instanceStatusDetail
	return in
}

// IsHostNotProvisioned checks if a host is not provisioned.
func IsHostNotProvisioned(hostres *computev1.HostResource) bool {
	hostInstance := hostres.GetInstance()
	if hostInstance == nil {
		// If a host has no Instance, it's not provisioned yet
		return true
	}
	return hostInstance.ProvisioningStatusIndicator != om_status.ProvisioningStatusDone.StatusIndicator &&
		hostInstance.ProvisioningStatus != om_status.ProvisioningStatusDone.Status
}

// IsHostUntrusted checks if a host is untrusted.
func IsHostUntrusted(hostres *computev1.HostResource) bool {
	// this can mean a state inconsistency if desired state != current state, but for safety we check both.
	// eventually we should only check the desired state,
	// if we're confident that upper layers will handle state inconsistency properly.
	return hostres.GetCurrentState() == computev1.HostState_HOST_STATE_UNTRUSTED ||
		hostres.GetDesiredState() == computev1.HostState_HOST_STATE_UNTRUSTED
}

// IsHostUnderMaintain checks if a host is under maintenance.
func IsHostUnderMaintain(hostres *computev1.HostResource) bool {
	hostInstance := hostres.GetInstance()
	// If the host has no instance, maintenance is not applicable
	if hostInstance == nil {
		return false
	}
	return hostInstance.UpdateStatusIndicator == mm_status.UpdateStatusInProgress.StatusIndicator &&
		hostInstance.UpdateStatus == mm_status.UpdateStatusInProgress.Status
}

// IsSameHost checks if two hosts are the same.
func IsSameHost(
	originalHostres *computev1.HostResource,
	updatedHostres *computev1.HostResource,
	fieldmask *fieldmaskpb.FieldMask,
) (bool, error) {
	// firstly, cloning Host resource to avoid changing its content
	clonedHostres := proto.Clone(originalHostres)
	// with the fieldmask we are filtering out the fields we don't need
	err := util.ValidateMaskAndFilterMessage(clonedHostres, fieldmask, true)
	if err != nil {
		return false, err
	}

	return proto.Equal(clonedHostres, updatedHostres), nil
}

// IsSameHostStatus checks if two host statuses are the same.
func IsSameHostStatus(hostres *computev1.HostResource, status *pb.HostStatus) bool {
	return hostres.GetHostStatus() == GetHostStatus(status.GetHostStatus()).Status
}

// IsSameInstanceStateStatusDetail checks if two instance state status details are the same.
func IsSameInstanceStateStatusDetail(
	in *pb.UpdateInstanceStateStatusByHostGUIDRequest,
	instanceInv *computev1.InstanceResource,
) bool {
	return in.GetInstanceState().String() == instanceInv.GetCurrentState().String() &&
		GetInstanceStatus(in.GetInstanceStatus()).Status == instanceInv.GetInstanceStatus() &&
		in.GetProviderStatusDetail() == instanceInv.InstanceStatusDetail
}

// InstanceStatusToHostStatusMsg converts instance status to host status message.
func InstanceStatusToHostStatusMsg(in *pb.UpdateInstanceStateStatusByHostGUIDRequest) *pb.HostStatus {
	hostStatusEnum, ok := instanceStatusToHostStatus[in.InstanceStatus]
	if !ok {
		hostStatusEnum = pb.HostStatus_UNSPECIFIED
	}
	// Note that we dont have provider status to fill details
	return &pb.HostStatus{
		Details:             "",
		HumanReadableStatus: in.ProviderStatusDetail,
		HostStatus:          hostStatusEnum,
	}
}

// UpdateInstanceStateStatusToUpdateHostStatus converts update instance state status to update host status.
func UpdateInstanceStateStatusToUpdateHostStatus(in *pb.UpdateInstanceStateStatusByHostGUIDRequest) *pb.UpdateHostStatusByHostGuidRequest { //nolint:lll // function signature
	return &pb.UpdateHostStatusByHostGuidRequest{
		HostGuid:   in.HostGuid,
		HostStatus: InstanceStatusToHostStatusMsg(in),
	}
}

// TenantIDResourceIDTuple represents a tuple of tenant ID and resource ID.
type TenantIDResourceIDTuple struct {
	TenantID   string
	ResourceID string
}

func (hbk TenantIDResourceIDTuple) String() string {
	// IsEmpty checks if the TenantIDResourceIDTuple is empty.
	return fmt.Sprintf("[tenantID=%s, resourceID=%s]", hbk.TenantID, hbk.ResourceID)
}

// IsEmpty checks if the TenantIDResourceIDTuple is empty.
func (hbk TenantIDResourceIDTuple) IsEmpty() bool {
	return hbk.TenantID == "" && hbk.ResourceID == ""
}

// NewTenantIDResourceIDTupleFromHost creates a new TenantIDResourceIDTuple from a host resource.
func NewTenantIDResourceIDTupleFromHost(host *computev1.HostResource) TenantIDResourceIDTuple {
	return TenantIDResourceIDTuple{
		TenantID:   host.GetTenantId(),
		ResourceID: host.GetResourceId(),
	}
}

// ProtoEqualSubset compares two proto messages but only compares the specified fields.
// If no fields are specified, it compares the entire messages.
// If the includedFields are not valid, it falls back to a full comparison.
// If the includedFields are not valid for the messages, it falls back to a full comparison.
// TODO: move to inventory shared library.
func ProtoEqualSubset[T proto.Message](a, b T, includedFields ...string) bool {
	// If no fields specified, compare everything
	if len(includedFields) == 0 {
		return proto.Equal(a, b)
	}

	// Clone the messages to avoid modifying the originals
	aClone := proto.Clone(a)
	bClone := proto.Clone(b)

	// Create a fieldmask from the included fields
	mask := &fieldmaskpb.FieldMask{
		Paths: includedFields,
	}

	// Filter both messages to only include the specified fields
	if err := util.ValidateMaskAndFilterMessage(aClone, mask, true); err != nil {
		// Fall back to equality check if filtering fails
		return proto.Equal(a, b)
	}
	if err := util.ValidateMaskAndFilterMessage(bClone, mask, true); err != nil {
		// Fall back to equality check if filtering fails
		return proto.Equal(a, b)
	}

	// Compare the filtered messages
	return proto.Equal(aClone, bClone)
}

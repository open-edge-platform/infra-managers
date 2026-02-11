// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package invclient

import (
	"context"
	"flag"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_status "github.com/open-edge-platform/infra-core/inventory/v2/pkg/status"
	inv_util "github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util/collections"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/host/pkg/errors"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
	util "github.com/open-edge-platform/infra-managers/host/pkg/utils"
)

const (
	// DefaultInventoryTimeout is the default timeout for Inventory operations.
	DefaultInventoryTimeout = 5 * time.Second // Default timeout for Inventory operations
	// ListAllDefaultTimeout The current estimation is very conservative considering 10k resources, batch size 100,
	//  and 600ms per request on average.
	// TODO: fine tune this longer timeout based on target scale and inventory client batch size.
	ListAllDefaultTimeout = time.Minute // Longer timeout for reconciling all resources
)

var (
	zlog = logging.GetLogger("InvClient")

	// InventoryTimeout is the timeout duration for Inventory API calls.
	InventoryTimeout = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
	// ListAllInventoryTimeout is the timeout for listing all inventory items.
	ListAllInventoryTimeout = flag.Duration(
		"timeoutInventoryListAll",
		ListAllDefaultTimeout,
		"Timeout used when listing all resources for a given type from Inventory",
	)

	// UpdateHoststorageFieldMask is the field mask for host storage updates.
	UpdateHoststorageFieldMask = []string{
		computev1.HoststorageResourceFieldKind,
		computev1.HoststorageResourceFieldDeviceName,
		computev1.HoststorageResourceFieldWwid,
		computev1.HoststorageResourceFieldSerial,
		computev1.HoststorageResourceFieldVendor,
		computev1.HoststorageResourceFieldModel,
		computev1.HoststorageResourceFieldCapacityBytes,
	}
	// UpdateHostnicFieldMask is the field mask for host NIC updates.
	UpdateHostnicFieldMask = []string{
		computev1.HostnicResourceFieldKind,
		computev1.HostnicResourceFieldDeviceName,
		computev1.HostnicResourceFieldPciIdentifier,
		computev1.HostnicResourceFieldMacAddr,
		computev1.HostnicResourceFieldSriovEnabled,
		computev1.HostnicResourceFieldSriovVfsNum,
		computev1.HostnicResourceFieldSriovVfsTotal,
		computev1.HostnicResourceFieldPeerName,
		computev1.HostnicResourceFieldPeerDescription,
		computev1.HostnicResourceFieldPeerMac,
		computev1.HostnicResourceFieldPeerMgmtIp,
		computev1.HostnicResourceFieldPeerPort,
		computev1.HostnicResourceFieldSupportedLinkMode,
		computev1.HostnicResourceFieldAdvertisingLinkMode,
		computev1.HostnicResourceFieldCurrentSpeedBps,
		computev1.HostnicResourceFieldCurrentDuplex,
		computev1.HostnicResourceFieldFeatures,
		computev1.HostnicResourceFieldMtu,
		computev1.HostnicResourceFieldLinkState,
		computev1.HostnicResourceFieldBmcInterface,
	}
	// UpdateIPAddressFieldMask is the field mask for IP address updates.
	UpdateIPAddressFieldMask = []string{
		network_v1.IPAddressResourceFieldAddress,
		network_v1.IPAddressResourceFieldConfigMethod,
		network_v1.IPAddressResourceFieldStatus,
		network_v1.IPAddressResourceFieldStatusDetail,
		network_v1.IPAddressResourceFieldCurrentState,
	}
	// UpdateHostusbFieldMask is the field mask for host USB updates.
	UpdateHostusbFieldMask = []string{
		computev1.HostusbResourceFieldKind,
		computev1.HostusbResourceFieldDeviceName,
		computev1.HostusbResourceFieldIdvendor,
		computev1.HostusbResourceFieldIdproduct,
		computev1.HostusbResourceFieldBus,
		computev1.HostusbResourceFieldAddr,
		computev1.HostusbResourceFieldClass,
		computev1.HostusbResourceFieldSerial,
	}
	// UpdateHostgpuFieldMask is the field mask for host GPU updates.
	UpdateHostgpuFieldMask = []string{
		computev1.HostgpuResourceFieldPciId,
		computev1.HostgpuResourceFieldProduct,
		computev1.HostgpuResourceFieldVendor,
		computev1.HostgpuResourceFieldDescription,
		computev1.HostgpuResourceFieldDeviceName,
		computev1.HostgpuResourceFieldFeatures,
	}
)

// List resources by the provided filter. Filter is done only on fields that are set (not default values of the
// resources). Note that this function will NOT return an error if an object is not found.
// No need to specify tenantID, if required will be provided in the filter.
func listAllResources(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	filter *inv_v1.ResourceFilter,
) ([]*inv_v1.Resource, error) {
	ctx, cancel := context.WithTimeout(ctx, *ListAllInventoryTimeout)
	defer cancel()
	// we agreed to not return a NotFound error to avoid too many 'Not Found'
	// responses to the consumer of our external APIs.
	objs, err := c.ListAll(ctx, filter)
	if err != nil && !inv_errors.IsNotFound(err) {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to listAll %v", filter)
		return nil, err
	}
	for _, v := range objs {
		if err = validator.ValidateMessage(v); err != nil {
			zlog.InfraSec().InfraErr(err).Msgf("Invalid input, validation has failed: %v", v)
			return nil, inv_errors.Wrap(err)
		}
	}
	return objs, nil
}

// UpdateInvResourceFields updates selected fields of a resource in Inventory.
// The resource object can contain any fields, but only the selected fields will be overwritten in Inventory
// (also if they are empty), so take care to always fill expected values for fields that will be updated.
// This function doesn't modify the resource object (instead creates a deep copy that is further modified).
func UpdateInvResourceFields(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, resource proto.Message, fields []string,
) error {
	if resource == nil {
		err := inv_errors.Errorfc(codes.InvalidArgument, "no resource provided")
		zlog.InfraSec().InfraErr(err).Msg("Empty resource is provided")
		return err
	}

	if len(fields) == 0 {
		zlog.InfraSec().Debug().
			Msgf("Skipping, no fields selected to update for an inventory resource: %v, tenantID=%s", resource, tenantID)
		return nil
	}

	resCopy := proto.Clone(resource)

	invResource, invResourceID, err := getInventoryResourceAndID(resCopy)
	if err != nil {
		return err
	}

	fieldMask, err := fieldmaskpb.New(resCopy, fields...)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Failed to construct a fieldmask")
		return errors.Wrap(err)
	}

	err = inv_util.ValidateMaskAndFilterMessage(resCopy, fieldMask, true)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Failed to validate a fieldmask and filter message")
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()

	_, err = c.Update(ctx, tenantID, invResourceID, fieldMask, invResource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to update resource (%s) for tenantID=%s", invResourceID, tenantID)
		return err
	}
	return nil
}

// GetHostResourceByGUID retrieves a host resource by its GUID.
func GetHostResourceByGUID(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID string,
	guid string,
) (*computev1.HostResource, error) {
	zlog.Debug().Msgf("Obtaining Host resource by its GUID (tID=%s, UUID=%s)", tenantID, guid)
	if guid == "" {
		err := inv_errors.Errorfc(codes.InvalidArgument, "Empty GUID")
		zlog.InfraSec().InfraErr(err).Msg("Empty GUID obtained at the input of the function")
		return nil, err
	}
	return c.GetHostByUUID(ctx, tenantID, guid)
}

// GetHostResourceByResourceID retrieves a host resource by its resource ID.
func GetHostResourceByResourceID(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string,
) (*computev1.HostResource, error) {
	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()

	getresresp, err := c.Get(ctx, tenantID, resourceID)
	if err != nil {
		return nil, err
	}

	hostres := getresresp.GetResource().GetHost()

	if validateErr := validator.ValidateMessage(hostres); validateErr != nil {
		zlog.InfraSec().Err(validateErr).Msgf("Failed to validate host resource: %v", hostres)
		return nil, inv_errors.Wrap(validateErr)
	}

	return hostres, nil
}

// CreateHostusb creates a new Hoststusb resource in Inventory.
func CreateHostusb(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostusb *computev1.HostusbResource,
) (string, error) {
	details := fmt.Sprintf("tenantID=%s, hostUSB=%v", tenantID, hostusb)

	zlog.Debug().Msgf("Create Hostusb: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Hostusb{
			Hostusb: hostusb,
		},
	}
	resp, err := c.Create(ctx, tenantID, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed create hostusb resource: tenantID=%s, hostUSB=%v", tenantID, hostusb)
		return "", err
	}
	return inv_util.GetResourceIDFromResource(resp)
}

// UpdateHostusb updates an existing Hostusb resource info in Inventory,
// except state and other fields not allowed from RM.
func UpdateHostusb(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostusb *computev1.HostusbResource,
) error {
	details := fmt.Sprintf("tenantID=%s, hostUSB=%v", tenantID, hostusb)

	zlog.Debug().Msgf("Update Hostusb: %s", details)

	err := UpdateInvResourceFields(ctx, c, tenantID, hostusb, UpdateHostusbFieldMask)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed update Hostusb resource: %s", details)
		return err
	}
	return nil
}

// DeleteHostusb deletes an existing Hostusb resource in Inventory. If it gets a not found error while deleting the given
// resource, it doesn't return an error.
//
//nolint:dupl // Protobuf oneOf-driven separation
func DeleteHostusb(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string) error {
	details := fmt.Sprintf("tenantID=%s, resourceID=%s", tenantID, resourceID)
	zlog.Debug().Msgf("Delete Hostusb: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()

	_, err := c.Delete(ctx, tenantID, resourceID)
	if inv_errors.IsNotFound(err) {
		zlog.Debug().Msgf("Not found while HostUSB delete, dropping err: %s", details)
		return nil
	}
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed delete Hostusb resource: %s", details)
		return err
	}

	return err
}

// CreateHostgpu creates a new host GPU resource in Inventory.
//
//nolint:dupl // Protobuf oneOf-driven separation
func CreateHostgpu(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostgpu *computev1.HostgpuResource) (
	string, error,
) {
	details := fmt.Sprintf("tenantID=%s, hostGPU=%v", tenantID, hostgpu)
	zlog.Debug().Msgf("Create Hostgpu: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Hostgpu{
			Hostgpu: hostgpu,
		},
	}
	resp, err := c.Create(ctx, tenantID, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed create hostgpu resource: %s", details)
		return "", err
	}
	return inv_util.GetResourceIDFromResource(resp)
}

// UpdateHostgpu updates an existing host GPU resource.
func UpdateHostgpu(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostgpu *computev1.HostgpuResource,
) error {
	details := fmt.Sprintf("tenantID=%s, hostGPU=%v", tenantID, hostgpu)
	zlog.Debug().Msgf("Update Hostgpu: %s", details)

	err := UpdateInvResourceFields(ctx, c, tenantID, hostgpu, UpdateHostgpuFieldMask)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed update Hostgpu resource: %s", details)
	}
	return err
}

// DeleteHostgpu deletes a host GPU resource.
//
//nolint:dupl // Protobuf oneOf-driven separation
func DeleteHostgpu(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string) error {
	details := fmt.Sprintf("tenantID=%s, resourceID=%s", tenantID, resourceID)
	zlog.Debug().Msgf("Delete Hostgpu: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	_, err := c.Delete(ctx, tenantID, resourceID)
	if inv_errors.IsNotFound(err) {
		zlog.Debug().Msgf("Not found while HostGPU delete, dropping err: %s", details)
		return nil
	}
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed delete Hostgpu resource: %s", details)
		return err
	}
	return err
}

// UpdateHostStatus updates only the host-related statuses, keeping other fields of HostResource unchanged.
// It takes computev1.HostResource as an argument, but ignores all fields other than host_status, provider_status
// and provider_status_detail. It overwrites host_status, provider_status and provider_status_detail
// with provided values in computev1.HostResource. The function requires ResourceId to be set in computev1.HostResource.
func UpdateHostStatus(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, host *computev1.HostResource,
) error {
	return UpdateInvResourceFields(ctx, c, tenantID, host, []string{
		computev1.HostResourceFieldHostStatus,
		computev1.HostResourceFieldHostStatusIndicator,
		computev1.HostResourceFieldHostStatusTimestamp,
	})
}

// CreateHoststorage creates a new Hoststorage resource in Inventory.
//
//nolint:dupl // Protobuf oneOf-driven separation
func CreateHoststorage(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostStorage *computev1.HoststorageResource,
) (
	string, error,
) {
	details := fmt.Sprintf("tenantID=%s, hostStorage=%v", tenantID, hostStorage)
	zlog.Debug().Msgf("Create Hoststorage: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Hoststorage{
			Hoststorage: hostStorage,
		},
	}
	resp, err := c.Create(ctx, tenantID, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed create Hoststorage resource: %s", details)
		return "", err
	}
	return inv_util.GetResourceIDFromResource(resp)
}

// UpdateHoststorage updates an existing Hoststorage resource info in Inventory,
// except state and other fields not allowed from RM.
func UpdateHoststorage(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string,
	hostStorage *computev1.HoststorageResource,
) error {
	details := fmt.Sprintf("tenantID=%s, hostStorage=%v", tenantID, hostStorage)
	zlog.Debug().Msgf("Update Hoststorage: %s", details)

	err := UpdateInvResourceFields(ctx, c, tenantID, hostStorage, UpdateHoststorageFieldMask)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed update Hoststorage resource: %s", details)
		return err
	}
	return nil
}

// DeleteHoststorage deletes an existing Hoststorage resource in Inventory. If it gets a not found error while deleting the given
// resource, it doesn't return an error.
//
//nolint:dupl // Protobuf oneOf-driven separation
func DeleteHoststorage(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string) error {
	details := fmt.Sprintf("tenantID=%s, resourceID=%s", tenantID, resourceID)
	zlog.Debug().Msgf("Delete Hoststorage: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()

	_, err := c.Delete(ctx, tenantID, resourceID)
	if inv_errors.IsNotFound(err) {
		zlog.Debug().Msgf("Not found while HostStorage delete, dropping err: %s", details)
		return nil
	}
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed delete Hoststorage resource: %s", details)
		return err
	}

	return err
}

// CreateHostnic creates a new Hostnic resource in Inventory.
//
//nolint:dupl // Protobuf oneOf-driven separation
func CreateHostnic(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostNic *computev1.HostnicResource,
) (string, error) {
	details := fmt.Sprintf("tenantID=%s, hostNIC=%v", tenantID, hostNic)
	zlog.Debug().Msgf("Create Hostnic: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Hostnic{
			Hostnic: hostNic,
		},
	}
	resp, err := c.Create(ctx, tenantID, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed create hostNic resource: %s", details)
		return "", err
	}
	return inv_util.GetResourceIDFromResource(resp)
}

// UpdateHostnic updates an existing Hostnic resource info in Inventory, except state and other fields not allowed from RM.
func UpdateHostnic(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostNic *computev1.HostnicResource,
) error {
	details := fmt.Sprintf("tenantID=%s, hostNIC=%v", tenantID, hostNic)
	zlog.Debug().Msgf("Update Hostnic: %s", details)

	err := UpdateInvResourceFields(ctx, c, tenantID, hostNic, UpdateHostnicFieldMask)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed update Hostnic resource: %s", details)
		return err
	}
	return nil
}

// DeleteHostnic deletes an existing Hostnic resource in Inventory. If it gets a not found error while deleting the given
// resource, it doesn't return an error.
//
//nolint:dupl // Protobuf oneOf-driven separation
func DeleteHostnic(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string) error {
	details := fmt.Sprintf("tenantID=%s, resourceID=%s", tenantID, resourceID)
	zlog.Debug().Msgf("Update Hostnic: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	_, err := c.Delete(ctx, tenantID, resourceID)
	if inv_errors.IsNotFound(err) {
		zlog.Debug().Msgf("Not found while HostNIC delete, dropping err: %s", details)
		return nil
	}
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed delete hostNic resource: %s", details)
		return err
	}
	return err
}

// ListIPAddresses returns the list of IP addresses associated to the nic.
func ListIPAddresses(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostNic *computev1.HostnicResource) (
	[]*network_v1.IPAddressResource, error,
) {
	zlog.Debug().Msgf("List IPAddress associated to: tenantID=%s, hostNIC=%v", tenantID, hostNic)

	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Ipaddress{}},
		Filter: fmt.Sprintf("%s.%s = %q AND %s = %q",
			network_v1.IPAddressResourceEdgeNic, computev1.HostnicResourceFieldResourceId, hostNic.GetResourceId(),
			network_v1.IPAddressResourceFieldTenantId, tenantID),
	}
	resources, err := listAllResources(ctx, c, filter)
	if err != nil {
		return nil, err
	}
	return inv_util.GetSpecificResourceList[*network_v1.IPAddressResource](resources)
}

// CreateIPAddress creates a new IP address resource in Inventory.
//
//nolint:dupl // Protobuf oneOf-driven separation
func CreateIPAddress(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostIP *network_v1.IPAddressResource) (
	string, error,
) {
	details := fmt.Sprintf("tenantID=%s, IPAddress=%v", tenantID, hostIP)
	zlog.Debug().Msgf("Create IPAddress: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: hostIP,
		},
	}
	resp, err := c.Create(ctx, tenantID, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed create IPAddress resource: %s", details)
		return "", err
	}
	return inv_util.GetResourceIDFromResource(resp)
}

// UpdateIPAddress updates an existing IP address resource in Inventory, except status/state fields.
func UpdateIPAddress(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID string, hostIP *network_v1.IPAddressResource,
) error {
	details := fmt.Sprintf("tenantID=%s, IPAddress=%v", tenantID, hostIP)
	zlog.Debug().Msgf("Update IPAddress: %s", details)
	err := UpdateInvResourceFields(ctx, c, tenantID, hostIP, UpdateIPAddressFieldMask)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed update IPAddress resource %s", details)
		return err
	}
	return nil
}

// DeleteIPAddress deletes an existing IP address resource in Inventory
// by setting to DELETED the current state of the resource.
func DeleteIPAddress(ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, resourceID string) error {
	details := fmt.Sprintf("tenantID=%s, resourceID=%v", tenantID, resourceID)
	zlog.Debug().Msgf("Delete IPAddress: %s", details)

	ctx, cancel := context.WithTimeout(ctx, *InventoryTimeout)
	defer cancel()
	ipAddress := &network_v1.IPAddressResource{
		ResourceId:   resourceID,
		CurrentState: network_v1.IPAddressState_IP_ADDRESS_STATE_DELETED,
	}

	err := UpdateInvResourceFields(ctx, c, tenantID, ipAddress, []string{network_v1.IPAddressResourceFieldCurrentState})
	if inv_errors.IsNotFound(err) {
		zlog.Debug().Msgf("Not found while IP address delete, dropping err: %s", details)
		return nil
	}
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed delete IPAddress resource: %s", details)
		return err
	}

	return err
}

// SetHostAsConnectionLost marks a host as having lost connection.
func SetHostAsConnectionLost(
	ctx context.Context, c inv_client.TenantAwareInventoryClient, tenantID, hostResourceID string, timeStamp uint64,
) error {
	hostRes, err := GetHostResourceByResourceID(ctx, c, tenantID, hostResourceID)
	if err != nil {
		return err
	}

	// Skip if current status is newer than when we started to try to set the connection lost
	if hostRes.HostStatusTimestamp < timeStamp &&
		(hostRes.GetHostStatus() == hrm_status.HostStatusRunning.Status ||
			hostRes.GetHostStatus() == hrm_status.HostStatusBooting.Status ||
			hostRes.GetHostStatus() == hrm_status.HostStatusError.Status) {
		updateHost := computev1.HostResource{
			ResourceId:          hostRes.GetResourceId(),
			HostStatus:          hrm_status.HostStatusNoConnection.Status,
			HostStatusIndicator: hrm_status.HostStatusNoConnection.StatusIndicator,
			HostStatusTimestamp: timeStamp,
		}
		return UpdateHostStatus(ctx, c, tenantID, &updateHost)
	}

	zlog.InfraSec().InfraError("The status of Host (%s, %s) is %s at %v. Skip heart beat time out event.",
		tenantID, hostResourceID, hostRes.GetHostStatus(), hostRes.HostStatusTimestamp).Msg("SetHostAsConnectionLost")
	return nil
}

// SetHostStatus sets the status of a host.
func SetHostStatus(
	ctx context.Context, c inv_client.TenantAwareInventoryClient,
	tenantID, resourceID string, hostStatus inv_status.ResourceStatus,
) error {
	updateHost := &computev1.HostResource{
		ResourceId:          resourceID,
		HostStatus:          hostStatus.Status,
		HostStatusIndicator: hostStatus.StatusIndicator,
	}
	updateHost.HostStatusTimestamp = uint64(time.Now().Unix())
	return UpdateHostStatus(ctx, c, tenantID, updateHost)
}

// GetHostResources Non-tenant specific function,should be used in bootstrap process of HRM only.
func GetHostResources(ctx context.Context, c inv_client.TenantAwareInventoryClient) (
	hostres []*computev1.HostResource, err error,
) {
	zlog.Debug().Msg("Get all host resources")

	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Host{}},
	}
	resources, err := listAllResources(ctx, c, filter)
	if err != nil {
		return nil, err
	}

	resList, err := inv_util.GetSpecificResourceList[*computev1.HostResource](resources)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Failed to get a Host resource list")
		return nil, err
	}
	return resList, nil
}

// FindAllTrustedHosts Non-tenant specific function, should be used in reconcileAll process of HRM only.
func FindAllTrustedHosts(ctx context.Context, c inv_client.TenantAwareInventoryClient) ([]util.TenantIDResourceIDTuple, error) {
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{
			Resource: &inv_v1.Resource_Host{},
		},
		Filter: fmt.Sprintf("%s != %s",
			computev1.HostResourceFieldCurrentState,
			computev1.HostState_HOST_STATE_UNTRUSTED),
	}

	resources, err := c.FindAll(ctx, filter)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("Failed to find all trusted hosts")
		return nil, err
	}
	return collections.MapSlice[*inv_v1.FindResourcesResponse_ResourceTenantIDCarrier, util.TenantIDResourceIDTuple](
		resources, func(resource *inv_v1.FindResourcesResponse_ResourceTenantIDCarrier) util.TenantIDResourceIDTuple {
			return util.TenantIDResourceIDTuple{
				TenantID:   resource.GetTenantId(),
				ResourceID: resource.GetResourceId(),
			}
		}), nil
}

// UpdateInstanceStateStatusByHostGUID updates Instance's Current State, Status
// and Status Detail.
func UpdateInstanceStateStatusByHostGUID(
	ctx context.Context,
	c inv_client.TenantAwareInventoryClient,
	tenantID, instResID string,
	instRes *computev1.InstanceResource,
) error {
	zlog.Debug().Msgf("Updating Instance (%s) for tenantID=%s state, status and status detail", instResID, tenantID)

	instRes.InstanceStatusTimestamp = uint64(time.Now().Unix())
	// Handcrafted PATCH update and validate before sending to Inventory
	fieldMask := &fieldmaskpb.FieldMask{
		Paths: []string{
			computev1.InstanceResourceFieldCurrentState,
			computev1.InstanceResourceFieldInstanceStatus,
			computev1.InstanceResourceFieldInstanceStatusIndicator,
			computev1.InstanceResourceFieldInstanceStatusTimestamp,
			computev1.InstanceResourceFieldInstanceStatusDetail,
		},
	}
	err := inv_util.ValidateMaskAndFilterMessage(instRes, fieldMask, true)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to validate mask and filter message accordingly for Instance resource")
		return err
	}
	instRes.ResourceId = instResID

	// making a call to update gRPC
	_, err = c.Update(ctx, tenantID, instResID, fieldMask, &inv_v1.Resource{
		Resource: &inv_v1.Resource_Instance{
			Instance: instRes,
		},
	})
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to update Instance resource in Inventory")
		return err
	}

	return nil
}

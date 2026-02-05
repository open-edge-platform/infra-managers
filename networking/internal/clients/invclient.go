// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package clients provides gRPC client implementations for inventory services.
package clients

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	compute_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	location_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/location/v1"
	network_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/network/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/networking/internal/utils"
)

const (
	// DefaultInventoryTimeout is the default timeout for inventory operations.
	DefaultInventoryTimeout = 5 * time.Second
	batchSize               = 20

	// ListAllDefaultTimeout is the default timeout for list all operations.
	ListAllDefaultTimeout = time.Minute // Longer timeout for reconciling all resources
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

var (
	clientName = "NetInventoryClient"
	zlog       = logging.GetLogger(clientName)

	inventoryTimeout = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
	// ListAllInventoryTimeout is the timeout used for list all inventory operations.
	ListAllInventoryTimeout = flag.Duration(
		"timeoutInventoryListAll",
		ListAllDefaultTimeout,
		"Timeout used when listing all resources for a given type from Inventory",
	)
)

// NetInventoryClient implements the methods to interact with the inventory gRPC server for the networking manager.
type NetInventoryClient struct {
	Client  client.TenantAwareInventoryClient
	Watcher chan *client.WatchEvents
}

// Options is options for init of the Inventory client.
type Options struct {
	InventoryAddress string
	EnableTracing    bool
	EnableMetrics    bool
}

// Option is an Inventory client option.
type Option func(*Options)

// WithInventoryAddress sets the Inventory Address.
func WithInventoryAddress(invAddr string) Option {
	return func(options *Options) {
		options.InventoryAddress = invAddr
	}
}

// WithEnableTracing enable tracing.
func WithEnableTracing(enableTracing bool) Option {
	return func(options *Options) {
		options.EnableTracing = enableTracing
	}
}

// WithEnableMetrics returns an option to enable or disable metrics collection.
func WithEnableMetrics(enableMetrics bool) Option {
	return func(options *Options) {
		options.EnableMetrics = enableMetrics
	}
}

// WithOptions sets the Inventory client options.
func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

// NewNetInventoryClientWithOptions creates a client by instantiating a new Inventory client. To be used in production.
func NewNetInventoryClientWithOptions(opts ...Option) (*NetInventoryClient, error) {
	// Misc preps for the client instantiation
	ctx := context.Background()
	var options Options
	for _, opt := range opts {
		opt(&options)
	}
	eventsWatcher := make(chan *client.WatchEvents, eventsWatcherBufSize)
	wg := sync.WaitGroup{}
	clientCfg := client.InventoryClientConfig{
		Name:                      clientName,
		Address:                   options.InventoryAddress,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		SecurityCfg: &client.SecurityConfig{
			Insecure: true,
			CaPath:   "",
			CertPath: "",
			KeyPath:  "",
		},
		Events:     eventsWatcher,
		ClientKind: inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		ResourceKinds: []inv_v1.ResourceKind{
			inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS,
		},
		Wg:            &wg,
		EnableTracing: options.EnableTracing,
		EnableMetrics: options.EnableMetrics,
	}
	invClient, err := client.NewTenantAwareInventoryClient(ctx, clientCfg)
	if err != nil {
		return nil, err
	}
	zlog.InfraSec().Info().Msgf("Inventory client started")
	return NewNetInventoryClient(invClient, eventsWatcher)
}

// NewNetInventoryClient creates a client that wraps an existing Inventory client. Mainly for testing.
func NewNetInventoryClient(invClient client.TenantAwareInventoryClient, watcher chan *client.WatchEvents) (
	*NetInventoryClient, error,
) {
	netCl := &NetInventoryClient{
		Client:  invClient,
		Watcher: watcher,
	}
	return netCl, nil
}

// Stop stops the client.
func (n *NetInventoryClient) Stop() {
	if err := n.Client.Close(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("")
	}
	zlog.InfraSec().Info().Msgf("Inventory client stopped")
}

// GetIPAddress retrieves the IP address from Inventory.
func (n *NetInventoryClient) GetIPAddress(ctx context.Context, tenantID, resourceID string) (
	*network_v1.IPAddressResource, error,
) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	// Get the resource and validate
	resp, err := n.Client.Get(ctx, tenantID, resourceID)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to get IPAddress: %s", utils.FormatTenantResourceID(tenantID, resourceID))
		return nil, err
	}
	ipAddress := resp.GetResource().GetIpaddress()
	if err = validator.ValidateMessage(ipAddress); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, inv_errors.Wrap(err)
	}
	return ipAddress, nil
}

// GetIPAddressesInSite retrieves the IP address from Inventory associated to a given site.
// Site can be undefined; meaning a query for all addresses not yet onboarded.
func (n *NetInventoryClient) GetIPAddressesInSite(ctx context.Context, tenantID string, ipAddress *network_v1.IPAddressResource,
) ([]*network_v1.IPAddressResource, error) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	f := fmt.Sprintf("%s = %q AND %s = %q",
		network_v1.IPAddressResourceFieldTenantId, tenantID,
		network_v1.IPAddressResourceFieldAddress, ipAddress.GetAddress())
	// Extend filter string based on embedded resources present in IP address resource.
	if ipAddress.GetNic() != nil {
		f = fmt.Sprintf("%v AND %s.%s = %q AND %s.%s = %q",
			f,
			network_v1.IPAddressResourceEdgeNic, compute_v1.HostnicResourceFieldTenantId,
			tenantID,
			network_v1.IPAddressResourceEdgeNic, compute_v1.HostnicResourceFieldResourceId,
			ipAddress.GetNic().GetResourceId(),
		)
		if ipAddress.GetNic().GetHost().GetSite() != nil {
			f = fmt.Sprintf("%v AND %s.%s.%s.%s = %q AND %s.%s.%s.%s = %q",
				f,
				network_v1.IPAddressResourceEdgeNic, compute_v1.HostnicResourceEdgeHost,
				compute_v1.HostResourceEdgeSite, location_v1.SiteResourceFieldTenantId,
				tenantID,
				network_v1.IPAddressResourceEdgeNic, compute_v1.HostnicResourceEdgeHost,
				compute_v1.HostResourceEdgeSite, location_v1.SiteResourceFieldResourceId,
				ipAddress.GetNic().GetHost().GetSite().GetResourceId(),
			)
		}
	}
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Ipaddress{}},
		Filter:   f,
	}
	// Get the resource and validate
	resources, err := n.Client.ListAll(ctx, filter)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to list All IPAddress in Site %v", ipAddress)
		return nil, err
	}
	for _, v := range resources {
		if err = validator.ValidateMessage(v); err != nil {
			zlog.InfraSec().InfraErr(err).Msg("")
			return nil, inv_errors.Wrap(err)
		}
	}
	return util.GetSpecificResourceList[*network_v1.IPAddressResource](resources)
}

// FindIPAddresses finds existing IP addresses in Inventory.
func (n *NetInventoryClient) FindIPAddresses(ctx context.Context) ([]*client.ResourceTenantIDCarrier, error) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	res, err := util.GetResourceFromKind(inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS)
	if err != nil {
		return nil, err
	}
	filter := &inv_v1.ResourceFilter{
		Resource: res,
	}
	ips, err := n.Client.FindAll(ctx, filter)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to find all IPAddresses")
		return nil, err
	}
	return ips, nil
}

// UpdateIPAddressStatusAndState updates an existing IPAddress status, status_detail and current state in Inventory.
func (n *NetInventoryClient) UpdateIPAddressStatusAndState(ctx context.Context, tenantID, resourceID string,
	ipAddress *network_v1.IPAddressResource,
) error {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	// Handcrafted PATCH update and validate before sending to Inventory
	fieldMask := &fieldmaskpb.FieldMask{
		Paths: []string{
			network_v1.IPAddressResourceFieldStatus,
			network_v1.IPAddressResourceFieldStatusDetail,
			network_v1.IPAddressResourceFieldCurrentState,
		},
	}
	err := util.ValidateMaskAndFilterMessage(ipAddress, fieldMask, true)
	if err != nil {
		return err
	}
	ipAddress.ResourceId = resourceID
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Ipaddress{
			Ipaddress: ipAddress,
		},
	}
	_, err = n.Client.Update(ctx, tenantID, ipAddress.GetResourceId(), fieldMask, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to update IPAddress tenantID=%s, resourceID=%s, Address=%s",
			tenantID, ipAddress.GetResourceId(), ipAddress.GetAddress())
		return err
	}
	return nil
}

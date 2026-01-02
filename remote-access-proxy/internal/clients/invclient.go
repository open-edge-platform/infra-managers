// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package clients

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/open-edge-platform/infra-managers/remote-access-proxy/internal/utils"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

const (
	DefaultInventoryTimeout = 5 * time.Second
	batchSize               = 20

	// TODO: fine tune this longer timeout based on target scale and inventory client batch size.
	ListAllDefaultTimeout = time.Minute // Longer timeout for reconciling all resources
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

var (
	clientName = "RmtAccessInventoryClient"
	zlog       = logging.GetLogger(clientName)

	inventoryTimeout        = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
	ListAllInventoryTimeout = flag.Duration(
		"timeoutInventoryListAll",
		ListAllDefaultTimeout,
		"Timeout used when listing all resources for a given type from Inventory",
	)
)

type RmtAccessInventoryClient struct {
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

// NewRAInventoryClientWithOptions creates a client by instantiating a new Inventory client. To be used in production.
func NewRAInventoryClientWithOptions(opts ...Option) (*RmtAccessInventoryClient, error) {
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
			inv_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF,
		},
		Wg:            &wg,
		EnableTracing: options.EnableTracing,
		EnableMetrics: options.EnableMetrics,
		//ClientCache: client.InvClientCacheConfig{
		//	EnableUUIDCache: true,
		//},
	}
	invClient, err := client.NewTenantAwareInventoryClient(ctx, clientCfg)
	if err != nil {
		return nil, err
	}
	zlog.InfraSec().Info().Msgf("Inventory client started")
	return NewRAInventoryClient(invClient, eventsWatcher)
}

// NewRAInventoryClient creates a client that wraps an existing Inventory client. Mainly for testing.
func NewRAInventoryClient(
	invClient client.TenantAwareInventoryClient,
	watcher chan *client.WatchEvents) (
	*RmtAccessInventoryClient, error,
) {
	rmtAccessCl := &RmtAccessInventoryClient{
		Client:  invClient,
		Watcher: watcher,
	}
	return rmtAccessCl, nil
}

// Stop stops the client.
func (n *RmtAccessInventoryClient) Stop() {
	if err := n.Client.Close(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("")
	}
	zlog.InfraSec().Info().Msgf("Inventory client stopped")
}

func (n *RmtAccessInventoryClient) GetRemoteAccessConf(ctx context.Context, tenantID, resourceID string) (
	*remoteaccessv1.RemoteAccessConfiguration, error) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	// Get the resource and validate
	resp, err := n.Client.Get(ctx, tenantID, resourceID)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to get RmtAccessConfig: %s", utils.FormatTenantResourceID(tenantID, resourceID))
		return nil, err
	}
	remAccessConf := resp.GetResource().GetRemoteAccess()
	if err = validator.ValidateMessage(remAccessConf); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return nil, inv_errors.Wrap(err)
	}
	return remAccessConf, nil
}

// UpdateRemoteAccessConfigState updates an existing  Remote Access Config desired and current state in Inventory.
func (n *RmtAccessInventoryClient) UpdateRemoteAccessConfigState(ctx context.Context, tenantID, resourceID string,
	remAccessConf *remoteaccessv1.RemoteAccessConfiguration,
) error {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	// Handcrafted PATCH update and validate before sending to Inventory
	fieldMask := &fieldmaskpb.FieldMask{
		Paths: []string{
			remoteaccessv1.RemoteAccessConfigurationFieldConfigurationStatus,
			remoteaccessv1.RemoteAccessConfigurationFieldConfigurationStatusTimestamp,
		},
	}
	err := util.ValidateMaskAndFilterMessage(remAccessConf, fieldMask, true)
	if err != nil {
		return err
	}
	remAccessConf.ResourceId = resourceID
	resource := &inv_v1.Resource{
		Resource: &inv_v1.Resource_RemoteAccess{
			RemoteAccess: remAccessConf,
		},
	}
	_, err = n.Client.Update(ctx, tenantID, remAccessConf.GetResourceId(), fieldMask, resource)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to update Remote Access Config tenantID=%s, resourceID=%s, UUID=%s",
			tenantID, remAccessConf.GetResourceId(), remAccessConf.GetInstance().GetHost().GetUuid())
		return err
	}
	return nil
}

// FindRemoteAccessConfigs finds existing Remote Access Configs in Inventory.
func (n *RmtAccessInventoryClient) FindRemoteAccessConfigs(ctx context.Context) ([]*client.ResourceTenantIDCarrier, error) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	res, err := util.GetResourceFromKind(inv_v1.ResourceKind_RESOURCE_KIND_RMT_ACCESS_CONF)
	if err != nil {
		return nil, err
	}
	filter := &inv_v1.ResourceFilter{
		Resource: res,
	}
	rmtAccessCfgs, err := n.Client.FindAll(ctx, filter)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to find all RemoteAccessConfigurations")
		return nil, err
	}
	return rmtAccessCfgs, nil
}

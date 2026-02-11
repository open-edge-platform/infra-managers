// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package invclient provides inventory client for telemetry manager.
package invclient

import (
	"context"
	"flag"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util/collections"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
)

const (
	// DefaultInventoryTimeout is the default timeout for inventory operations.
	DefaultInventoryTimeout = 5 * time.Second
	// ScaleFactor is the scale factor for inventory operations.
	ScaleFactor = 5
)

var (
	clientName = "TelemetryInventoryClient"
	zlog       = logging.GetLogger(clientName)

	inventoryTimeout = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

// TelemetryInventoryClient provides methods to interact with the inventory service.
type TelemetryInventoryClient struct {
	Client         client.TenantAwareInventoryClient
	TelemetryCache *TelemetryCache
	Watcher        chan *client.WatchEvents
}

// Options contains configuration options for the telemetry inventory client.
type Options struct {
	InventoryAddress   string
	EnableTracing      bool
	EnableUUIDCache    bool
	UUIDCacheTTL       time.Duration
	UUIDCacheTTLOffset uint
	DialOptions        []grpc.DialOption
	EnableMetrics      bool
}

// Option is a functional option for configuring the telemetry inventory client.
type Option func(*Options)

// WithInventoryAddress sets the Inventory Address.
func WithInventoryAddress(invAddr string) Option {
	return func(options *Options) {
		options.InventoryAddress = invAddr
	}
}

// WithEnableTracing enables tracing.
func WithEnableTracing(enableTracing bool) Option {
	return func(options *Options) {
		options.EnableTracing = enableTracing
	}
}

// WithEnableUUIDCache enables UUID cache.
func WithEnableUUIDCache(enableUUIDCache bool) Option {
	return func(options *Options) {
		options.EnableUUIDCache = enableUUIDCache
	}
}

// WithUUIDCacheTTL sets the UUID cache TTL.
func WithUUIDCacheTTL(uuidCacheTTL time.Duration) Option {
	return func(options *Options) {
		options.UUIDCacheTTL = uuidCacheTTL
	}
}

// WithUUIDCacheTTLOffset sets the UUID cache TTL offset percentage.
func WithUUIDCacheTTLOffset(uuidCacheTTLOffset uint) Option {
	return func(options *Options) {
		options.UUIDCacheTTLOffset = uuidCacheTTLOffset
	}
}

// WithEnableMetrics enables client-side gRPC metrics.
func WithEnableMetrics(enableMetrics bool) Option {
	return func(options *Options) {
		options.EnableMetrics = enableMetrics
	}
}

// WithDialOption adds a gRPC dial option.
func WithDialOption(dialOption grpc.DialOption) Option {
	return func(options *Options) {
		options.DialOptions = append(options.DialOptions, dialOption)
	}
}

// NewTelemetryInventoryClientWithOptions creates a client by instantiating a new Inventory client.
func NewTelemetryInventoryClientWithOptions(wg *sync.WaitGroup, opts ...Option) (*TelemetryInventoryClient, error) {
	ctx := context.Background()
	var options Options
	for _, opt := range opts {
		opt(&options)
	}

	eventsWatcher := make(chan *client.WatchEvents, eventsWatcherBufSize)
	cfg := client.InventoryClientConfig{
		Name:                      clientName,
		Address:                   options.InventoryAddress,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		// TODO: add security credentials
		SecurityCfg: &client.SecurityConfig{
			Insecure: true,
			CaPath:   "",
			CertPath: "",
			KeyPath:  "",
		},
		Events:     eventsWatcher,
		ClientKind: inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		ResourceKinds: []inv_v1.ResourceKind{
			inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE,
		},
		Wg:            wg,
		EnableTracing: options.EnableTracing,
		EnableMetrics: options.EnableMetrics,
		ClientCache: client.InvClientCacheConfig{
			EnableUUIDCache: options.EnableUUIDCache,
			StaleTime:       options.UUIDCacheTTL,
			StateTimeOffset: int(options.UUIDCacheTTLOffset),
		},
		DialOptions: options.DialOptions,
	}

	invClient, err := client.NewTenantAwareInventoryClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Same options are used for Telemetry cache
	cacheClient, err := NewTelemetryCacheClientWithOptions(ctx, opts...)
	if err != nil {
		return nil, err
	}

	zlog.InfraSec().Info().Msgf("Inventory client started")
	return NewTelemetryInventoryClient(invClient, cacheClient, eventsWatcher)
}

// NewTelemetryInventoryClient creates a new telemetry inventory client.
func NewTelemetryInventoryClient(
	invClient client.TenantAwareInventoryClient, cacheClient *TelemetryCache, watcher chan *client.WatchEvents,
) (*TelemetryInventoryClient, error) {
	cli := &TelemetryInventoryClient{
		Client:         invClient,
		Watcher:        watcher,
		TelemetryCache: cacheClient,
	}
	return cli, nil
}

// Close closes the telemetry inventory client connection.
func (c *TelemetryInventoryClient) Close() {
	if err := c.Client.Close(); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("")
	}
	zlog.InfraSec().Info().Msgf("Inventory client stopped")
}

// GetHostAndInstanceIDResourceByHostUUID retrieves host and instance ID by host UUID.
func (c *TelemetryInventoryClient) GetHostAndInstanceIDResourceByHostUUID(ctx context.Context, tenantID, hostUUID string) (
	hostID, instanceID string,
	err error,
) {
	zlog.Debug().Msgf("GetHostAndInstanceIDResourceByHostUUID: tenantID=%s, HostUUID=%s", tenantID, hostUUID)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	host, err := c.Client.GetHostByUUID(childCtx, tenantID, hostUUID)
	if err != nil {
		return "", "", err
	}

	if err = validator.ValidateMessage(host); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", "", inv_errors.Wrap(err)
	}

	if host.GetInstance() == nil {
		return "", "", inv_errors.Errorfc(
			codes.NotFound, "Instance not found for tenantID=%s, hostID=%s", tenantID, host.GetResourceId())
	}

	return host.GetResourceId(), host.GetInstance().GetResourceId(), nil
}

// ListTelemetryProfilesByHostAndInstanceID lists telemetry profiles by host and instance ID.
func (c *TelemetryInventoryClient) ListTelemetryProfilesByHostAndInstanceID(
	ctx context.Context, tenantID, hostID, instanceID string,
) ([]*telemetryv1.TelemetryProfile, error) {
	treeResIDs, err := c.getHostHierarchy(ctx, tenantID, hostID)
	if err != nil {
		return nil, err
	}
	treeResIDs = append(treeResIDs, instanceID)

	var telemetryProfiles []*telemetryv1.TelemetryProfile
	collections.ForEach[string](treeResIDs, func(resID string) {
		telemetryProfiles = append(
			telemetryProfiles,
			c.TelemetryCache.ListTelemetryProfileByRelation(tenantID, resID)...,
		)
	})
	return telemetryProfiles, nil
}

func (c *TelemetryInventoryClient) getHostHierarchy(
	ctx context.Context,
	tenantID, hostID string,
) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()
	tree, err := c.Client.GetTreeHierarchy(
		ctx,
		&inv_v1.GetTreeHierarchyRequest{
			TenantId:   tenantID,
			Filter:     []string{hostID},
			Descending: true, // Returning root to leaf, so we can later append the instance ID as leaf
		},
	)
	if err != nil {
		zlog.Err(err).Msgf("error while querying hierarchy")
		return nil, err
	}
	hResIDs := make([]string, 0, len(tree))
	collections.ForEach[*inv_v1.GetTreeHierarchyResponse_TreeNode](
		tree,
		func(node *inv_v1.GetTreeHierarchyResponse_TreeNode) {
			switch node.GetCurrentNode().GetResourceKind() {
			case inv_v1.ResourceKind_RESOURCE_KIND_REGION, inv_v1.ResourceKind_RESOURCE_KIND_SITE:
				hResIDs = append(hResIDs, node.GetCurrentNode().GetResourceId())
			case inv_v1.ResourceKind_RESOURCE_KIND_HOST:
				// silently drop hosts, we care about instance not host, and instance ID is known in the caller
			default:
				zlog.Debug().Msgf("skipping resource %v when parsing tree hierarchy", node.GetCurrentNode().GetResourceKind())
			}
		})
	return hResIDs, nil
}

// ListAllTelemetryProfile gets all profiles, across tenants. Should be used carefully, only when validating TPs on startup.
func (c *TelemetryInventoryClient) ListAllTelemetryProfile() ([]*telemetryv1.TelemetryProfile, error) {
	zlog.Debug().Msgf("List All Telemetry Profile.")

	ctxChild, cancel := context.WithTimeout(context.Background(), ScaleFactor*(*inventoryTimeout))
	defer cancel()

	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_TelemetryProfile{}},
		Filter:   "",
	}

	objs, err := c.Client.ListAll(ctxChild, filter)
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

	var profileList []*telemetryv1.TelemetryProfile

	for i := 0; i < len(objs); i++ {
		if err = validator.ValidateMessage(objs[i].GetTelemetryProfile()); err != nil {
			zlog.InfraSec().InfraErr(err).Msgf("Fail to validate profile.")
			return nil, inv_errors.Wrap(err)
		}
		profileList = append(profileList, objs[i].GetTelemetryProfile())
	}

	return profileList, nil
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package invclient

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	osv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/os/v1"
	providerv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/provider/v1"
	tenant_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/tenant/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
)

var (
	zlog = logging.GetLogger("OSRMInvclient")

	InventoryTimeout        = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
	ListAllInventoryTimeout = flag.Duration("timeoutInventoryListAll", ListAllDefaultTimeout,
		"Timeout used when listing all resources for a given type from Inventory")
)

const (
	DefaultInventoryTimeout = 5 * time.Second
	ListAllDefaultTimeout   = time.Minute // Longer timeout for reconciling all resources
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

type InventoryClient struct {
	Client   inv_client.TenantAwareInventoryClient
	Watcher  chan *inv_client.WatchEvents
	termChan chan bool
}

func NewInventoryClient(
	termChan chan bool,
	wg *sync.WaitGroup,
	enableTracing bool,
	invsvcaddr, caCertPath, tlsKeyPath, tlsCertPath string,
	insecureGrpc bool, enableMetrics bool,
) (*InventoryClient, error) {
	zlog.InfraSec().Info().Msgf("Starting Inventory client. invAddress=%s", invsvcaddr)

	ctx := context.Background()
	kinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_TENANT,
	}

	events := make(chan *inv_client.WatchEvents, eventsWatcherBufSize)
	cfg := inv_client.InventoryClientConfig{
		Name:                      "osrm_clientsrv",
		Address:                   invsvcaddr,
		Events:                    events,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		// TODO: Use dedicated client type: ITEP-14459
		ClientKind:    inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		ResourceKinds: kinds,
		EnableTracing: enableTracing,
		EnableMetrics: enableMetrics,
		Wg:            wg,
		SecurityCfg: &inv_client.SecurityConfig{
			CaPath:   caCertPath,
			KeyPath:  tlsKeyPath,
			CertPath: tlsCertPath,
			Insecure: insecureGrpc,
		},
	}
	invClient, err := inv_client.NewTenantAwareInventoryClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return NewOSRMInventoryClient(invClient, events, termChan)
}

func NewOSRMInventoryClient(invClient inv_client.TenantAwareInventoryClient, watcher chan *inv_client.WatchEvents,
	termChan chan bool,
) (*InventoryClient, error) {
	c := &InventoryClient{
		Client:   invClient,
		Watcher:  watcher,
		termChan: termChan,
	}
	return c, nil
}

func (c *InventoryClient) Close() {
	zlog.InfraSec().Info().Msg("Stopping Inventory client")
	err := c.Client.Close()
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
	}
}

func (c *InventoryClient) getResourceByID(ctx context.Context, tenantID, resourceID string) (*inv_v1.GetResourceResponse, error) {
	getresresp, err := c.Client.Get(ctx, tenantID, resourceID)
	if err != nil {
		return nil, err
	}

	return getresresp, nil
}

func (c *InventoryClient) listAllResources(
	ctx context.Context,
	filter *inv_v1.ResourceFilter,
) ([]*inv_v1.Resource, error) {
	ctx, cancel := context.WithTimeout(ctx, *ListAllInventoryTimeout)
	defer cancel()
	// we agreed to not return a NotFound error to avoid too many 'Not Found'
	// responses to the consumer of our external APIs.
	objs, err := c.Client.ListAll(ctx, filter)
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

func (c *InventoryClient) GetTenantByResourceID(ctx context.Context,
	tenantID string,
	resourceID string,
) (*tenant_v1.Tenant, error) {
	resp, err := c.getResourceByID(ctx, tenantID, resourceID)
	if err != nil {
		return nil, err
	}

	tenant := resp.GetResource().GetTenant()

	if validateErr := validator.ValidateMessage(tenant); validateErr != nil {
		return nil, inv_errors.Wrap(validateErr)
	}

	return tenant, nil
}

func (c *InventoryClient) ListOSResourcesForTenant(ctx context.Context,
	tenantID string,
) ([]*osv1.OperatingSystemResource, error) {
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Os{}},
		Filter: fmt.Sprintf(`%s = %q`,
			osv1.OperatingSystemResourceFieldTenantId,
			tenantID),
	}
	resources, err := c.listAllResources(ctx, filter)
	if err != nil {
		return nil, err
	}
	return util.GetSpecificResourceList[*osv1.OperatingSystemResource](resources)
}

func (c *InventoryClient) ListInstancesForTenant(ctx context.Context,
	tenantID string,
) ([]*computev1.InstanceResource, error) {
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Instance{}},
		Filter: fmt.Sprintf(`%s = %q`,
			computev1.InstanceResourceFieldTenantId,
			tenantID),
	}
	resources, err := c.listAllResources(ctx, filter)
	if err != nil {
		return nil, err
	}
	return util.GetSpecificResourceList[*computev1.InstanceResource](resources)
}

func (c *InventoryClient) CreateOSResource(
	ctx context.Context, tenantID string, osRes *osv1.OperatingSystemResource,
) (string, error) {
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Os{
			Os: osRes,
		},
	}

	res, err := c.Client.Create(ctx, tenantID, res)
	if err != nil {
		return "", err
	}

	return res.GetOs().GetResourceId(), err
}

func (c *InventoryClient) CreateProvider(ctx context.Context, tenantID string, providerRes *providerv1.ProviderResource) error {
	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Provider{
			Provider: providerRes,
		},
	}

	_, err := c.Client.Create(ctx, tenantID, res)
	return err
}

func (c *InventoryClient) GetProviderSingularByName(
	ctx context.Context, tenantID, providerName string,
) (*providerv1.ProviderResource, error) {
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Provider{}},
		Filter: fmt.Sprintf(`%s = %q AND %s = %q`,
			providerv1.ProviderResourceFieldTenantId, tenantID,
			providerv1.ProviderResourceFieldName, providerName),
	}
	resources, err := c.listAllResources(ctx, filter)
	if err != nil {
		return nil, err
	}

	err = util.CheckListOutputIsSingular(resources)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Obtained non-singular Provider resource: name=%s, totalElem=%v",
			providerName, len(resources))
		return nil, err
	}

	return resources[0].GetProvider(), nil
}

func (c *InventoryClient) UpdateTenantOSWatcher(
	ctx context.Context,
	tenantID string,
	resourceID string,
	ackFlag bool,
) error {
	fm := &fieldmaskpb.FieldMask{
		Paths: []string{
			tenant_v1.TenantFieldWatcherOsmanager,
		},
	}

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Tenant{
			Tenant: &tenant_v1.Tenant{
				WatcherOsmanager: ackFlag,
			},
		},
	}

	_, err := c.Client.Update(ctx, tenantID, resourceID, fm, res)
	return err
}

func (c *InventoryClient) FindAllResources(ctx context.Context,
	kinds []inv_v1.ResourceKind,
) ([]*inv_client.ResourceTenantIDCarrier, error) {
	var allResources []*inv_client.ResourceTenantIDCarrier
	for _, kind := range kinds {
		res, err := util.GetResourceFromKind(kind)
		if err != nil {
			return nil, err
		}
		filter := &inv_v1.ResourceFilter{
			Resource: res,
		}
		resources, err := c.Client.FindAll(ctx, filter)
		if err != nil {
			return nil, err
		}
		allResources = append(allResources, resources...)
	}
	return allResources, nil
}

func (c *InventoryClient) FindOSResourceID(ctx context.Context,
	tenantID, profileName, osImageVersion string,
) (string, error) {
	filter := &inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_Os{}},
		Filter: fmt.Sprintf(`%s = %q AND %s = %q AND %s = %q`,
			osv1.OperatingSystemResourceFieldTenantId, tenantID,
			osv1.OperatingSystemResourceFieldProfileName, profileName,
			osv1.OperatingSystemResourceFieldImageId, osImageVersion,
		),
	}

	resources, err := c.Client.ListAll(ctx, filter)
	if err != nil {
		return "", err
	}

	// Inventory can return multiple OS resources due to wildcard matching on strings.
	// TODO: This will be fixed in ITEP-14836.
	// The below code is temporary and does exact matching to uniquely identify OS resources.
	// The exact matching also requires ListAll that introduces performance penalty and
	// should be replaced with FindAll as part of ITEP-14836.
	osResources := make([]*inv_v1.Resource, 0)
	for _, res := range resources {
		os := res.GetOs()

		if os.GetProfileName() == profileName && os.GetImageId() == osImageVersion && os.GetTenantId() == tenantID {
			osResources = append(osResources, res)
		}
	}

	err = util.CheckListOutputIsSingular(osResources)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf(
			"Obtained non-singular OS resource ID: tenant=%s, profileName=%s, osImageVersion=%s, totalElem=%v",
			tenantID, profileName, osImageVersion, len(resources),
		)
		return "", err
	}

	return osResources[0].GetOs().GetResourceId(), nil
}

func (c *InventoryClient) UpdateOSResourceExistingCves(ctx context.Context,
	tenantID string, osRes *osv1.OperatingSystemResource,
) error {
	fm := &fieldmaskpb.FieldMask{
		Paths: []string{
			osv1.OperatingSystemResourceFieldExistingCves,
		},
	}

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Os{
			Os: osRes,
		},
	}

	_, err := c.Client.Update(ctx, tenantID, osRes.GetResourceId(), fm, res)
	return err
}

func (c *InventoryClient) UpdateOSResourceFixedCves(ctx context.Context,
	tenantID string, osRes *osv1.OperatingSystemResource,
) error {
	fm := &fieldmaskpb.FieldMask{
		Paths: []string{
			osv1.OperatingSystemResourceFieldFixedCves,
		},
	}

	res := &inv_v1.Resource{
		Resource: &inv_v1.Resource_Os{
			Os: osRes,
		},
	}

	_, err := c.Client.Update(ctx, tenantID, osRes.GetResourceId(), fm, res)
	return err
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package invclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util/collections"
)

var logC = logging.GetLogger("TelemetryCache")

const defaultBatchSize = uint32(500)

var (
	// PeriodicCacheRefresh is the interval for periodic cache refresh.
	PeriodicCacheRefresh = 5 * time.Minute
	// BatchSize is the batch size for cache operations.
	BatchSize = defaultBatchSize
)

type cacheKey struct {
	tenantID, id string
}

func (c cacheKey) String() string {
	return fmt.Sprintf("key[tenantID=%s, resourceID=%s]", c.tenantID, c.id)
}

func (c cacheKey) IsEmpty() bool {
	return c.tenantID == "" && c.id == ""
}

func emptyCacheKey() cacheKey {
	return cacheKey{"", ""}
}

func getKeyFromTP(tp *telemetryv1.TelemetryProfile) cacheKey {
	return getKey(tp.GetTenantId(), tp.GetResourceId())
}

func getKeyFromTG(tg *telemetryv1.TelemetryGroupResource) cacheKey {
	return getKey(tg.GetTenantId(), tg.GetResourceId())
}

func getKey(tenantID, id string) cacheKey {
	return cacheKey{
		tenantID: tenantID,
		id:       id,
	}
}

// TelemetryCache provides caching for telemetry profiles.
type TelemetryCache struct {
	InvClient client.TenantAwareInventoryClient

	wg      *sync.WaitGroup
	sigTerm chan bool

	ProfileIDMap map[cacheKey]*telemetryv1.TelemetryProfile // maps resource ID to Telemetry Profile

	groupsIDToProfileID    map[cacheKey][]cacheKey // Map Telemetry Groups ID to Telemetry Profile ID
	relationIDToProfilesID map[cacheKey][]cacheKey // Maps Instance, Site, or Region IDs to corresponding Telemetry Profile IDs
	lock                   sync.Mutex              // Mutex to access cache maps
}

// NewTelemetryCacheClientWithOptions creates a new telemetry cache client with options.
func NewTelemetryCacheClientWithOptions(
	ctx context.Context,
	opts ...Option,
) (*TelemetryCache, error) {
	var wg sync.WaitGroup
	eventsWatcher := make(chan *client.WatchEvents)

	var options Options
	for _, opt := range opts {
		opt(&options)
	}

	clientCfg := client.InventoryClientConfig{
		Name:    "telemetry_cache",
		Address: options.InventoryAddress,
		SecurityCfg: &client.SecurityConfig{
			// TODO: support secured connection
			Insecure: true,
		},
		Events:              eventsWatcher,
		EnableRegisterRetry: true,
		ClientKind:          inv_v1.ClientKind_CLIENT_KIND_API,
		ResourceKinds: []inv_v1.ResourceKind{
			inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE,
			inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_GROUP,
		},
		Wg:            &wg,
		EnableTracing: options.EnableTracing,
		DialOptions:   options.DialOptions,
	}

	invClient, err := client.NewTenantAwareInventoryClient(ctx, clientCfg)
	if err != nil {
		return nil, err
	}

	invHandler := NewTelemetryCacheClient(invClient)

	// blocking load the first time
	invHandler.LoadAllTelemetryProfiles()

	// Schedule periodic job to update the cache every periodicCacheRefresh
	ticker := time.NewTicker(PeriodicCacheRefresh)

	invHandler.wg.Add(1)
	go func() {
		for {
			select {
			case ev, ok := <-eventsWatcher:
				if ok {
					invHandler.manageEvent(ev.Event)
				} else {
					// If eventsWatcher channel is closed, stream ended
					ticker.Stop()
					logC.InfraSec().Fatal().Msg("gRPC stream with inventory closed")
				}
			case <-ticker.C:
				// Reconcile state of the cache
				invHandler.LoadAllTelemetryProfiles()
			case <-invHandler.sigTerm:
				// Stop the ticker and signal done
				// No other events will be processed
				ticker.Stop()
				invHandler.wg.Done()
				return
			}
		}
	}()

	return invHandler, err
}

// NewTelemetryCacheClient creates a client that wraps an existing Inventory client. Mainly for testing.
func NewTelemetryCacheClient(invClient client.TenantAwareInventoryClient) *TelemetryCache {
	return &TelemetryCache{
		InvClient: invClient,
		wg:        &sync.WaitGroup{},
		sigTerm:   make(chan bool),
	}
}

// Stop stops the telemetry cache.
func (tc *TelemetryCache) Stop() {
	close(tc.sigTerm)
	tc.wg.Wait()
	logC.Info().Msg("Schedule cache client stopped")
}

func (tc *TelemetryCache) manageEvent(event *inv_v1.SubscribeEventsResponse) {
	tenantID, resourceID, err := util.GetResourceKeyFromResource(event.Resource)
	if err != nil {
		// Error here should never happen, event.Resource should always be a tenantID and resourceID carrier
		logC.Err(err).Msgf("this should never happen")
		return
	}
	evKey := getKey(tenantID, resourceID)
	logC.Debug().Msgf("Got event: eventKind=%s, %s", event.EventKind, evKey)
	kind, err := util.GetResourceKindFromResourceID(evKey.id)
	if err != nil {
		return
	}
	if kind != inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE && kind != inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_GROUP {
		logC.InfraError("Unexpected resource kind in manageEvent: eventKind=%s", kind)
		return
	}
	tc.reconcileResource(evKey, kind, event.EventKind)
}

// reconcileResource reconciles the given resourceID, upon event.
// If DELETED event and the resource is a TelemetryProfile, we need to clean up any state in the cache, by cleaning both
// the ProfileIDMap and all the reverse maps.
// If CREATED event and the resource is a TelemetryProfile, we need to populate the cache and reverse maps.
// If UPDATED event and the resource is a TelemetryGroup, we to update the eager loaded edge of the TelemetryProfiles
// using the given TelemetryGroup.
// If UPDATED event and the resource is a TelemetryProfile, we update the status of the cache by comparing old and new state.
func (tc *TelemetryCache) reconcileResource(
	key cacheKey,
	resKind inv_v1.ResourceKind,
	evKind inv_v1.SubscribeEventsResponse_EventKind,
) {
	if evKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_DELETED {
		// Do not query inventory when deleting, resource won't be there
		// The assumption here is that telemetry profiles will be deleted before telemetry groups to respect
		// referential constraints.
		tc.lock.Lock()
		defer tc.lock.Unlock()
		// TODO: maybe better a switch by resKind?
		if tp, ok := tc.ProfileIDMap[key]; ok {
			// Here we are reconciling telemetry profile!
			delete(tc.ProfileIDMap, key)
			// Remove TP in the reverse map from TG to TP ID if any
			updateResourceIDMap(tc.groupsIDToProfileID, key, tp.GetGroup(), nil)
			// Delete also the reverse map instance, region, site to TP
			updateRelationIDMap(tc.relationIDToProfilesID, key, tp, nil)
		}
		// We don't care about Telemetry Groups
		return
	}

	// Not a DELETE
	res, err := tc.getResource(key)
	if err != nil {
		logC.InfraErr(err).Msgf("Failed to reconcile resource: %s", key)
		return
	}
	tc.lock.Lock()
	defer tc.lock.Unlock()
	switch resKind {
	case inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_GROUP:
		if evKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED {
			// Update Groups in telemetry profiles retrieving them via the reverseMap
			tg := res.GetTelemetryGroup()
			tgKey := getKeyFromTG(tg)
			// Reset eager loaded resources
			tg.Profiles = nil
			tpIDs := tc.groupsIDToProfileID[tgKey]
			collections.ForEach[cacheKey](tpIDs, func(tpID cacheKey) {
				if tp, ok := tc.ProfileIDMap[tpID]; ok {
					tp.Group = tg
				}
			})
		}
	case inv_v1.ResourceKind_RESOURCE_KIND_TELEMETRY_PROFILE:
		tp := res.GetTelemetryProfile()
		tpKey := getKeyFromTP(tp)
		var oldTp *telemetryv1.TelemetryProfile
		if evKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED {
			oldTp = tc.ProfileIDMap[tpKey]
		}
		tc.ProfileIDMap[tpKey] = tp
		updateResourceIDMap(tc.groupsIDToProfileID, key, oldTp.GetGroup(), tp.GetGroup())
		updateRelationIDMap(tc.relationIDToProfilesID, key, oldTp, tp)
	default:
		logC.InfraError("Unexpected resource kind while reconciling resource: %s, evKind=%s", key, evKind)
	}
}

func (tc *TelemetryCache) getResource(key cacheKey) (*inv_v1.Resource, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := tc.InvClient.Get(ctx, key.tenantID, key.id)
	if err != nil {
		return nil, err
	}
	return res.GetResource(), nil
}

// ListTelemetryProfileByRelation lists telemetry profiles by relation.
func (tc *TelemetryCache) ListTelemetryProfileByRelation(tenantID, relationResourceID string) []*telemetryv1.TelemetryProfile {
	key := getKey(tenantID, relationResourceID)
	tc.lock.Lock()
	defer tc.lock.Unlock()
	var res []*telemetryv1.TelemetryProfile
	for _, tpID := range tc.relationIDToProfilesID[key] {
		if tp, ok := tc.ProfileIDMap[tpID]; ok {
			res = append(res, tp)
		} else {
			logC.Error().Msgf(
				"Missing TP while it's there in the relation map: %s, tpID=%s",
				key, tpID,
			)
		}
	}
	return res
}

// LoadAllTelemetryProfiles loads all telemetry profiles into the cache.
func (tc *TelemetryCache) LoadAllTelemetryProfiles() {
	// TODO Current reconciliation is dumb, clean everything in the local cache and re-build it
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	telemetryProfiles := loadTelemetryProfilesFromInv(ctx, tc.InvClient)
	tc.lock.Lock()
	defer tc.lock.Unlock()
	reverseMap := make(map[cacheKey][]cacheKey)
	relationMap := make(map[cacheKey][]cacheKey)

	for resID, tp := range telemetryProfiles {
		updateResourceIDMap(reverseMap, resID, nil, tp.GetGroup())
		updateRelationIDMap(relationMap, resID, nil, tp)
	}

	tc.ProfileIDMap = telemetryProfiles
	tc.groupsIDToProfileID = reverseMap
	tc.relationIDToProfilesID = relationMap
}

// TestGetAllTelemetryProfiles gets all the schedule without tenant distinction. Used for testing purposes only.
func (tc *TelemetryCache) TestGetAllTelemetryProfiles() []*telemetryv1.TelemetryProfile {
	allTelprof := make([]*telemetryv1.TelemetryProfile, 0, len(tc.ProfileIDMap))
	for _, v := range tc.ProfileIDMap {
		allTelprof = append(allTelprof, v)
	}
	return allTelprof
}

func loadTelemetryProfilesFromInv(
	ctx context.Context,
	invClient client.TenantAwareInventoryClient,
) map[cacheKey]*telemetryv1.TelemetryProfile {
	filterRequest := inv_v1.ResourceFilter{
		Resource: &inv_v1.Resource{Resource: &inv_v1.Resource_TelemetryProfile{}},
		Filter:   "", // Empty filter, get all resources for the given type
		Limit:    BatchSize,
		Offset:   0,
	}
	resources := make(map[cacheKey]*telemetryv1.TelemetryProfile)
	hasNext := true
	for hasNext {
		listResponse, err := invClient.List(ctx, &filterRequest)
		if errors.IsNotFound(err) {
			logC.Debug().Msg("No more TP resources in inventory")
			break
		}
		if err != nil {
			logC.InfraErr(err).Msgf("Failed to %v resources from inventory.", resources)
			break
		}
		collections.ForEach[*inv_v1.GetResourceResponse](listResponse.GetResources(), func(response *inv_v1.GetResourceResponse) {
			if response.GetResource().GetTelemetryProfile().GetResourceId() != "" {
				tp := response.GetResource().GetTelemetryProfile()
				resources[getKeyFromTP(tp)] = tp
			} else {
				// We should never reach this point
				logC.InfraError("Got wrong resource type, expected Telemetry Profile")
			}
		})
		hasNext = listResponse.HasNext
		filterRequest.Offset += BatchSize
	}
	return resources
}

func removeFromSlice[T comparable](slice []T, target T) []T {
	for i, elem := range slice {
		if elem == target {
			slice[i] = slice[len(slice)-1]
			return slice[:len(slice)-1]
		}
	}
	return slice
}

type resourceKeyCarrier interface {
	GetResourceId() string
	GetTenantId() string
}

func updateRelationIDMap(relationMap map[cacheKey][]cacheKey, targetTpKey cacheKey, oldTp, newTp *telemetryv1.TelemetryProfile) {
	updateResourceIDMap(relationMap, targetTpKey, oldTp.GetInstance(), newTp.GetInstance())
	updateResourceIDMap(relationMap, targetTpKey, oldTp.GetSite(), newTp.GetSite())
	updateResourceIDMap(relationMap, targetTpKey, oldTp.GetRegion(), newTp.GetRegion())
}

func updateResourceIDMap(relationMap map[cacheKey][]cacheKey, targetTpKey cacheKey, oldRes, newRes resourceKeyCarrier) {
	oldKey := emptyCacheKey()
	newKey := emptyCacheKey()
	if oldRes != nil {
		oldKey = getKey(oldRes.GetTenantId(), oldRes.GetResourceId())
	}
	if newRes != nil {
		newKey = getKey(newRes.GetTenantId(), newRes.GetResourceId())
	}
	if oldRes != newRes {
		if !oldKey.IsEmpty() {
			relationMap[oldKey] = removeFromSlice(relationMap[oldKey], targetTpKey)
		}
		if !newKey.IsEmpty() {
			relationMap[newKey] = append(relationMap[newKey], targetTpKey)
		}
	}
}

// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package handlers implements northbound handlers for the Host Manager.
package handlers

import (
	"context"
	"sync"
	"time"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/host/pkg/alivemgr"
	"github.com/open-edge-platform/infra-managers/host/pkg/invclient"
	hrm_status "github.com/open-edge-platform/infra-managers/host/pkg/status"
)

// Filter returns true if the event must be processed.
type Filter func(event *inv_v1.SubscribeEventsResponse) bool

const (
	loggerName = "NBHandler"

	// a default interval for periodic reconciliation.
	// Periodic reconciliation guarantees events are handled even
	// if our notification won't deliver an event.
	// In HRM, reconciliation is only used to clean up stale hosts from the heartbeat map,
	// so we can use quite-long interval (24h).
	defaultTickerPeriod = 24 * time.Hour
)

var (
	// TickerPeriod is the period for the ticker in the northbound handler.
	zlog = logging.GetLogger(loggerName)
	// TickerPeriod is the interval between host status updates.
	TickerPeriod = defaultTickerPeriod
)

// HostManagerNBHandler handles northbound communication for host manager..
type HostManagerNBHandler struct {
	invClient inv_client.TenantAwareInventoryClient
	invEvents chan *inv_client.WatchEvents
	filters   map[inv_v1.ResourceKind]Filter
	wg        *sync.WaitGroup
	sigTerm   chan bool
}

// NewNBHandler creates a new northbound handler for host management.
func NewNBHandler(
	invClient inv_client.TenantAwareInventoryClient,
	invEvents chan *inv_client.WatchEvents,
) (*HostManagerNBHandler, error) {
	filters := make(map[inv_v1.ResourceKind]Filter)
	// Adding Host-related filter on events
	filters[inv_v1.ResourceKind_RESOURCE_KIND_HOST] = filterHostEvents
	filters[inv_v1.ResourceKind_RESOURCE_KIND_INSTANCE] = filterInstanceEvents

	return &HostManagerNBHandler{
		invClient: invClient,
		invEvents: invEvents,
		filters:   filters,
		wg:        &sync.WaitGroup{},
		sigTerm:   make(chan bool),
	}, nil
}

// Start begins the periodic host status update loop..
func (nbh *HostManagerNBHandler) Start() error {
	zlog.Info().Msgf("HRM northbound handler started")
	if err := nbh.initializeAliveMgrWithHosts(); err != nil {
		return err
	}
	// This is needed for the internal coordination
	// See Stop() method for further details
	nbh.wg.Add(1)
	// Full reconciliation is triggered periodically and concur with the events
	// Note, reconcilers will guarantee the in-order processing (by hashing).
	go nbh.controlLoop(time.NewTicker(TickerPeriod))

	return nil
}

func (nbh *HostManagerNBHandler) controlLoop(ticker *time.Ticker) {
	for {
		select {
		case ev, ok := <-nbh.invEvents:
			if !ok {
				// Event channel is closed, stream ended. Bye!
				ticker.Stop()
				// Note this will cover the sigterm scenario as well
				zlog.InfraSec().Fatal().Msg("gRPC stream with inventory closed")
			}
			// Either an error or unexpected event.
			if !nbh.filterEvent(ev.Event) {
				continue
			}
			nbh.reconcileResource(ev.Event.Resource)
		case <-ticker.C:
			// Full periodic reconcile action
			if err := nbh.reconcileAll(); err != nil {
				zlog.InfraSec().InfraErr(err).Msg("Full reconciliation failed")
			}
		case <-nbh.sigTerm:
			// Stop the ticker and signal done
			// No other events will be processed
			ticker.Stop()
			nbh.wg.Done()
			return
		}
	}
}

// Stop stops the northbound handler and cleans up resources.
func (nbh *HostManagerNBHandler) Stop() {
	close(nbh.sigTerm)
	nbh.wg.Wait()
	zlog.Info().Msg("HRM northbound handler stopped")
}

func (nbh *HostManagerNBHandler) initializeAliveMgrWithHosts() error {
	ctx, cancel := context.WithTimeout(context.Background(), *invclient.ListAllInventoryTimeout)
	defer cancel()

	hosts, err := invclient.GetHostResources(ctx, nbh.invClient)
	if err != nil {
		return err
	}

	for _, host := range hosts {
		if host.GetHostStatus() == hrm_status.HostStatusRunning.Status ||
			(host.Instance != nil && host.Instance.GetCurrentState() == computev1.InstanceState_INSTANCE_STATE_RUNNING) {
			err = alivemgr.UpdateHostHeartBeat(host)
			if err != nil {
				continue
			}
		}
	}

	return nil
}

func (nbh *HostManagerNBHandler) filterEvent(event *inv_v1.SubscribeEventsResponse) bool {
	zlog.Debug().Msgf("New inventory event received. ResourceID=%v, Kind=%s", event.ResourceId, event.EventKind)
	if err := validator.ValidateMessage(event); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Invalid event received: %s", event.ResourceId)
		return false
	}

	expectedKind, err := util.GetResourceKindFromResourceID(event.ResourceId)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unknown resource kind for ID %s.", event.ResourceId)
		return false
	}
	filterFunc, ok := nbh.filters[expectedKind]
	if !ok {
		// no filter, we accept all events
		zlog.Debug().Msgf("No filter found for resource kind %s, accepting all events", expectedKind)
		return true
	}
	return filterFunc(event)
}

// reconcileAll reconciles the state of availability manager.
// Note that the current implementation is adjusted to availability manager
// and should be refactored if more generic reconciliation is needed in the future.
func (nbh *HostManagerNBHandler) reconcileAll() error {
	zlog.Info().Msg("Reconciling all hosts")
	ctx, cancel := context.WithTimeout(context.Background(), *invclient.ListAllInventoryTimeout)
	defer cancel()

	// hostIDs will contain all hosts that exist in the system, but are not deauthorized.
	hostIDs, err := invclient.FindAllTrustedHosts(ctx, nbh.invClient)
	// Note NOT_FOUND is ok, in case of different errors we stop
	if err != nil && !inv_errors.IsNotFound(err) {
		zlog.InfraSec().InfraError("Unable to reconcile all hosts").Msg("reconcileAll")
		return err
	}

	// We pass hostIDs as desired host list to the alive manager's heartbeat map.
	// Since hostIDs contains only authorized hosts, all currently deauthorized hosts
	// (or already deleted and therefore not included in hostIDs) will be removed from
	// the heartbeat map.
	alivemgr.SyncHosts(hostIDs)

	return nil
}

// Helper function to reconcile the resources.
func (nbh *HostManagerNBHandler) reconcileResource(resource *inv_v1.Resource) {
	expectedKind := util.GetResourceKindFromResource(resource)

	switch expectedKind {
	case inv_v1.ResourceKind_RESOURCE_KIND_HOST:
		reconcileHost(resource)
	case inv_v1.ResourceKind_RESOURCE_KIND_INSTANCE:
		monitorInstanceRunning(resource)
	default:
		zlog.Debug().Msgf("Unsupported resource kind %s, ignoring", expectedKind)
	}
}

func monitorInstanceRunning(resource *inv_v1.Resource) {
	instance := resource.GetInstance()
	host := instance.Host

	// check if host has been provisioned, if so, start checking heartbeat
	if !alivemgr.IsHostTracked(host) && instance.GetCurrentState() == computev1.InstanceState_INSTANCE_STATE_RUNNING {
		zlog.Debug().Msgf("Host %s has been provisioned, start monitoring heartbeat", host.GetResourceId())
		err := alivemgr.UpdateHostHeartBeat(host)
		if err != nil {
			zlog.Error().Msgf("Host %s could not be added to the heartbeat list", host.GetResourceId())
		}
	}
}

func reconcileHost(resource *inv_v1.Resource) {
	host := resource.GetHost()

	zlog.Debug().Msgf("Reconciling host (tID=%s, resID=%s)", host.GetTenantId(), host.GetResourceId())
	// current state should be enough but in case of any potential issues/races
	// it's more robust to use both desired and current states.
	if host.GetDesiredState() == computev1.HostState_HOST_STATE_UNTRUSTED ||
		host.GetDesiredState() == computev1.HostState_HOST_STATE_DELETED ||
		host.GetCurrentState() == computev1.HostState_HOST_STATE_UNTRUSTED ||
		host.GetCurrentState() == computev1.HostState_HOST_STATE_DELETED {
		zlog.Debug().Msgf("Host %s has been deleted or invalidated, removing from the heartbeat list",
			host.GetResourceId())
		alivemgr.ForgetHost(host)
	}
}

func filterHostEvents(event *inv_v1.SubscribeEventsResponse) bool {
	return event.EventKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED
}

func filterInstanceEvents(event *inv_v1.SubscribeEventsResponse) bool {
	return event.EventKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED
}

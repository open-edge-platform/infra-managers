// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

// Package handlers implements the gRPC service handlers for the networking manager.
package handlers

import (
	"context"
	"sync"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/networking/internal/clients"
	"github.com/open-edge-platform/infra-managers/networking/internal/reconcilers"
	"github.com/open-edge-platform/infra-managers/networking/internal/utils"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

// Misc variables.
var (
	loggerName  = "NBHandler"
	zlog        = logging.GetLogger(loggerName)
	parallelism = 1

	// a default interval for periodic reconciliation.
	// Periodic reconciliation guarantees events are handled even
	// if our notification won't deliver an event.
	// In NRM, we do a reconciliation check every 24 hours,
	// Since it's only validating the IP duplicates, it's not a time-sensitive operation.
	TickerPeriod = 24 * time.Hour
)

// NBHandler supervises a list of controllers.
type NBHandler struct {
	Controllers map[inv_v1.ResourceKind]*rec_v2.Controller[reconcilers.ReconcilerID]
	Filters     map[inv_v1.ResourceKind]Filter
	netClient   *clients.NetInventoryClient
	wg          *sync.WaitGroup
	sigTerm     chan bool
}

// NewNBHandler creates a new NBHandler that supervises the reconciliation
// of the Northbound (Inventory) resources.
func NewNBHandler(netCl *clients.NetInventoryClient, tracingEnabled bool) (*NBHandler, error) {
	// Initialize all the reconcilers with their controllers
	controllers := make(map[inv_v1.ResourceKind]*rec_v2.Controller[reconcilers.ReconcilerID], 1)
	filters := make(map[inv_v1.ResourceKind]Filter, 1)
	ipReconciler, err := reconcilers.NewIPReconciler(netCl, tracingEnabled)
	if err != nil {
		return nil, err
	}
	ipController := rec_v2.NewController[reconcilers.ReconcilerID](ipReconciler.Reconcile, rec_v2.WithParallelism(parallelism))
	controllers[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = ipController
	// Initialize the associated filters
	filters[inv_v1.ResourceKind_RESOURCE_KIND_IPADDRESS] = filterEvents
	// Note that the resourceID from the events will provide the mapping
	nbHandler := &NBHandler{
		Controllers: controllers,
		Filters:     filters,
		netClient:   netCl,
		wg:          &sync.WaitGroup{},
		sigTerm:     make(chan bool),
	}
	return nbHandler, nil
}

// Start starts the NB handler and its control loop.
func (nbh *NBHandler) Start() error {
	// 1. Reconcile all
	// 2. Start controlLoop
	// 3. Stop by watching events
	if err := nbh.reconcileAll(); err != nil {
		return err
	}
	// This is needed for the internal coordination
	// See Stop() method for further details
	nbh.wg.Add(1)
	// Full reconciliation is triggered periodically and concur with the events
	// Note, reconcilers will guarantee the in-order processing (by hashing).
	go nbh.controlLoop(time.NewTicker(TickerPeriod))
	zlog.InfraSec().Info().Msgf("NB handler started")
	return nil
}

// Stop stops the NB handler and its control loop.
func (nbh *NBHandler) Stop() {
	// 1. Take down the controlLoop
	// 2. Wait the end of the control loop
	// 3. Graceful shutdown of the controllers
	close(nbh.sigTerm)
	nbh.wg.Wait()
	for _, controller := range nbh.Controllers {
		controller.Stop()
	}
	zlog.InfraSec().Info().Msgf("NB handler stopped")
}

// NB control loop implementation.
func (nbh *NBHandler) controlLoop(ticker *time.Ticker) {
	for {
		select {
		case ev, ok := <-nbh.netClient.Watcher:
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
			tID, resID, err := util.GetResourceKeyFromResource(ev.Event.GetResource())
			if err != nil {
				zlog.InfraSec().Err(err).Msgf("Failed to get resource key from event: event=%v", ev.Event)
				continue
			}
			nbh.reconcileResource(tID, resID)
		case <-ticker.C:
			// Full periodic reconcile action
			_ = nbh.reconcileAll() //nolint:errcheck // Errors logged within reconcileAll
		case <-nbh.sigTerm:
			// Stop the ticker and signal done
			// No other events will be processed
			ticker.Stop()
			nbh.wg.Done()
			return
		}
	}
}

// Reconcile all resources of interest for this RM.
func (nbh *NBHandler) reconcileAll() error {
	zlog.Debug().Msgf("Reconciling all IPAddresses")

	ctx, cancel := context.WithTimeout(context.Background(), *clients.ListAllInventoryTimeout)
	defer cancel()
	// Go backward in the hierarchy.
	ipIDs, err := nbh.netClient.FindIPAddresses(ctx)
	if err != nil {
		return err
	}
	for _, ipID := range ipIDs {
		nbh.reconcileResource(ipID.GetTenantId(), ipID.GetResourceId())
	}
	// IPSubnet, NetworkSegment, ...
	return nil
}

// Helper function to reconcile the resources.
func (nbh *NBHandler) reconcileResource(tenantID, resourceID string) {
	expectedKind, err := util.GetResourceKindFromResourceID(resourceID)
	// Unexpected events or error scenarios
	if err != nil {
		return
	}
	zlog.Debug().Msgf("Reconciling resource %s of kind=%s", utils.FormatTenantResourceID(tenantID, resourceID), expectedKind)
	controller, ok := nbh.Controllers[expectedKind]
	if !ok {
		zlog.InfraSec().InfraError("Unhandled resource %s", utils.FormatTenantResourceID(tenantID, resourceID)).Msgf("")
		return
	}
	err = controller.Reconcile(reconcilers.NewReconcilerID(tenantID, resourceID))
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Unable to reconcile resource %s", resourceID)
	}
}

// Helper function to filter events.
func (nbh *NBHandler) filterEvent(event *inv_v1.SubscribeEventsResponse) bool {
	zlog.Debug().Msgf("New inventory event received. %s, Kind=%s", event, event.GetResourceId())
	if err := validator.ValidateMessage(event); err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Invalid event received: %s", event.GetResourceId())
		return false
	}
	expectedKind, err := util.GetResourceKindFromResourceID(event.GetResourceId())
	// Unexpected events or error scenarios
	if err != nil {
		return true
	}
	filterFunc, ok := nbh.Filters[expectedKind]
	if !ok {
		// Means everything is accepted
		return false
	}
	return filterFunc(event)
}

// Filter returns true if the event must be processed.
type Filter func(event *inv_v1.SubscribeEventsResponse) bool

func filterEvents(event *inv_v1.SubscribeEventsResponse) bool {
	// For now we care only when the events are UPDATED or CREATED
	return event.GetEventKind() != inv_v1.SubscribeEventsResponse_EVENT_KIND_DELETED
}

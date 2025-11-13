// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"sync"
	"time"

	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/common"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/controller/reconcilers"
	"github.com/open-edge-platform/infra-managers/os-resource/internal/invclient"
	rec_v2 "github.com/open-edge-platform/orch-library/go/pkg/controller/v2"
)

const (
	parallelism = 1
	// Reconciliation timeout must be longer than CVE download operations.
	// which can take 1+ minutes for large CVE datasets.
	reconciliationTimeout = 90 * time.Second
)

var (
	loggerName = "OSResourceController"
	zlog       = logging.GetLogger(loggerName)

	defaultInventoryTickerPeriod = 60 * time.Minute
)

type OSResourceController struct {
	invClient *invclient.InventoryClient

	tenantReconciler *rec_v2.Controller[reconcilers.ReconcilerID]

	wg   *sync.WaitGroup
	stop chan bool
}

func New(
	invClient *invclient.InventoryClient,
	osConfig common.OsConfig,
) (*OSResourceController, error) {
	tenantRcnl := reconcilers.NewTenantReconciler(invClient, osConfig)
	tenantCtrl := rec_v2.NewController[reconcilers.ReconcilerID](
		tenantRcnl.Reconcile,
		rec_v2.WithParallelism(parallelism),
		rec_v2.WithTimeout(reconciliationTimeout))

	return &OSResourceController{
		invClient:        invClient,
		tenantReconciler: tenantCtrl,
		wg:               &sync.WaitGroup{},
		stop:             make(chan bool),
	}, nil
}

func (c *OSResourceController) Start() error {
	if err := c.reconcileAll(); err != nil {
		return err
	}

	c.wg.Add(1)
	go c.controlLoop()

	zlog.InfraSec().Info().Msgf("Inventory controller started")
	return nil
}

func (c *OSResourceController) Stop() {
	close(c.stop)
	c.wg.Wait()
	zlog.InfraSec().Info().Msgf("Inventory controller stopped")
}

func (c *OSResourceController) controlLoop() {
	// TODO: to be decided if we need separate tickers (separate sync loops) for RS and Inv
	ticker := time.NewTicker(defaultInventoryTickerPeriod)
	defer ticker.Stop()

	for {
		select {
		case ev, ok := <-c.invClient.Watcher:
			// we assume that watcher listens for tenant events only
			if !ok {
				zlog.InfraSec().Fatal().Msg("gRPC stream with inventory closed")
				return
			}
			if !tenantEventFilter(ev.Event) {
				zlog.Debug().Msgf("Event %v is not allowed by filter", ev.Event)
				continue
			}

			tenantID, resID, err := util.GetResourceKeyFromResource(ev.Event.GetResource())
			if err != nil {
				zlog.InfraSec().Err(err).Msgf("Failed to get resource key from event: event=%v", ev.Event)
				continue
			}

			if err := c.tenantReconciler.Reconcile(reconcilers.WrapReconcilerID(tenantID, resID)); err != nil {
				zlog.InfraSec().InfraErr(err).Msgf("reconciliation resource failed")
			}
		case <-ticker.C:
			if err := c.reconcileAll(); err != nil {
				zlog.InfraSec().InfraErr(err).Msgf("full reconciliation failed")
			}
		case <-c.stop:
			c.wg.Done()
			return
		}
	}
}

func (c *OSResourceController) reconcileAll() error {
	zlog.Debug().Msgf("Reconciling all resources")

	// Use context.WithTimeout to set a timeout for the operation
	ctx, cancel := context.WithTimeout(context.Background(), *invclient.InventoryTimeout)
	defer cancel()

	resourceKinds := []inv_v1.ResourceKind{
		inv_v1.ResourceKind_RESOURCE_KIND_TENANT,
	}
	resourceTenantIDs, err := c.invClient.FindAllResources(ctx, resourceKinds)
	if err != nil && !inv_errors.IsNotFound(err) {
		return err
	}

	for _, resourceTenantID := range resourceTenantIDs {
		err = c.tenantReconciler.Reconcile(reconcilers.WrapReconcilerID(
			resourceTenantID.GetTenantId(), resourceTenantID.GetResourceId(),
		))
		if err != nil {
			return err
		}
	}

	return nil
}

func tenantEventFilter(event *inv_v1.SubscribeEventsResponse) bool {
	return event.EventKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_CREATED ||
		event.EventKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_UPDATED ||
		event.EventKind == inv_v1.SubscribeEventsResponse_EVENT_KIND_DELETED
}

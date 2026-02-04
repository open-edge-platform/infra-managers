// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

// Package invclient provides an interface to the Inventory service for the Attestation Status Manager.
package invclient

import (
	"context"
	"flag"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	computev1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/compute/v1"
	inv_v1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/inventory/v1"
	inv_client "github.com/open-edge-platform/infra-core/inventory/v2/pkg/client"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	inv_util "github.com/open-edge-platform/infra-core/inventory/v2/pkg/util"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/validator"
	"github.com/open-edge-platform/infra-managers/attestationstatus/pkg/config"
)

const (
	// DefaultInventoryTimeout is the default timeout for Inventory operations.
	DefaultInventoryTimeout = 5 * time.Second
	// ListAllDefaultTimeout The current estimation is very conservative considering 10k resources, batch size 100,
	//  and 600ms per request on average.
	// TODO: fine tune this longer timeout based on target scale and inventory client batch size.
	ListAllDefaultTimeout = time.Minute // Longer timeout for reconciling all resources
	// eventsWatcherBufSize is the buffer size for the events channel.
	eventsWatcherBufSize = 10
)

var (
	zlog = logging.GetLogger("InvClient")

	inventoryClient inv_client.TenantAwareInventoryClient

	inventoryTimeout = flag.Duration("invTimeout", DefaultInventoryTimeout, "Inventory API calls timeout")
)

// StartInventoryClient initializes and starts the Inventory gRPC client.
func StartInventoryClient(wg *sync.WaitGroup, conf config.AttestationStatusMgrConfig) error {
	ctx := context.Background()

	zlog.InfraSec().Info().Msg("Starting Inventory gRPC Client")

	events := make(chan *inv_client.WatchEvents, eventsWatcherBufSize)
	cfg := inv_client.InventoryClientConfig{
		Name:                      "attestmgr",
		Address:                   conf.InventoryAddr,
		Events:                    events,
		EnableRegisterRetry:       false,
		AbortOnUnknownClientError: true,
		ClientKind:                inv_v1.ClientKind_CLIENT_KIND_RESOURCE_MANAGER,
		EnableTracing:             conf.EnableTracing,
		EnableMetrics:             conf.EnableMetrics,
		Wg:                        wg,
		SecurityCfg: &inv_client.SecurityConfig{
			CaPath:   conf.CACertPath,
			KeyPath:  conf.TLSKeyPath,
			CertPath: conf.TLSCertPath,
			Insecure: conf.InsecureGRPC,
		},
	}

	gcli, err := inv_client.NewTenantAwareInventoryClient(ctx, cfg)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Cannot create new Inventory gRPC client")
		return err
	}

	SetInventoryClient(gcli)
	zlog.InfraSec().Info().Msg("Inventory gRPC Client started")

	return nil
}

// SetInventoryClient sets the global inventory client.
func SetInventoryClient(gcli inv_client.TenantAwareInventoryClient) {
	inventoryClient = gcli
}

// CloseInventoryClient closes the global inventory client connection.
func CloseInventoryClient() {
	if err := inventoryClient.Close(); err != nil {
		zlog.Warn().Err(err).Msg("Failed to close inventory client")
	}
}

// GetInstanceIDByHostGUID returns an Instance ResourceID for the given Host GUID if it exists.
func GetInstanceIDByHostGUID(
	ctx context.Context,
	tenantID string,
	inHostGUID string,
) (string, error) {
	zlog.Debug().Msgf("GetInstanceIDByHostGUID tenantID=%s, HostGUID=%s", tenantID, inHostGUID)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	// get Host by GUID, or fail
	hostRes, err := inventoryClient.GetHostByUUID(childCtx, tenantID, inHostGUID)
	if err != nil {
		return "", err
	}

	if err = validator.ValidateMessage(hostRes); err != nil {
		zlog.InfraSec().InfraErr(err).Msg("")
		return "", inv_errors.Wrap(err)
	}

	instanceRes := hostRes.GetInstance()

	// if host has no instance
	if instanceRes == nil {
		return "", inv_errors.Errorfc(
			codes.NotFound, "Instance not found for tenantID=%s, hostID=%s", tenantID, hostRes.GetResourceId())
	}

	return instanceRes.GetResourceId(), nil
}

// UpdateInstanceAttestationStatus updates the attestation status for an instance.
func UpdateInstanceAttestationStatus(
	ctx context.Context,
	tenantID string,
	instResID string,
	instRes *computev1.InstanceResource,
) error {
	zlog.Debug().Msgf("Updating Instance (%s) for tenantID=%s state, status and status detail", instResID, tenantID)

	childCtx, cancel := context.WithTimeout(ctx, *inventoryTimeout)
	defer cancel()

	// Set Timestamp - Unix timestamps are always positive, so conversion from int64 to uint64 is safe
	now := time.Now().Unix()
	instRes.TrustedAttestationStatusTimestamp = uint64(now)

	// only PATCH these fields per Fieldmaks
	fieldMask := &fieldmaskpb.FieldMask{
		Paths: []string{
			computev1.InstanceResourceFieldTrustedAttestationStatus,
			computev1.InstanceResourceFieldTrustedAttestationStatusIndicator,
			computev1.InstanceResourceFieldTrustedAttestationStatusTimestamp,
		},
	}

	// validate
	err := inv_util.ValidateMaskAndFilterMessage(instRes, fieldMask, true)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to validate mask and filter message accordingly for Instance resource")
		return err
	}
	instRes.ResourceId = instResID

	// update via gRPC
	_, err = inventoryClient.Update(childCtx, tenantID, instResID, fieldMask, &inv_v1.Resource{
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

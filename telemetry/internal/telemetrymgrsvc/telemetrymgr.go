/*
* SPDX-FileCopyrightText: (C) 2025 Intel Corporation
* SPDX-License-Identifier: Apache-2.0
 */

// Package telemetrymgr provides telemetry manager service implementation.
package telemetrymgr

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"

	telemetryv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/telemetry/v1"
	inv_errors "github.com/open-edge-platform/infra-core/inventory/v2/pkg/errors"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/logging"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/policy/rbac"
	"github.com/open-edge-platform/infra-core/inventory/v2/pkg/tenant"
	"github.com/open-edge-platform/infra-managers/telemetry/internal/invclient"
	pb "github.com/open-edge-platform/infra-managers/telemetry/pkg/api/telemetrymgr/v1"
)

var (
	zlog               = logging.GetLogger("TelemetryManager")
	expectedConfigSize = 100
)

// RPCServer implements the TelemetryManager gRPC handlers.
type RPCServer struct {
	pb.UnimplementedTelemetryMgrServer
	telemetryClient *invclient.TelemetryInventoryClient
	rbac            *rbac.Policy
	enableAuth      bool
}

// NewTelemetrymgrServer creates a new telemetry manager server.
func NewTelemetrymgrServer(
	telCli *invclient.TelemetryInventoryClient,
	enableAuth bool,
	opaServer *rbac.Policy,
	enableVal bool,
) *RPCServer {
	iserv := RPCServer{
		telemetryClient: telCli,
		rbac:            opaServer,
		enableAuth:      enableAuth,
	}

	// validate compatibility
	if enableVal {
		iserv.validateData()
	}

	return &iserv
}

func (s *RPCServer) validateData() {
	// get all profiles
	profileList, err := s.telemetryClient.ListAllTelemetryProfile()
	if err != nil {
		if inv_errors.IsNotFound(err) {
			zlog.InfraSec().InfraErr(err).Msgf("No profiles available to validate.")
		} else {
			zlog.InfraSec().Fatal().Err(err).Msgf("Error getting all profile.")
		}
		return
	}
	// convert all profile to config
	telemetryResponse := &pb.GetTelemetryConfigResponse{
		HostGuid:  "619e8a82-6102-43ea-bac3-7bb126248307",
		Timestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
		Cfg:       s.convertProfileToConfig(profileList),
	}
	// validate config
	if err = telemetryResponse.ValidateAll(); err != nil {
		zlog.InfraSec().Fatal().Err(err).Msgf("Fail to validate config.")
		return
	}
	zlog.Info().Msg("All profile validation success.")
}

func (s *RPCServer) listProfilesByGUID(ctx context.Context, tenantID, uuid string) ([]*telemetryv1.TelemetryProfile, error) {
	hostID, instID, err := s.telemetryClient.GetHostAndInstanceIDResourceByHostUUID(ctx, tenantID, uuid)
	if err != nil {
		zlog.InfraSec().InfraErr(err).Msgf("Failed to get instance for GUID %s", uuid)
		return nil, err
	}
	return s.telemetryClient.ListTelemetryProfilesByHostAndInstanceID(ctx, tenantID, hostID, instID)
}

// GetTelemetryConfigByGUID fetch telemetry config for a host based on GUID, including the whole inherited profiles from
// parent Site and Regions.
func (s *RPCServer) GetTelemetryConfigByGUID(ctx context.Context,
	guid *pb.GetTelemetryConfigByGuidRequest,
) (*pb.GetTelemetryConfigResponse, error) {
	zlog.InfraSec().Debug().Str("guid", guid.Guid).Msg("GetTelemetryConfigByGuid with")
	if s.enableAuth {
		if !s.rbac.IsRequestAuthorized(ctx, rbac.GetKey) {
			err := inv_errors.Errorfc(codes.PermissionDenied, "Request is blocked by RBAC")
			zlog.InfraSec().InfraErr(err).Msgf("Request GetTelemetryConfigByGUID is not authenticated")
			return nil, err
		}
	}

	tenantID, present := tenant.GetTenantIDFromContext(ctx)
	if !present {
		// This should never happen! Interceptor should either fail or set it!
		err := inv_errors.Errorfc(codes.Unauthenticated, "Tenant ID is not present in context")
		zlog.InfraSec().InfraErr(err).Msg("Request GetTelemetryConfigByGUID is not authenticated")
		return nil, err
	}

	// Get all profiles
	profileList, err := s.listProfilesByGUID(ctx, tenantID, guid.Guid)
	if err != nil {
		return nil, err
	}

	// final data
	telemetryResponse := &pb.GetTelemetryConfigResponse{
		HostGuid:  guid.Guid,
		Timestamp: time.Now().Format("2006-01-02T15:04:05-07:00"),
		Cfg:       s.convertProfileToConfig(profileList),
	}

	return telemetryResponse, nil
}

func (s *RPCServer) convertProfileToConfig(
	profileList []*telemetryv1.TelemetryProfile,
) []*pb.GetTelemetryConfigResponse_TelemetryCfg {
	// var to store all telemetry resource
	//nolint:mnd // Estimated capacity: each profile typically has ~4 metric groups
	cfgLists := make([]TelemetryResources, 0, len(profileList)*4)

	// convert telemetry profile into telemetry resources
	for _, profileRow := range profileList {
		// get group from profile
		metricGroup := profileRow.GetGroup()
		// get list of metric based on group
		for _, metricName := range metricGroup.GetGroups() {
			metricResource := TelemetryResources{
				MetricGroup:    metricName,
				MetricInterval: profileRow.GetMetricsInterval(),
				MetricKind:     metricGroup.GetCollectorKind().String(),
				MetricType:     profileRow.GetKind().String(),
				LogSeverity:    uint32(profileRow.GetLogLevel().Number()),
			}
			cfgLists = append(cfgLists, metricResource)
		}
	}

	// sort telemetry resource
	cfgListSorted := Deduplicate(cfgLists)

	// convert to telemetrycfg
	// Pre-allocate telemetryCfgArray with the expected size
	telemetryCfgArray := make([]*pb.GetTelemetryConfigResponse_TelemetryCfg, 0, expectedConfigSize)

	for _, cfgRow := range cfgListSorted {
		cfg := &pb.GetTelemetryConfigResponse_TelemetryCfg{
			Input:    cfgRow.MetricGroup,
			Type:     AssignTelemetryResourceKind(telemetryv1.TelemetryResourceKind_value[cfgRow.MetricType]),
			Kind:     AssignTelemetryCollectorKind(telemetryv1.CollectorKind_value[cfgRow.MetricKind]),
			Interval: int64(cfgRow.MetricInterval),
			Level:    AssignTelemetryResourceSeverity(int32(cfgRow.LogSeverity)),
		}

		telemetryCfgArray = append(telemetryCfgArray, cfg)
	}

	return telemetryCfgArray
}

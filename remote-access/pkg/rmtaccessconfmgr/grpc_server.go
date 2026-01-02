// SPDX-FileCopyrightText: (C) 2025 Intel Corporation
//
// SPDX-License-Identifier: Apache-2.0

package rmtaccessconfmgr

import (
	"context"
	"time"

	remoteaccessv1 "github.com/open-edge-platform/infra-core/inventory/v2/pkg/api/remoteaccess/v1"
	pb "github.com/open-edge-platform/infra-managers/remote-access/pkg/api/remaccessmgr/v1"
	inv_client "github.com/open-edge-platform/infra-managers/remote-access/pkg/clients"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	inv *inv_client.RmtAccessInventoryClient
	pb.UnimplementedRemaccessmgrServiceServer
}

func NewServer(inv *inv_client.RmtAccessInventoryClient) *Server {
	return &Server{inv: inv}
}

// GetRemoteAccessConfigByGuid: polling endpoint for agent.
// Inventory is source of truth.
func (s *Server) GetRemoteAccessConfigByGuid(
	ctx context.Context,
	req *pb.GetRemoteAccessConfigByGuidRequest,
) (*pb.GetResourceAccessConfigResponse, error) {

	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if err := req.ValidateAll(); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	tenantID := req.GetTenantID()
	uuid := req.GetUuid()

	// Inventory is source of truth.
	ra, err := s.inv.GetRemoteAccessConf(ctx, tenantID, uuid)

	// IMPORTANT: for polling endpoint, "not found" => status=NONE (not gRPC error)
	if isNotFoundErr(ra, err) { // <- implement based on your inventory error types
		return &pb.GetResourceAccessConfigResponse{
			ObservedAt: timestamppb.Now(),
			Status:     pb.ConfigStatus_CONFIG_STATUS_NONE,
			Error:      nil,
		}, nil
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "inventory get remote access config: %v", err)
	}

	now := time.Now().UTC()

	cfgStatus, spec, cfgErr := mapInventoryToAgentResponse(ra, now)

	resp := &pb.GetResourceAccessConfigResponse{
		Seq:        ra.GetConfigurationStatusTimestamp(), // best-effort "version"; can be 0 if unused
		ObservedAt: timestamppb.Now(),
		Status:     cfgStatus,
		Spec:       spec,
		Error:      cfgErr,
	}
	return resp, nil
}

func mapInventoryToAgentResponse(
	ra *remoteaccessv1.RemoteAccessConfiguration,
	now time.Time,
) (pb.ConfigStatus, *pb.AgentRemoteAccessSpec, *pb.ConfigError) {

	// If already marked ERROR by provider/manager
	if ra.GetCurrentState() == remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR {
		return pb.ConfigStatus_CONFIG_STATUS_ERROR, nil, &pb.ConfigError{Code: "current_state=ERROR"}
	}

	// Desired DISABLED/DELETED => DISABLED (agent should stop / no-op)
	switch ra.GetDesiredState() {
	case remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_DELETED,
		remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_DISABLED:
		return pb.ConfigStatus_CONFIG_STATUS_DISABLED, nil, nil
	case remoteaccessv1.RemoteAccessState_REMOTE_ACCESS_STATE_ERROR:
		return pb.ConfigStatus_CONFIG_STATUS_ERROR, nil, &pb.ConfigError{Code: "desired_state=ERROR"}
	}

	// Validate readiness (pending vs invalid)
	ready, code := evaluateReadiness(ra, now)
	switch ready {
	case readinessInvalid:
		return pb.ConfigStatus_CONFIG_STATUS_ERROR, nil, &pb.ConfigError{Code: code}
	case readinessPending:
		return pb.ConfigStatus_CONFIG_STATUS_PENDING, nil, nil
	case readinessReady:
		// fallthrough
	}

	spec := &pb.AgentRemoteAccessSpec{
		RemoteAccessProxyEndpoint: ra.GetProxyHost(), // should be agent-reachable RAP endpoint (ws/wss)
		SessionToken:              ra.GetSessionToken(),
		ReverseBindPort:           ra.GetLocalPort(),  // RAP reverse bind port
		TargetHost:                ra.GetTargetHost(), // usually 127.0.0.1
		TargetPort:                ra.GetTargetPort(), // usually 22
		SshUser:                   ra.GetUser(),
		ExpirationTimestamp:       ra.GetExpirationTimestamp(),
		Uuid:                      ra.GetResourceId(), // ra id
	}

	return pb.ConfigStatus_CONFIG_STATUS_ACTIVE, spec, nil
}

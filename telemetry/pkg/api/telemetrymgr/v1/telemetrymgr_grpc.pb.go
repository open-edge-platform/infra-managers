// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: telemetrymgr/v1/telemetrymgr.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// TelemetryMgrClient is the client API for TelemetryMgr service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TelemetryMgrClient interface {
	GetTelemetryConfigByGUID(ctx context.Context, in *GetTelemetryConfigByGuidRequest, opts ...grpc.CallOption) (*GetTelemetryConfigResponse, error)
}

type telemetryMgrClient struct {
	cc grpc.ClientConnInterface
}

func NewTelemetryMgrClient(cc grpc.ClientConnInterface) TelemetryMgrClient {
	return &telemetryMgrClient{cc}
}

func (c *telemetryMgrClient) GetTelemetryConfigByGUID(ctx context.Context, in *GetTelemetryConfigByGuidRequest, opts ...grpc.CallOption) (*GetTelemetryConfigResponse, error) {
	out := new(GetTelemetryConfigResponse)
	err := c.cc.Invoke(ctx, "/telemetrymgr.v1.TelemetryMgr/GetTelemetryConfigByGUID", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TelemetryMgrServer is the server API for TelemetryMgr service.
// All implementations should embed UnimplementedTelemetryMgrServer
// for forward compatibility
type TelemetryMgrServer interface {
	GetTelemetryConfigByGUID(context.Context, *GetTelemetryConfigByGuidRequest) (*GetTelemetryConfigResponse, error)
}

// UnimplementedTelemetryMgrServer should be embedded to have forward compatible implementations.
type UnimplementedTelemetryMgrServer struct {
}

func (UnimplementedTelemetryMgrServer) GetTelemetryConfigByGUID(context.Context, *GetTelemetryConfigByGuidRequest) (*GetTelemetryConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTelemetryConfigByGUID not implemented")
}

// UnsafeTelemetryMgrServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TelemetryMgrServer will
// result in compilation errors.
type UnsafeTelemetryMgrServer interface {
	mustEmbedUnimplementedTelemetryMgrServer()
}

func RegisterTelemetryMgrServer(s grpc.ServiceRegistrar, srv TelemetryMgrServer) {
	s.RegisterService(&TelemetryMgr_ServiceDesc, srv)
}

func _TelemetryMgr_GetTelemetryConfigByGUID_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetTelemetryConfigByGuidRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TelemetryMgrServer).GetTelemetryConfigByGUID(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/telemetrymgr.v1.TelemetryMgr/GetTelemetryConfigByGUID",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TelemetryMgrServer).GetTelemetryConfigByGUID(ctx, req.(*GetTelemetryConfigByGuidRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TelemetryMgr_ServiceDesc is the grpc.ServiceDesc for TelemetryMgr service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TelemetryMgr_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "telemetrymgr.v1.TelemetryMgr",
	HandlerType: (*TelemetryMgrServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetTelemetryConfigByGUID",
			Handler:    _TelemetryMgr_GetTelemetryConfigByGUID_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "telemetrymgr/v1/telemetrymgr.proto",
}

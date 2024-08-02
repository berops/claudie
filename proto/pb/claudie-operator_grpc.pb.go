// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v5.27.1
// source: claudie-operator.proto

package pb

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

// OperatorServiceClient is the client API for OperatorService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OperatorServiceClient interface {
	// SendAutoscalerEvent sends events to claudie-operator when a scale up/down occurs
	SendAutoscalerEvent(ctx context.Context, in *SendAutoscalerEventRequest, opts ...grpc.CallOption) (*SendAutoscalerEventResponse, error)
}

type operatorServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewOperatorServiceClient(cc grpc.ClientConnInterface) OperatorServiceClient {
	return &operatorServiceClient{cc}
}

func (c *operatorServiceClient) SendAutoscalerEvent(ctx context.Context, in *SendAutoscalerEventRequest, opts ...grpc.CallOption) (*SendAutoscalerEventResponse, error) {
	out := new(SendAutoscalerEventResponse)
	err := c.cc.Invoke(ctx, "/claudie.OperatorService/SendAutoscalerEvent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OperatorServiceServer is the server API for OperatorService service.
// All implementations must embed UnimplementedOperatorServiceServer
// for forward compatibility
type OperatorServiceServer interface {
	// SendAutoscalerEvent sends events to claudie-operator when a scale up/down occurs
	SendAutoscalerEvent(context.Context, *SendAutoscalerEventRequest) (*SendAutoscalerEventResponse, error)
	mustEmbedUnimplementedOperatorServiceServer()
}

// UnimplementedOperatorServiceServer must be embedded to have forward compatible implementations.
type UnimplementedOperatorServiceServer struct {
}

func (UnimplementedOperatorServiceServer) SendAutoscalerEvent(context.Context, *SendAutoscalerEventRequest) (*SendAutoscalerEventResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendAutoscalerEvent not implemented")
}
func (UnimplementedOperatorServiceServer) mustEmbedUnimplementedOperatorServiceServer() {}

// UnsafeOperatorServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OperatorServiceServer will
// result in compilation errors.
type UnsafeOperatorServiceServer interface {
	mustEmbedUnimplementedOperatorServiceServer()
}

func RegisterOperatorServiceServer(s grpc.ServiceRegistrar, srv OperatorServiceServer) {
	s.RegisterService(&OperatorService_ServiceDesc, srv)
}

func _OperatorService_SendAutoscalerEvent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SendAutoscalerEventRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OperatorServiceServer).SendAutoscalerEvent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/claudie.OperatorService/SendAutoscalerEvent",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OperatorServiceServer).SendAutoscalerEvent(ctx, req.(*SendAutoscalerEventRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// OperatorService_ServiceDesc is the grpc.ServiceDesc for OperatorService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OperatorService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "claudie.OperatorService",
	HandlerType: (*OperatorServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SendAutoscalerEvent",
			Handler:    _OperatorService_SendAutoscalerEvent_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "claudie-operator.proto",
}

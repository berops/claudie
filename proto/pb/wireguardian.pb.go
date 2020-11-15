// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.13.0
// source: wireguardian.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Status struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Success bool `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
}

func (x *Status) Reset() {
	*x = Status{}
	if protoimpl.UnsafeEnabled {
		mi := &file_wireguardian_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Status) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Status) ProtoMessage() {}

func (x *Status) ProtoReflect() protoreflect.Message {
	mi := &file_wireguardian_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Status.ProtoReflect.Descriptor instead.
func (*Status) Descriptor() ([]byte, []int) {
	return file_wireguardian_proto_rawDescGZIP(), []int{0}
}

func (x *Status) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

var File_wireguardian_proto protoreflect.FileDescriptor

var file_wireguardian_proto_rawDesc = []byte{
	0x0a, 0x12, 0x77, 0x69, 0x72, 0x65, 0x67, 0x75, 0x61, 0x72, 0x64, 0x69, 0x61, 0x6e, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x1a, 0x0e,
	0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x22,
	0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x75, 0x63, 0x63,
	0x65, 0x73, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x32, 0x42, 0x0a, 0x0f, 0x42, 0x75, 0x69, 0x6c, 0x64, 0x56, 0x50, 0x4e, 0x53, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x2f, 0x0a, 0x08, 0x42, 0x75, 0x69, 0x6c, 0x64, 0x56, 0x50,
	0x4e, 0x12, 0x11, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x50, 0x72, 0x6f,
	0x6a, 0x65, 0x63, 0x74, 0x1a, 0x10, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e,
	0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x42, 0x0a, 0x5a, 0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_wireguardian_proto_rawDescOnce sync.Once
	file_wireguardian_proto_rawDescData = file_wireguardian_proto_rawDesc
)

func file_wireguardian_proto_rawDescGZIP() []byte {
	file_wireguardian_proto_rawDescOnce.Do(func() {
		file_wireguardian_proto_rawDescData = protoimpl.X.CompressGZIP(file_wireguardian_proto_rawDescData)
	})
	return file_wireguardian_proto_rawDescData
}

var file_wireguardian_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_wireguardian_proto_goTypes = []interface{}{
	(*Status)(nil),  // 0: platform.Status
	(*Project)(nil), // 1: platform.Project
}
var file_wireguardian_proto_depIdxs = []int32{
	1, // 0: platform.BuildVPNService.BuildVPN:input_type -> platform.Project
	0, // 1: platform.BuildVPNService.BuildVPN:output_type -> platform.Status
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_wireguardian_proto_init() }
func file_wireguardian_proto_init() {
	if File_wireguardian_proto != nil {
		return
	}
	file_platform_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_wireguardian_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Status); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_wireguardian_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_wireguardian_proto_goTypes,
		DependencyIndexes: file_wireguardian_proto_depIdxs,
		MessageInfos:      file_wireguardian_proto_msgTypes,
	}.Build()
	File_wireguardian_proto = out.File
	file_wireguardian_proto_rawDesc = nil
	file_wireguardian_proto_goTypes = nil
	file_wireguardian_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// BuildVPNServiceClient is the client API for BuildVPNService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type BuildVPNServiceClient interface {
	BuildVPN(ctx context.Context, in *Project, opts ...grpc.CallOption) (*Status, error)
}

type buildVPNServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBuildVPNServiceClient(cc grpc.ClientConnInterface) BuildVPNServiceClient {
	return &buildVPNServiceClient{cc}
}

func (c *buildVPNServiceClient) BuildVPN(ctx context.Context, in *Project, opts ...grpc.CallOption) (*Status, error) {
	out := new(Status)
	err := c.cc.Invoke(ctx, "/platform.BuildVPNService/BuildVPN", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BuildVPNServiceServer is the server API for BuildVPNService service.
type BuildVPNServiceServer interface {
	BuildVPN(context.Context, *Project) (*Status, error)
}

// UnimplementedBuildVPNServiceServer can be embedded to have forward compatible implementations.
type UnimplementedBuildVPNServiceServer struct {
}

func (*UnimplementedBuildVPNServiceServer) BuildVPN(context.Context, *Project) (*Status, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BuildVPN not implemented")
}

func RegisterBuildVPNServiceServer(s *grpc.Server, srv BuildVPNServiceServer) {
	s.RegisterService(&_BuildVPNService_serviceDesc, srv)
}

func _BuildVPNService_BuildVPN_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Project)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BuildVPNServiceServer).BuildVPN(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/platform.BuildVPNService/BuildVPN",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BuildVPNServiceServer).BuildVPN(ctx, req.(*Project))
	}
	return interceptor(ctx, in, info, handler)
}

var _BuildVPNService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "platform.BuildVPNService",
	HandlerType: (*BuildVPNServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "BuildVPN",
			Handler:    _BuildVPNService_BuildVPN_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "wireguardian.proto",
}

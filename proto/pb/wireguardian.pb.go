// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        v3.12.4
// source: proto/wireguardian.proto

package pb

import (
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

type RunAnsibleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DesiredState *Project `protobuf:"bytes,1,opt,name=desiredState,proto3" json:"desiredState,omitempty"`
	CurrentState *Project `protobuf:"bytes,2,opt,name=currentState,proto3" json:"currentState,omitempty"`
}

func (x *RunAnsibleRequest) Reset() {
	*x = RunAnsibleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_wireguardian_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunAnsibleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunAnsibleRequest) ProtoMessage() {}

func (x *RunAnsibleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_wireguardian_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunAnsibleRequest.ProtoReflect.Descriptor instead.
func (*RunAnsibleRequest) Descriptor() ([]byte, []int) {
	return file_proto_wireguardian_proto_rawDescGZIP(), []int{0}
}

func (x *RunAnsibleRequest) GetDesiredState() *Project {
	if x != nil {
		return x.DesiredState
	}
	return nil
}

func (x *RunAnsibleRequest) GetCurrentState() *Project {
	if x != nil {
		return x.CurrentState
	}
	return nil
}

type RunAnsibleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DesiredState *Project `protobuf:"bytes,1,opt,name=desiredState,proto3" json:"desiredState,omitempty"`
	ErrorMessage string   `protobuf:"bytes,2,opt,name=errorMessage,proto3" json:"errorMessage,omitempty"`
}

func (x *RunAnsibleResponse) Reset() {
	*x = RunAnsibleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_wireguardian_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RunAnsibleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RunAnsibleResponse) ProtoMessage() {}

func (x *RunAnsibleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_wireguardian_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RunAnsibleResponse.ProtoReflect.Descriptor instead.
func (*RunAnsibleResponse) Descriptor() ([]byte, []int) {
	return file_proto_wireguardian_proto_rawDescGZIP(), []int{1}
}

func (x *RunAnsibleResponse) GetDesiredState() *Project {
	if x != nil {
		return x.DesiredState
	}
	return nil
}

func (x *RunAnsibleResponse) GetErrorMessage() string {
	if x != nil {
		return x.ErrorMessage
	}
	return ""
}

var File_proto_wireguardian_proto protoreflect.FileDescriptor

var file_proto_wireguardian_proto_rawDesc = []byte{
	0x0a, 0x18, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x77, 0x69, 0x72, 0x65, 0x67, 0x75, 0x61, 0x72,
	0x64, 0x69, 0x61, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x08, 0x70, 0x6c, 0x61, 0x74,
	0x66, 0x6f, 0x72, 0x6d, 0x1a, 0x12, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x6f, 0x6e, 0x66,
	0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x81, 0x01, 0x0a, 0x11, 0x52, 0x75, 0x6e,
	0x41, 0x6e, 0x73, 0x69, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x35,
	0x0a, 0x0c, 0x64, 0x65, 0x73, 0x69, 0x72, 0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e,
	0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x0c, 0x64, 0x65, 0x73, 0x69, 0x72, 0x65, 0x64,
	0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x35, 0x0a, 0x0c, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74,
	0x53, 0x74, 0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70, 0x6c,
	0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x0c,
	0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x53, 0x74, 0x61, 0x74, 0x65, 0x22, 0x6f, 0x0a, 0x12,
	0x52, 0x75, 0x6e, 0x41, 0x6e, 0x73, 0x69, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x12, 0x35, 0x0a, 0x0c, 0x64, 0x65, 0x73, 0x69, 0x72, 0x65, 0x64, 0x53, 0x74, 0x61,
	0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x11, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66,
	0x6f, 0x72, 0x6d, 0x2e, 0x50, 0x72, 0x6f, 0x6a, 0x65, 0x63, 0x74, 0x52, 0x0c, 0x64, 0x65, 0x73,
	0x69, 0x72, 0x65, 0x64, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x22, 0x0a, 0x0c, 0x65, 0x72, 0x72,
	0x6f, 0x72, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0c, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x32, 0x5e, 0x0a,
	0x13, 0x57, 0x69, 0x72, 0x65, 0x67, 0x75, 0x61, 0x72, 0x64, 0x69, 0x61, 0x6e, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x12, 0x47, 0x0a, 0x0a, 0x52, 0x75, 0x6e, 0x41, 0x6e, 0x73, 0x69, 0x62,
	0x6c, 0x65, 0x12, 0x1b, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x52, 0x75,
	0x6e, 0x41, 0x6e, 0x73, 0x69, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x1c, 0x2e, 0x70, 0x6c, 0x61, 0x74, 0x66, 0x6f, 0x72, 0x6d, 0x2e, 0x52, 0x75, 0x6e, 0x41, 0x6e,
	0x73, 0x69, 0x62, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x0a, 0x5a,
	0x08, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_proto_wireguardian_proto_rawDescOnce sync.Once
	file_proto_wireguardian_proto_rawDescData = file_proto_wireguardian_proto_rawDesc
)

func file_proto_wireguardian_proto_rawDescGZIP() []byte {
	file_proto_wireguardian_proto_rawDescOnce.Do(func() {
		file_proto_wireguardian_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_wireguardian_proto_rawDescData)
	})
	return file_proto_wireguardian_proto_rawDescData
}

var file_proto_wireguardian_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_wireguardian_proto_goTypes = []interface{}{
	(*RunAnsibleRequest)(nil),  // 0: platform.RunAnsibleRequest
	(*RunAnsibleResponse)(nil), // 1: platform.RunAnsibleResponse
	(*Project)(nil),            // 2: platform.Project
}
var file_proto_wireguardian_proto_depIdxs = []int32{
	2, // 0: platform.RunAnsibleRequest.desiredState:type_name -> platform.Project
	2, // 1: platform.RunAnsibleRequest.currentState:type_name -> platform.Project
	2, // 2: platform.RunAnsibleResponse.desiredState:type_name -> platform.Project
	0, // 3: platform.WireguardianService.RunAnsible:input_type -> platform.RunAnsibleRequest
	1, // 4: platform.WireguardianService.RunAnsible:output_type -> platform.RunAnsibleResponse
	4, // [4:5] is the sub-list for method output_type
	3, // [3:4] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_proto_wireguardian_proto_init() }
func file_proto_wireguardian_proto_init() {
	if File_proto_wireguardian_proto != nil {
		return
	}
	file_proto_config_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_proto_wireguardian_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunAnsibleRequest); i {
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
		file_proto_wireguardian_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RunAnsibleResponse); i {
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
			RawDescriptor: file_proto_wireguardian_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_wireguardian_proto_goTypes,
		DependencyIndexes: file_proto_wireguardian_proto_depIdxs,
		MessageInfos:      file_proto_wireguardian_proto_msgTypes,
	}.Build()
	File_proto_wireguardian_proto = out.File
	file_proto_wireguardian_proto_rawDesc = nil
	file_proto_wireguardian_proto_goTypes = nil
	file_proto_wireguardian_proto_depIdxs = nil
}

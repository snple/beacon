// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.12.4
// source: cores/device_service.proto

package cores

import (
	pb "github.com/snple/kokomi/pb"
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

type ListDeviceRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page *pb.Page `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Tags string   `protobuf:"bytes,2,opt,name=tags,proto3" json:"tags,omitempty"`
	Type string   `protobuf:"bytes,3,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *ListDeviceRequest) Reset() {
	*x = ListDeviceRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_device_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListDeviceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListDeviceRequest) ProtoMessage() {}

func (x *ListDeviceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_device_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListDeviceRequest.ProtoReflect.Descriptor instead.
func (*ListDeviceRequest) Descriptor() ([]byte, []int) {
	return file_cores_device_service_proto_rawDescGZIP(), []int{0}
}

func (x *ListDeviceRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListDeviceRequest) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *ListDeviceRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type ListDeviceResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page   *pb.Page     `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Count  uint32       `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Device []*pb.Device `protobuf:"bytes,3,rep,name=device,proto3" json:"device,omitempty"`
}

func (x *ListDeviceResponse) Reset() {
	*x = ListDeviceResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_device_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListDeviceResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListDeviceResponse) ProtoMessage() {}

func (x *ListDeviceResponse) ProtoReflect() protoreflect.Message {
	mi := &file_cores_device_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListDeviceResponse.ProtoReflect.Descriptor instead.
func (*ListDeviceResponse) Descriptor() ([]byte, []int) {
	return file_cores_device_service_proto_rawDescGZIP(), []int{1}
}

func (x *ListDeviceResponse) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListDeviceResponse) GetCount() uint32 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ListDeviceResponse) GetDevice() []*pb.Device {
	if x != nil {
		return x.Device
	}
	return nil
}

type LinkDeviceRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id     string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Status int32  `protobuf:"zigzag32,2,opt,name=status,proto3" json:"status,omitempty"`
}

func (x *LinkDeviceRequest) Reset() {
	*x = LinkDeviceRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_device_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LinkDeviceRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LinkDeviceRequest) ProtoMessage() {}

func (x *LinkDeviceRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_device_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LinkDeviceRequest.ProtoReflect.Descriptor instead.
func (*LinkDeviceRequest) Descriptor() ([]byte, []int) {
	return file_cores_device_service_proto_rawDescGZIP(), []int{2}
}

func (x *LinkDeviceRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *LinkDeviceRequest) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

var File_cores_device_service_proto protoreflect.FileDescriptor

var file_cores_device_service_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2f, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x63, 0x6f,
	0x72, 0x65, 0x73, 0x1a, 0x14, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72,
	0x69, 0x63, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x59, 0x0a, 0x11, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70,
	0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x6c, 0x0a, 0x12, 0x4c,
	0x69, 0x73, 0x74, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12,
	0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x22, 0x0a, 0x06, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x18,
	0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63,
	0x65, 0x52, 0x06, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x22, 0x3b, 0x0a, 0x11, 0x4c, 0x69, 0x6e,
	0x6b, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e,
	0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x11, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x32, 0xea, 0x02, 0x0a, 0x0d, 0x44, 0x65, 0x76, 0x69, 0x63,
	0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x22, 0x0a, 0x06, 0x43, 0x72, 0x65, 0x61,
	0x74, 0x65, 0x12, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x1a, 0x0a,
	0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x22, 0x00, 0x12, 0x22, 0x0a, 0x06,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69,
	0x63, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x22, 0x00,
	0x12, 0x1c, 0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64,
	0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x22, 0x00, 0x12, 0x24,
	0x0a, 0x0a, 0x56, 0x69, 0x65, 0x77, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x08, 0x2e, 0x70,
	0x62, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x44, 0x65, 0x76, 0x69,
	0x63, 0x65, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x06,
	0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f,
	0x6f, 0x6c, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x18, 0x2e, 0x63,
	0x6f, 0x72, 0x65, 0x73, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x4c,
	0x69, 0x73, 0x74, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x2e, 0x0a, 0x04, 0x4c, 0x69, 0x6e, 0x6b, 0x12, 0x18, 0x2e, 0x63, 0x6f,
	0x72, 0x65, 0x73, 0x2e, 0x4c, 0x69, 0x6e, 0x6b, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f,
	0x6c, 0x22, 0x00, 0x12, 0x1f, 0x0a, 0x07, 0x44, 0x65, 0x73, 0x74, 0x6f, 0x72, 0x79, 0x12, 0x06,
	0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f,
	0x6f, 0x6c, 0x22, 0x00, 0x12, 0x1d, 0x0a, 0x05, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x12, 0x06, 0x2e,
	0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f,
	0x6c, 0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x6b, 0x6f, 0x6b, 0x6f, 0x6d, 0x69, 0x2f, 0x70,
	0x62, 0x2f, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x3b, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_cores_device_service_proto_rawDescOnce sync.Once
	file_cores_device_service_proto_rawDescData = file_cores_device_service_proto_rawDesc
)

func file_cores_device_service_proto_rawDescGZIP() []byte {
	file_cores_device_service_proto_rawDescOnce.Do(func() {
		file_cores_device_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_cores_device_service_proto_rawDescData)
	})
	return file_cores_device_service_proto_rawDescData
}

var file_cores_device_service_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_cores_device_service_proto_goTypes = []interface{}{
	(*ListDeviceRequest)(nil),  // 0: cores.ListDeviceRequest
	(*ListDeviceResponse)(nil), // 1: cores.ListDeviceResponse
	(*LinkDeviceRequest)(nil),  // 2: cores.LinkDeviceRequest
	(*pb.Page)(nil),            // 3: pb.Page
	(*pb.Device)(nil),          // 4: pb.Device
	(*pb.Id)(nil),              // 5: pb.Id
	(*pb.Name)(nil),            // 6: pb.Name
	(*pb.MyBool)(nil),          // 7: pb.MyBool
}
var file_cores_device_service_proto_depIdxs = []int32{
	3,  // 0: cores.ListDeviceRequest.page:type_name -> pb.Page
	3,  // 1: cores.ListDeviceResponse.page:type_name -> pb.Page
	4,  // 2: cores.ListDeviceResponse.device:type_name -> pb.Device
	4,  // 3: cores.DeviceService.Create:input_type -> pb.Device
	4,  // 4: cores.DeviceService.Update:input_type -> pb.Device
	5,  // 5: cores.DeviceService.View:input_type -> pb.Id
	6,  // 6: cores.DeviceService.ViewByName:input_type -> pb.Name
	5,  // 7: cores.DeviceService.Delete:input_type -> pb.Id
	0,  // 8: cores.DeviceService.List:input_type -> cores.ListDeviceRequest
	2,  // 9: cores.DeviceService.Link:input_type -> cores.LinkDeviceRequest
	5,  // 10: cores.DeviceService.Destory:input_type -> pb.Id
	5,  // 11: cores.DeviceService.Clone:input_type -> pb.Id
	4,  // 12: cores.DeviceService.Create:output_type -> pb.Device
	4,  // 13: cores.DeviceService.Update:output_type -> pb.Device
	4,  // 14: cores.DeviceService.View:output_type -> pb.Device
	4,  // 15: cores.DeviceService.ViewByName:output_type -> pb.Device
	7,  // 16: cores.DeviceService.Delete:output_type -> pb.MyBool
	1,  // 17: cores.DeviceService.List:output_type -> cores.ListDeviceResponse
	7,  // 18: cores.DeviceService.Link:output_type -> pb.MyBool
	7,  // 19: cores.DeviceService.Destory:output_type -> pb.MyBool
	7,  // 20: cores.DeviceService.Clone:output_type -> pb.MyBool
	12, // [12:21] is the sub-list for method output_type
	3,  // [3:12] is the sub-list for method input_type
	3,  // [3:3] is the sub-list for extension type_name
	3,  // [3:3] is the sub-list for extension extendee
	0,  // [0:3] is the sub-list for field type_name
}

func init() { file_cores_device_service_proto_init() }
func file_cores_device_service_proto_init() {
	if File_cores_device_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_cores_device_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListDeviceRequest); i {
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
		file_cores_device_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListDeviceResponse); i {
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
		file_cores_device_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LinkDeviceRequest); i {
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
			RawDescriptor: file_cores_device_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_cores_device_service_proto_goTypes,
		DependencyIndexes: file_cores_device_service_proto_depIdxs,
		MessageInfos:      file_cores_device_service_proto_msgTypes,
	}.Build()
	File_cores_device_service_proto = out.File
	file_cores_device_service_proto_rawDesc = nil
	file_cores_device_service_proto_goTypes = nil
	file_cores_device_service_proto_depIdxs = nil
}

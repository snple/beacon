// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v3.12.4
// source: device_message.proto

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

type Device struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Desc          string                 `protobuf:"bytes,3,opt,name=desc,proto3" json:"desc,omitempty"`
	Tags          string                 `protobuf:"bytes,4,opt,name=tags,proto3" json:"tags,omitempty"`
	Secret        string                 `protobuf:"bytes,5,opt,name=secret,proto3" json:"secret,omitempty"`
	Config        string                 `protobuf:"bytes,6,opt,name=config,proto3" json:"config,omitempty"`
	Link          int32                  `protobuf:"zigzag32,7,opt,name=link,proto3" json:"link,omitempty"`
	Status        int32                  `protobuf:"zigzag32,8,opt,name=status,proto3" json:"status,omitempty"`
	Created       int64                  `protobuf:"varint,9,opt,name=created,proto3" json:"created,omitempty"`
	Updated       int64                  `protobuf:"varint,10,opt,name=updated,proto3" json:"updated,omitempty"`
	Deleted       int64                  `protobuf:"varint,11,opt,name=deleted,proto3" json:"deleted,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Device) Reset() {
	*x = Device{}
	mi := &file_device_message_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Device) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Device) ProtoMessage() {}

func (x *Device) ProtoReflect() protoreflect.Message {
	mi := &file_device_message_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Device.ProtoReflect.Descriptor instead.
func (*Device) Descriptor() ([]byte, []int) {
	return file_device_message_proto_rawDescGZIP(), []int{0}
}

func (x *Device) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Device) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Device) GetDesc() string {
	if x != nil {
		return x.Desc
	}
	return ""
}

func (x *Device) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *Device) GetSecret() string {
	if x != nil {
		return x.Secret
	}
	return ""
}

func (x *Device) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *Device) GetLink() int32 {
	if x != nil {
		return x.Link
	}
	return 0
}

func (x *Device) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Device) GetCreated() int64 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Device) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

func (x *Device) GetDeleted() int64 {
	if x != nil {
		return x.Deleted
	}
	return 0
}

var File_device_message_proto protoreflect.FileDescriptor

var file_device_message_proto_rawDesc = []byte{
	0x0a, 0x14, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x22, 0xfe, 0x01, 0x0a, 0x06, 0x44,
	0x65, 0x76, 0x69, 0x63, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x65, 0x73,
	0x63, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x64, 0x65, 0x73, 0x63, 0x12, 0x12, 0x0a,
	0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67,
	0x73, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x69, 0x6e, 0x6b, 0x18, 0x07, 0x20, 0x01, 0x28, 0x11, 0x52,
	0x04, 0x6c, 0x69, 0x6e, 0x6b, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18,
	0x08, 0x20, 0x01, 0x28, 0x11, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x18, 0x0a,
	0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x18, 0x09, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07,
	0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x64, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x0b, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x42, 0x1f, 0x5a, 0x1d, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f,
	0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2f, 0x70, 0x62, 0x3b, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_device_message_proto_rawDescOnce sync.Once
	file_device_message_proto_rawDescData = file_device_message_proto_rawDesc
)

func file_device_message_proto_rawDescGZIP() []byte {
	file_device_message_proto_rawDescOnce.Do(func() {
		file_device_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_device_message_proto_rawDescData)
	})
	return file_device_message_proto_rawDescData
}

var file_device_message_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_device_message_proto_goTypes = []any{
	(*Device)(nil), // 0: pb.Device
}
var file_device_message_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_device_message_proto_init() }
func file_device_message_proto_init() {
	if File_device_message_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_device_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_device_message_proto_goTypes,
		DependencyIndexes: file_device_message_proto_depIdxs,
		MessageInfos:      file_device_message_proto_msgTypes,
	}.Build()
	File_device_message_proto = out.File
	file_device_message_proto_rawDesc = nil
	file_device_message_proto_goTypes = nil
	file_device_message_proto_depIdxs = nil
}

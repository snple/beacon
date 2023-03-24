// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.12.4
// source: slot_message.proto

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

type Slot struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id       string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	DeviceId string `protobuf:"bytes,2,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Name     string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Desc     string `protobuf:"bytes,4,opt,name=desc,proto3" json:"desc,omitempty"`
	Tags     string `protobuf:"bytes,5,opt,name=tags,proto3" json:"tags,omitempty"`
	Type     string `protobuf:"bytes,6,opt,name=type,proto3" json:"type,omitempty"`
	Secret   string `protobuf:"bytes,7,opt,name=secret,proto3" json:"secret,omitempty"`
	Location string `protobuf:"bytes,8,opt,name=location,proto3" json:"location,omitempty"`
	Config   string `protobuf:"bytes,9,opt,name=config,proto3" json:"config,omitempty"`
	Link     int32  `protobuf:"zigzag32,10,opt,name=link,proto3" json:"link,omitempty"`
	Status   int32  `protobuf:"zigzag32,11,opt,name=status,proto3" json:"status,omitempty"`
	Created  int64  `protobuf:"varint,12,opt,name=created,proto3" json:"created,omitempty"`
	Updated  int64  `protobuf:"varint,13,opt,name=updated,proto3" json:"updated,omitempty"`
	Deleted  int64  `protobuf:"varint,14,opt,name=deleted,proto3" json:"deleted,omitempty"`
}

func (x *Slot) Reset() {
	*x = Slot{}
	if protoimpl.UnsafeEnabled {
		mi := &file_slot_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Slot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Slot) ProtoMessage() {}

func (x *Slot) ProtoReflect() protoreflect.Message {
	mi := &file_slot_message_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Slot.ProtoReflect.Descriptor instead.
func (*Slot) Descriptor() ([]byte, []int) {
	return file_slot_message_proto_rawDescGZIP(), []int{0}
}

func (x *Slot) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Slot) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *Slot) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Slot) GetDesc() string {
	if x != nil {
		return x.Desc
	}
	return ""
}

func (x *Slot) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *Slot) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Slot) GetSecret() string {
	if x != nil {
		return x.Secret
	}
	return ""
}

func (x *Slot) GetLocation() string {
	if x != nil {
		return x.Location
	}
	return ""
}

func (x *Slot) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *Slot) GetLink() int32 {
	if x != nil {
		return x.Link
	}
	return 0
}

func (x *Slot) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Slot) GetCreated() int64 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Slot) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

func (x *Slot) GetDeleted() int64 {
	if x != nil {
		return x.Deleted
	}
	return 0
}

var File_slot_message_proto protoreflect.FileDescriptor

var file_slot_message_proto_rawDesc = []byte{
	0x0a, 0x12, 0x73, 0x6c, 0x6f, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x22, 0xc9, 0x02, 0x0a, 0x04, 0x53, 0x6c, 0x6f,
	0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x64, 0x12, 0x1b, 0x0a, 0x09, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x64, 0x65, 0x73, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x6c, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x09, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x69,
	0x6e, 0x6b, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x11, 0x52, 0x04, 0x6c, 0x69, 0x6e, 0x6b, 0x12, 0x16,
	0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x11, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x64, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64,
	0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x0d, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x65,
	0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x64, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x64, 0x42, 0x18, 0x5a, 0x16, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x6b, 0x6f, 0x6b, 0x6f, 0x6d, 0x69, 0x2f, 0x70, 0x62, 0x3b, 0x70, 0x62, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_slot_message_proto_rawDescOnce sync.Once
	file_slot_message_proto_rawDescData = file_slot_message_proto_rawDesc
)

func file_slot_message_proto_rawDescGZIP() []byte {
	file_slot_message_proto_rawDescOnce.Do(func() {
		file_slot_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_slot_message_proto_rawDescData)
	})
	return file_slot_message_proto_rawDescData
}

var file_slot_message_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_slot_message_proto_goTypes = []interface{}{
	(*Slot)(nil), // 0: pb.Slot
}
var file_slot_message_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_slot_message_proto_init() }
func file_slot_message_proto_init() {
	if File_slot_message_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_slot_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Slot); i {
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
			RawDescriptor: file_slot_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_slot_message_proto_goTypes,
		DependencyIndexes: file_slot_message_proto_depIdxs,
		MessageInfos:      file_slot_message_proto_msgTypes,
	}.Build()
	File_slot_message_proto = out.File
	file_slot_message_proto_rawDesc = nil
	file_slot_message_proto_goTypes = nil
	file_slot_message_proto_depIdxs = nil
}

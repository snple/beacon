// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v3.12.4
// source: wire_message.proto

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

type Wire struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	NodeId        string                 `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	Name          string                 `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
	Desc          string                 `protobuf:"bytes,4,opt,name=desc,proto3" json:"desc,omitempty"`
	Tags          string                 `protobuf:"bytes,5,opt,name=tags,proto3" json:"tags,omitempty"`
	Source        string                 `protobuf:"bytes,6,opt,name=source,proto3" json:"source,omitempty"`
	Params        string                 `protobuf:"bytes,7,opt,name=params,proto3" json:"params,omitempty"`
	Config        string                 `protobuf:"bytes,8,opt,name=config,proto3" json:"config,omitempty"`
	Link          int32                  `protobuf:"zigzag32,9,opt,name=link,proto3" json:"link,omitempty"`
	Status        int32                  `protobuf:"zigzag32,10,opt,name=status,proto3" json:"status,omitempty"`
	Created       int64                  `protobuf:"varint,11,opt,name=created,proto3" json:"created,omitempty"`
	Updated       int64                  `protobuf:"varint,12,opt,name=updated,proto3" json:"updated,omitempty"`
	Deleted       int64                  `protobuf:"varint,13,opt,name=deleted,proto3" json:"deleted,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Wire) Reset() {
	*x = Wire{}
	mi := &file_wire_message_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Wire) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Wire) ProtoMessage() {}

func (x *Wire) ProtoReflect() protoreflect.Message {
	mi := &file_wire_message_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Wire.ProtoReflect.Descriptor instead.
func (*Wire) Descriptor() ([]byte, []int) {
	return file_wire_message_proto_rawDescGZIP(), []int{0}
}

func (x *Wire) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Wire) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *Wire) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Wire) GetDesc() string {
	if x != nil {
		return x.Desc
	}
	return ""
}

func (x *Wire) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *Wire) GetSource() string {
	if x != nil {
		return x.Source
	}
	return ""
}

func (x *Wire) GetParams() string {
	if x != nil {
		return x.Params
	}
	return ""
}

func (x *Wire) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *Wire) GetLink() int32 {
	if x != nil {
		return x.Link
	}
	return 0
}

func (x *Wire) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Wire) GetCreated() int64 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Wire) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

func (x *Wire) GetDeleted() int64 {
	if x != nil {
		return x.Deleted
	}
	return 0
}

type Pin struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	NodeId        string                 `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	WireId        string                 `protobuf:"bytes,3,opt,name=wire_id,json=wireId,proto3" json:"wire_id,omitempty"`
	Name          string                 `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	Desc          string                 `protobuf:"bytes,5,opt,name=desc,proto3" json:"desc,omitempty"`
	Tags          string                 `protobuf:"bytes,6,opt,name=tags,proto3" json:"tags,omitempty"`
	DataType      string                 `protobuf:"bytes,7,opt,name=data_type,json=dataType,proto3" json:"data_type,omitempty"`
	Address       string                 `protobuf:"bytes,8,opt,name=address,proto3" json:"address,omitempty"`
	Value         string                 `protobuf:"bytes,9,opt,name=value,proto3" json:"value,omitempty"`
	Config        string                 `protobuf:"bytes,10,opt,name=config,proto3" json:"config,omitempty"`
	Status        int32                  `protobuf:"zigzag32,11,opt,name=status,proto3" json:"status,omitempty"`
	Access        int32                  `protobuf:"zigzag32,12,opt,name=access,proto3" json:"access,omitempty"`
	Created       int64                  `protobuf:"varint,13,opt,name=created,proto3" json:"created,omitempty"`
	Updated       int64                  `protobuf:"varint,14,opt,name=updated,proto3" json:"updated,omitempty"`
	Deleted       int64                  `protobuf:"varint,15,opt,name=deleted,proto3" json:"deleted,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Pin) Reset() {
	*x = Pin{}
	mi := &file_wire_message_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Pin) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Pin) ProtoMessage() {}

func (x *Pin) ProtoReflect() protoreflect.Message {
	mi := &file_wire_message_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Pin.ProtoReflect.Descriptor instead.
func (*Pin) Descriptor() ([]byte, []int) {
	return file_wire_message_proto_rawDescGZIP(), []int{1}
}

func (x *Pin) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Pin) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *Pin) GetWireId() string {
	if x != nil {
		return x.WireId
	}
	return ""
}

func (x *Pin) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Pin) GetDesc() string {
	if x != nil {
		return x.Desc
	}
	return ""
}

func (x *Pin) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *Pin) GetDataType() string {
	if x != nil {
		return x.DataType
	}
	return ""
}

func (x *Pin) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *Pin) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *Pin) GetConfig() string {
	if x != nil {
		return x.Config
	}
	return ""
}

func (x *Pin) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

func (x *Pin) GetAccess() int32 {
	if x != nil {
		return x.Access
	}
	return 0
}

func (x *Pin) GetCreated() int64 {
	if x != nil {
		return x.Created
	}
	return 0
}

func (x *Pin) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

func (x *Pin) GetDeleted() int64 {
	if x != nil {
		return x.Deleted
	}
	return 0
}

type PinValue struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Value         string                 `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
	Updated       int64                  `protobuf:"varint,3,opt,name=updated,proto3" json:"updated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PinValue) Reset() {
	*x = PinValue{}
	mi := &file_wire_message_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PinValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PinValue) ProtoMessage() {}

func (x *PinValue) ProtoReflect() protoreflect.Message {
	mi := &file_wire_message_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PinValue.ProtoReflect.Descriptor instead.
func (*PinValue) Descriptor() ([]byte, []int) {
	return file_wire_message_proto_rawDescGZIP(), []int{2}
}

func (x *PinValue) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *PinValue) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *PinValue) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

type PinNameValue struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Value         string                 `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	Updated       int64                  `protobuf:"varint,4,opt,name=updated,proto3" json:"updated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PinNameValue) Reset() {
	*x = PinNameValue{}
	mi := &file_wire_message_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PinNameValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PinNameValue) ProtoMessage() {}

func (x *PinNameValue) ProtoReflect() protoreflect.Message {
	mi := &file_wire_message_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PinNameValue.ProtoReflect.Descriptor instead.
func (*PinNameValue) Descriptor() ([]byte, []int) {
	return file_wire_message_proto_rawDescGZIP(), []int{3}
}

func (x *PinNameValue) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *PinNameValue) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *PinNameValue) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *PinNameValue) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

type PinValueUpdated struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	NodeId        string                 `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	WireId        string                 `protobuf:"bytes,3,opt,name=wire_id,json=wireId,proto3" json:"wire_id,omitempty"`
	Value         string                 `protobuf:"bytes,4,opt,name=value,proto3" json:"value,omitempty"`
	Updated       int64                  `protobuf:"varint,5,opt,name=updated,proto3" json:"updated,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *PinValueUpdated) Reset() {
	*x = PinValueUpdated{}
	mi := &file_wire_message_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *PinValueUpdated) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PinValueUpdated) ProtoMessage() {}

func (x *PinValueUpdated) ProtoReflect() protoreflect.Message {
	mi := &file_wire_message_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PinValueUpdated.ProtoReflect.Descriptor instead.
func (*PinValueUpdated) Descriptor() ([]byte, []int) {
	return file_wire_message_proto_rawDescGZIP(), []int{4}
}

func (x *PinValueUpdated) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *PinValueUpdated) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *PinValueUpdated) GetWireId() string {
	if x != nil {
		return x.WireId
	}
	return ""
}

func (x *PinValueUpdated) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *PinValueUpdated) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

var File_wire_message_proto protoreflect.FileDescriptor

var file_wire_message_proto_rawDesc = []byte{
	0x0a, 0x12, 0x77, 0x69, 0x72, 0x65, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x70, 0x62, 0x22, 0xad, 0x02, 0x0a, 0x04, 0x57, 0x69, 0x72,
	0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69,
	0x64, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x64, 0x65,
	0x73, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x70, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x12,
	0x0a, 0x04, 0x6c, 0x69, 0x6e, 0x6b, 0x18, 0x09, 0x20, 0x01, 0x28, 0x11, 0x52, 0x04, 0x6c, 0x69,
	0x6e, 0x6b, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x0a, 0x20, 0x01,
	0x28, 0x11, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x64, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x63, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18,
	0x0c, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x12, 0x18,
	0x0a, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x22, 0xe6, 0x02, 0x0a, 0x03, 0x50, 0x69, 0x6e,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x17, 0x0a, 0x07, 0x77, 0x69, 0x72,
	0x65, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x77, 0x69, 0x72, 0x65,
	0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x65, 0x73, 0x63, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x64, 0x65, 0x73, 0x63, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61,
	0x67, 0x73, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x1b,
	0x0a, 0x09, 0x64, 0x61, 0x74, 0x61, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x64, 0x61, 0x74, 0x61, 0x54, 0x79, 0x70, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x61,
	0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64,
	0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x09,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x63, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x0b, 0x20,
	0x01, 0x28, 0x11, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x61,
	0x63, 0x63, 0x65, 0x73, 0x73, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x11, 0x52, 0x06, 0x61, 0x63, 0x63,
	0x65, 0x73, 0x73, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x18, 0x0d,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x12, 0x18, 0x0a,
	0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x0e, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07,
	0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x64, 0x18, 0x0f, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x64, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x64, 0x22, 0x4a, 0x0a, 0x08, 0x50, 0x69, 0x6e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x14, 0x0a,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x22, 0x62, 0x0a,
	0x0c, 0x50, 0x69, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x0e, 0x0a,
	0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x64, 0x22, 0x83, 0x01, 0x0a, 0x0f, 0x50, 0x69, 0x6e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x64, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x17,
	0x0a, 0x07, 0x77, 0x69, 0x72, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x77, 0x69, 0x72, 0x65, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x18, 0x0a,
	0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07,
	0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x42, 0x1f, 0x5a, 0x1d, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x62, 0x65, 0x61, 0x63,
	0x6f, 0x6e, 0x2f, 0x70, 0x62, 0x3b, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_wire_message_proto_rawDescOnce sync.Once
	file_wire_message_proto_rawDescData = file_wire_message_proto_rawDesc
)

func file_wire_message_proto_rawDescGZIP() []byte {
	file_wire_message_proto_rawDescOnce.Do(func() {
		file_wire_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_wire_message_proto_rawDescData)
	})
	return file_wire_message_proto_rawDescData
}

var file_wire_message_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_wire_message_proto_goTypes = []any{
	(*Wire)(nil),            // 0: pb.Wire
	(*Pin)(nil),             // 1: pb.Pin
	(*PinValue)(nil),        // 2: pb.PinValue
	(*PinNameValue)(nil),    // 3: pb.PinNameValue
	(*PinValueUpdated)(nil), // 4: pb.PinValueUpdated
}
var file_wire_message_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_wire_message_proto_init() }
func file_wire_message_proto_init() {
	if File_wire_message_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_wire_message_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_wire_message_proto_goTypes,
		DependencyIndexes: file_wire_message_proto_depIdxs,
		MessageInfos:      file_wire_message_proto_msgTypes,
	}.Build()
	File_wire_message_proto = out.File
	file_wire_message_proto_rawDesc = nil
	file_wire_message_proto_goTypes = nil
	file_wire_message_proto_depIdxs = nil
}

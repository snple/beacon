// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.12.4
// source: cores/const_service.proto

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

type ListConstRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page     *pb.Page `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	DeviceId string   `protobuf:"bytes,2,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Tags     string   `protobuf:"bytes,3,opt,name=tags,proto3" json:"tags,omitempty"`
	Type     string   `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *ListConstRequest) Reset() {
	*x = ListConstRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListConstRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListConstRequest) ProtoMessage() {}

func (x *ListConstRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListConstRequest.ProtoReflect.Descriptor instead.
func (*ListConstRequest) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{0}
}

func (x *ListConstRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListConstRequest) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *ListConstRequest) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *ListConstRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type ListConstResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page  *pb.Page    `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Count uint32      `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Const []*pb.Const `protobuf:"bytes,3,rep,name=const,proto3" json:"const,omitempty"`
}

func (x *ListConstResponse) Reset() {
	*x = ListConstResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListConstResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListConstResponse) ProtoMessage() {}

func (x *ListConstResponse) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListConstResponse.ProtoReflect.Descriptor instead.
func (*ListConstResponse) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{1}
}

func (x *ListConstResponse) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListConstResponse) GetCount() uint32 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ListConstResponse) GetConst() []*pb.Const {
	if x != nil {
		return x.Const
	}
	return nil
}

type ViewConstByNameRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DeviceId string `protobuf:"bytes,1,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Name     string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *ViewConstByNameRequest) Reset() {
	*x = ViewConstByNameRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ViewConstByNameRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ViewConstByNameRequest) ProtoMessage() {}

func (x *ViewConstByNameRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ViewConstByNameRequest.ProtoReflect.Descriptor instead.
func (*ViewConstByNameRequest) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{2}
}

func (x *ViewConstByNameRequest) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *ViewConstByNameRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type CloneConstRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id       string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	DeviceId string `protobuf:"bytes,2,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
}

func (x *CloneConstRequest) Reset() {
	*x = CloneConstRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CloneConstRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CloneConstRequest) ProtoMessage() {}

func (x *CloneConstRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CloneConstRequest.ProtoReflect.Descriptor instead.
func (*CloneConstRequest) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{3}
}

func (x *CloneConstRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *CloneConstRequest) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

type GetConstValueByNameRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DeviceId string `protobuf:"bytes,1,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Name     string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *GetConstValueByNameRequest) Reset() {
	*x = GetConstValueByNameRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetConstValueByNameRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetConstValueByNameRequest) ProtoMessage() {}

func (x *GetConstValueByNameRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetConstValueByNameRequest.ProtoReflect.Descriptor instead.
func (*GetConstValueByNameRequest) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{4}
}

func (x *GetConstValueByNameRequest) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *GetConstValueByNameRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type ConstNameValue struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	DeviceId string `protobuf:"bytes,1,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Name     string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Value    string `protobuf:"bytes,3,opt,name=value,proto3" json:"value,omitempty"`
	Updated  int64  `protobuf:"varint,4,opt,name=updated,proto3" json:"updated,omitempty"`
}

func (x *ConstNameValue) Reset() {
	*x = ConstNameValue{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ConstNameValue) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstNameValue) ProtoMessage() {}

func (x *ConstNameValue) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstNameValue.ProtoReflect.Descriptor instead.
func (*ConstNameValue) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{5}
}

func (x *ConstNameValue) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *ConstNameValue) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *ConstNameValue) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

func (x *ConstNameValue) GetUpdated() int64 {
	if x != nil {
		return x.Updated
	}
	return 0
}

type PullConstRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	After    int64  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit    uint32 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	DeviceId string `protobuf:"bytes,3,opt,name=device_id,json=deviceId,proto3" json:"device_id,omitempty"`
	Type     string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *PullConstRequest) Reset() {
	*x = PullConstRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullConstRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullConstRequest) ProtoMessage() {}

func (x *PullConstRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullConstRequest.ProtoReflect.Descriptor instead.
func (*PullConstRequest) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{6}
}

func (x *PullConstRequest) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *PullConstRequest) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *PullConstRequest) GetDeviceId() string {
	if x != nil {
		return x.DeviceId
	}
	return ""
}

func (x *PullConstRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type PullConstResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	After int64       `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit uint32      `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	Const []*pb.Const `protobuf:"bytes,3,rep,name=const,proto3" json:"const,omitempty"`
}

func (x *PullConstResponse) Reset() {
	*x = PullConstResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_cores_const_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullConstResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullConstResponse) ProtoMessage() {}

func (x *PullConstResponse) ProtoReflect() protoreflect.Message {
	mi := &file_cores_const_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullConstResponse.ProtoReflect.Descriptor instead.
func (*PullConstResponse) Descriptor() ([]byte, []int) {
	return file_cores_const_service_proto_rawDescGZIP(), []int{7}
}

func (x *PullConstResponse) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *PullConstResponse) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *PullConstResponse) GetConst() []*pb.Const {
	if x != nil {
		return x.Const
	}
	return nil
}

var File_cores_const_service_proto protoreflect.FileDescriptor

var file_cores_const_service_proto_rawDesc = []byte{
	0x0a, 0x19, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x5f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x63, 0x6f, 0x72,
	0x65, 0x73, 0x1a, 0x13, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63,
	0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x75,
	0x0a, 0x10, 0x4c, 0x69, 0x73, 0x74, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65,
	0x12, 0x1b, 0x0a, 0x09, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a,
	0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67,
	0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x68, 0x0a, 0x11, 0x4c, 0x69, 0x73, 0x74, 0x43, 0x6f, 0x6e,
	0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61,
	0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61,
	0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1f,
	0x0a, 0x05, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e,
	0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x05, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x22,
	0x49, 0x0a, 0x16, 0x56, 0x69, 0x65, 0x77, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x42, 0x79, 0x4e, 0x61,
	0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x64, 0x65, 0x76,
	0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64, 0x65,
	0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x40, 0x0a, 0x11, 0x43, 0x6c,
	0x6f, 0x6e, 0x65, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x1b, 0x0a, 0x09, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x22, 0x4d, 0x0a, 0x1a,
	0x47, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x79, 0x4e,
	0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x64, 0x65,
	0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x64,
	0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x71, 0x0a, 0x0e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x1b, 0x0a,
	0x09, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14,
	0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x22, 0x6f,
	0x0a, 0x10, 0x50, 0x75, 0x6c, 0x6c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x1b,
	0x0a, 0x09, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x64, 0x65, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22,
	0x60, 0x0a, 0x11, 0x50, 0x75, 0x6c, 0x6c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69,
	0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74,
	0x12, 0x1f, 0x0a, 0x05, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x05, 0x63, 0x6f, 0x6e, 0x73,
	0x74, 0x32, 0xae, 0x06, 0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x20, 0x0a, 0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x12, 0x09, 0x2e, 0x70,
	0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e,
	0x73, 0x74, 0x22, 0x00, 0x12, 0x20, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x09,
	0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00, 0x12, 0x1b, 0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06,
	0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73,
	0x74, 0x22, 0x00, 0x12, 0x38, 0x0a, 0x0a, 0x56, 0x69, 0x65, 0x77, 0x42, 0x79, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x1d, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x56, 0x69, 0x65, 0x77, 0x43, 0x6f,
	0x6e, 0x73, 0x74, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00, 0x12, 0x27, 0x0a,
	0x0e, 0x56, 0x69, 0x65, 0x77, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x46, 0x75, 0x6c, 0x6c, 0x12,
	0x08, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65,
	0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79,
	0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x3b, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x17,
	0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x43, 0x6f, 0x6e, 0x73, 0x74,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e,
	0x4c, 0x69, 0x73, 0x74, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x2f, 0x0a, 0x05, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x12, 0x18, 0x2e, 0x63,
	0x6f, 0x72, 0x65, 0x73, 0x2e, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f,
	0x6f, 0x6c, 0x22, 0x00, 0x12, 0x24, 0x0a, 0x08, 0x47, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0e, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f,
	0x6e, 0x73, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x00, 0x12, 0x28, 0x0a, 0x08, 0x53, 0x65,
	0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x0e, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73,
	0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f,
	0x6f, 0x6c, 0x22, 0x00, 0x12, 0x31, 0x0a, 0x11, 0x53, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x55, 0x6e, 0x63, 0x68, 0x65, 0x63, 0x6b, 0x65, 0x64, 0x12, 0x0e, 0x2e, 0x70, 0x62, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d,
	0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x4c, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x21, 0x2e, 0x63, 0x6f, 0x72, 0x65,
	0x73, 0x2e, 0x47, 0x65, 0x74, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42,
	0x79, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x63,
	0x6f, 0x72, 0x65, 0x73, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x22, 0x00, 0x12, 0x35, 0x0a, 0x0e, 0x53, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x15, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e,
	0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a,
	0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x17,
	0x53, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x55, 0x6e,
	0x63, 0x68, 0x65, 0x63, 0x6b, 0x65, 0x64, 0x12, 0x15, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e,
	0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a,
	0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x26, 0x0a, 0x0f,
	0x56, 0x69, 0x65, 0x77, 0x57, 0x69, 0x74, 0x68, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x12,
	0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e,
	0x73, 0x74, 0x22, 0x00, 0x12, 0x3b, 0x0a, 0x04, 0x50, 0x75, 0x6c, 0x6c, 0x12, 0x17, 0x2e, 0x63,
	0x6f, 0x72, 0x65, 0x73, 0x2e, 0x50, 0x75, 0x6c, 0x6c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x50, 0x75,
	0x6c, 0x6c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x1f, 0x0a, 0x04, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c,
	0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x6b, 0x6f, 0x6b, 0x6f, 0x6d, 0x69, 0x2f, 0x70, 0x62,
	0x2f, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x3b, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_cores_const_service_proto_rawDescOnce sync.Once
	file_cores_const_service_proto_rawDescData = file_cores_const_service_proto_rawDesc
)

func file_cores_const_service_proto_rawDescGZIP() []byte {
	file_cores_const_service_proto_rawDescOnce.Do(func() {
		file_cores_const_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_cores_const_service_proto_rawDescData)
	})
	return file_cores_const_service_proto_rawDescData
}

var file_cores_const_service_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_cores_const_service_proto_goTypes = []interface{}{
	(*ListConstRequest)(nil),           // 0: cores.ListConstRequest
	(*ListConstResponse)(nil),          // 1: cores.ListConstResponse
	(*ViewConstByNameRequest)(nil),     // 2: cores.ViewConstByNameRequest
	(*CloneConstRequest)(nil),          // 3: cores.CloneConstRequest
	(*GetConstValueByNameRequest)(nil), // 4: cores.GetConstValueByNameRequest
	(*ConstNameValue)(nil),             // 5: cores.ConstNameValue
	(*PullConstRequest)(nil),           // 6: cores.PullConstRequest
	(*PullConstResponse)(nil),          // 7: cores.PullConstResponse
	(*pb.Page)(nil),                    // 8: pb.Page
	(*pb.Const)(nil),                   // 9: pb.Const
	(*pb.Id)(nil),                      // 10: pb.Id
	(*pb.Name)(nil),                    // 11: pb.Name
	(*pb.ConstValue)(nil),              // 12: pb.ConstValue
	(*pb.MyBool)(nil),                  // 13: pb.MyBool
}
var file_cores_const_service_proto_depIdxs = []int32{
	8,  // 0: cores.ListConstRequest.page:type_name -> pb.Page
	8,  // 1: cores.ListConstResponse.page:type_name -> pb.Page
	9,  // 2: cores.ListConstResponse.const:type_name -> pb.Const
	9,  // 3: cores.PullConstResponse.const:type_name -> pb.Const
	9,  // 4: cores.ConstService.Create:input_type -> pb.Const
	9,  // 5: cores.ConstService.Update:input_type -> pb.Const
	10, // 6: cores.ConstService.View:input_type -> pb.Id
	2,  // 7: cores.ConstService.ViewByName:input_type -> cores.ViewConstByNameRequest
	11, // 8: cores.ConstService.ViewByNameFull:input_type -> pb.Name
	10, // 9: cores.ConstService.Delete:input_type -> pb.Id
	0,  // 10: cores.ConstService.List:input_type -> cores.ListConstRequest
	3,  // 11: cores.ConstService.Clone:input_type -> cores.CloneConstRequest
	10, // 12: cores.ConstService.GetValue:input_type -> pb.Id
	12, // 13: cores.ConstService.SetValue:input_type -> pb.ConstValue
	12, // 14: cores.ConstService.SetValueUnchecked:input_type -> pb.ConstValue
	4,  // 15: cores.ConstService.GetValueByName:input_type -> cores.GetConstValueByNameRequest
	5,  // 16: cores.ConstService.SetValueByName:input_type -> cores.ConstNameValue
	5,  // 17: cores.ConstService.SetValueByNameUnchecked:input_type -> cores.ConstNameValue
	10, // 18: cores.ConstService.ViewWithDeleted:input_type -> pb.Id
	6,  // 19: cores.ConstService.Pull:input_type -> cores.PullConstRequest
	9,  // 20: cores.ConstService.Sync:input_type -> pb.Const
	9,  // 21: cores.ConstService.Create:output_type -> pb.Const
	9,  // 22: cores.ConstService.Update:output_type -> pb.Const
	9,  // 23: cores.ConstService.View:output_type -> pb.Const
	9,  // 24: cores.ConstService.ViewByName:output_type -> pb.Const
	9,  // 25: cores.ConstService.ViewByNameFull:output_type -> pb.Const
	13, // 26: cores.ConstService.Delete:output_type -> pb.MyBool
	1,  // 27: cores.ConstService.List:output_type -> cores.ListConstResponse
	13, // 28: cores.ConstService.Clone:output_type -> pb.MyBool
	12, // 29: cores.ConstService.GetValue:output_type -> pb.ConstValue
	13, // 30: cores.ConstService.SetValue:output_type -> pb.MyBool
	13, // 31: cores.ConstService.SetValueUnchecked:output_type -> pb.MyBool
	5,  // 32: cores.ConstService.GetValueByName:output_type -> cores.ConstNameValue
	13, // 33: cores.ConstService.SetValueByName:output_type -> pb.MyBool
	13, // 34: cores.ConstService.SetValueByNameUnchecked:output_type -> pb.MyBool
	9,  // 35: cores.ConstService.ViewWithDeleted:output_type -> pb.Const
	7,  // 36: cores.ConstService.Pull:output_type -> cores.PullConstResponse
	13, // 37: cores.ConstService.Sync:output_type -> pb.MyBool
	21, // [21:38] is the sub-list for method output_type
	4,  // [4:21] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_cores_const_service_proto_init() }
func file_cores_const_service_proto_init() {
	if File_cores_const_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_cores_const_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListConstRequest); i {
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
		file_cores_const_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListConstResponse); i {
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
		file_cores_const_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ViewConstByNameRequest); i {
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
		file_cores_const_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CloneConstRequest); i {
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
		file_cores_const_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetConstValueByNameRequest); i {
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
		file_cores_const_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ConstNameValue); i {
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
		file_cores_const_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullConstRequest); i {
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
		file_cores_const_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullConstResponse); i {
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
			RawDescriptor: file_cores_const_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_cores_const_service_proto_goTypes,
		DependencyIndexes: file_cores_const_service_proto_depIdxs,
		MessageInfos:      file_cores_const_service_proto_msgTypes,
	}.Build()
	File_cores_const_service_proto = out.File
	file_cores_const_service_proto_rawDesc = nil
	file_cores_const_service_proto_goTypes = nil
	file_cores_const_service_proto_depIdxs = nil
}

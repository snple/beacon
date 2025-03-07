// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v3.12.4
// source: cores/slot_service.proto

package cores

import (
	pb "github.com/snple/beacon/pb"
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

type SlotListRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Page          *pb.Page               `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	NodeId        string                 `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	Tags          string                 `protobuf:"bytes,3,opt,name=tags,proto3" json:"tags,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotListRequest) Reset() {
	*x = SlotListRequest{}
	mi := &file_cores_slot_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotListRequest) ProtoMessage() {}

func (x *SlotListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotListRequest.ProtoReflect.Descriptor instead.
func (*SlotListRequest) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{0}
}

func (x *SlotListRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *SlotListRequest) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *SlotListRequest) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

type SlotListResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Page          *pb.Page               `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Count         uint32                 `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Slot          []*pb.Slot             `protobuf:"bytes,3,rep,name=slot,proto3" json:"slot,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotListResponse) Reset() {
	*x = SlotListResponse{}
	mi := &file_cores_slot_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotListResponse) ProtoMessage() {}

func (x *SlotListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotListResponse.ProtoReflect.Descriptor instead.
func (*SlotListResponse) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{1}
}

func (x *SlotListResponse) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *SlotListResponse) GetCount() uint32 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *SlotListResponse) GetSlot() []*pb.Slot {
	if x != nil {
		return x.Slot
	}
	return nil
}

type SlotNameRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	NodeId        string                 `protobuf:"bytes,1,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	Name          string                 `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotNameRequest) Reset() {
	*x = SlotNameRequest{}
	mi := &file_cores_slot_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotNameRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotNameRequest) ProtoMessage() {}

func (x *SlotNameRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotNameRequest.ProtoReflect.Descriptor instead.
func (*SlotNameRequest) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{2}
}

func (x *SlotNameRequest) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *SlotNameRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type SlotLinkRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Status        int32                  `protobuf:"zigzag32,2,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotLinkRequest) Reset() {
	*x = SlotLinkRequest{}
	mi := &file_cores_slot_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotLinkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotLinkRequest) ProtoMessage() {}

func (x *SlotLinkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotLinkRequest.ProtoReflect.Descriptor instead.
func (*SlotLinkRequest) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{3}
}

func (x *SlotLinkRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *SlotLinkRequest) GetStatus() int32 {
	if x != nil {
		return x.Status
	}
	return 0
}

type SlotCloneRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	NodeId        string                 `protobuf:"bytes,2,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotCloneRequest) Reset() {
	*x = SlotCloneRequest{}
	mi := &file_cores_slot_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotCloneRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotCloneRequest) ProtoMessage() {}

func (x *SlotCloneRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotCloneRequest.ProtoReflect.Descriptor instead.
func (*SlotCloneRequest) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{4}
}

func (x *SlotCloneRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *SlotCloneRequest) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

type SlotPullRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	After         int64                  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit         uint32                 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	NodeId        string                 `protobuf:"bytes,3,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotPullRequest) Reset() {
	*x = SlotPullRequest{}
	mi := &file_cores_slot_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotPullRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotPullRequest) ProtoMessage() {}

func (x *SlotPullRequest) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotPullRequest.ProtoReflect.Descriptor instead.
func (*SlotPullRequest) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{5}
}

func (x *SlotPullRequest) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *SlotPullRequest) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *SlotPullRequest) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

type SlotPullResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	After         int64                  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit         uint32                 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	Slot          []*pb.Slot             `protobuf:"bytes,3,rep,name=slot,proto3" json:"slot,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotPullResponse) Reset() {
	*x = SlotPullResponse{}
	mi := &file_cores_slot_service_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotPullResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotPullResponse) ProtoMessage() {}

func (x *SlotPullResponse) ProtoReflect() protoreflect.Message {
	mi := &file_cores_slot_service_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SlotPullResponse.ProtoReflect.Descriptor instead.
func (*SlotPullResponse) Descriptor() ([]byte, []int) {
	return file_cores_slot_service_proto_rawDescGZIP(), []int{6}
}

func (x *SlotPullResponse) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *SlotPullResponse) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *SlotPullResponse) GetSlot() []*pb.Slot {
	if x != nil {
		return x.Slot
	}
	return nil
}

var File_cores_slot_service_proto protoreflect.FileDescriptor

var file_cores_slot_service_proto_rawDesc = []byte{
	0x0a, 0x18, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2f, 0x73, 0x6c, 0x6f, 0x74, 0x5f, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x63, 0x6f, 0x72, 0x65,
	0x73, 0x1a, 0x12, 0x73, 0x6c, 0x6f, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63, 0x5f, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x5c, 0x0a, 0x0f,
	0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e,
	0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x17, 0x0a,
	0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06,
	0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x22, 0x64, 0x0a, 0x10, 0x53, 0x6c,
	0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c,
	0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70,
	0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x63, 0x6f, 0x75,
	0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x52, 0x04, 0x73, 0x6c, 0x6f, 0x74,
	0x22, 0x3e, 0x0a, 0x0f, 0x53, 0x6c, 0x6f, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x22, 0x39, 0x0a, 0x0f, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x6e, 0x6b, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x02, 0x69, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x11, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x3b, 0x0a, 0x10, 0x53,
	0x6c, 0x6f, 0x74, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x22, 0x56, 0x0a, 0x0f, 0x53, 0x6c, 0x6f, 0x74,
	0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x61,
	0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65,
	0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d,
	0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f,
	0x69, 0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64,
	0x22, 0x5c, 0x0a, 0x10, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69,
	0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74,
	0x12, 0x1c, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x08,
	0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x52, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x32, 0xf2,
	0x03, 0x0a, 0x0b, 0x53, 0x6c, 0x6f, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x1e,
	0x0a, 0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c,
	0x6f, 0x74, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x1e,
	0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c,
	0x6f, 0x74, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x1a,
	0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x08,
	0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x2a, 0x0a, 0x04, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x16, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4e,
	0x61, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e,
	0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x20, 0x0a, 0x08, 0x4e, 0x61, 0x6d, 0x65, 0x46, 0x75,
	0x6c, 0x6c, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x08, 0x2e, 0x70,
	0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65,
	0x74, 0x65, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e,
	0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x39, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74,
	0x12, 0x16, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73,
	0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x2c, 0x0a, 0x04, 0x4c, 0x69, 0x6e, 0x6b, 0x12, 0x16, 0x2e, 0x63, 0x6f,
	0x72, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x6e, 0x6b, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22,
	0x00, 0x12, 0x2e, 0x0a, 0x05, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x12, 0x17, 0x2e, 0x63, 0x6f, 0x72,
	0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22,
	0x00, 0x12, 0x25, 0x0a, 0x0f, 0x56, 0x69, 0x65, 0x77, 0x57, 0x69, 0x74, 0x68, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x64, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x08, 0x2e, 0x70,
	0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x39, 0x0a, 0x04, 0x50, 0x75, 0x6c, 0x6c,
	0x12, 0x16, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75, 0x6c,
	0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e, 0x63, 0x6f, 0x72, 0x65, 0x73,
	0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x04, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x08, 0x2e, 0x70, 0x62,
	0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f,
	0x6c, 0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2f, 0x70,
	0x62, 0x2f, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x3b, 0x63, 0x6f, 0x72, 0x65, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_cores_slot_service_proto_rawDescOnce sync.Once
	file_cores_slot_service_proto_rawDescData = file_cores_slot_service_proto_rawDesc
)

func file_cores_slot_service_proto_rawDescGZIP() []byte {
	file_cores_slot_service_proto_rawDescOnce.Do(func() {
		file_cores_slot_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_cores_slot_service_proto_rawDescData)
	})
	return file_cores_slot_service_proto_rawDescData
}

var file_cores_slot_service_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_cores_slot_service_proto_goTypes = []any{
	(*SlotListRequest)(nil),  // 0: cores.SlotListRequest
	(*SlotListResponse)(nil), // 1: cores.SlotListResponse
	(*SlotNameRequest)(nil),  // 2: cores.SlotNameRequest
	(*SlotLinkRequest)(nil),  // 3: cores.SlotLinkRequest
	(*SlotCloneRequest)(nil), // 4: cores.SlotCloneRequest
	(*SlotPullRequest)(nil),  // 5: cores.SlotPullRequest
	(*SlotPullResponse)(nil), // 6: cores.SlotPullResponse
	(*pb.Page)(nil),          // 7: pb.Page
	(*pb.Slot)(nil),          // 8: pb.Slot
	(*pb.Id)(nil),            // 9: pb.Id
	(*pb.Name)(nil),          // 10: pb.Name
	(*pb.MyBool)(nil),        // 11: pb.MyBool
}
var file_cores_slot_service_proto_depIdxs = []int32{
	7,  // 0: cores.SlotListRequest.page:type_name -> pb.Page
	7,  // 1: cores.SlotListResponse.page:type_name -> pb.Page
	8,  // 2: cores.SlotListResponse.slot:type_name -> pb.Slot
	8,  // 3: cores.SlotPullResponse.slot:type_name -> pb.Slot
	8,  // 4: cores.SlotService.Create:input_type -> pb.Slot
	8,  // 5: cores.SlotService.Update:input_type -> pb.Slot
	9,  // 6: cores.SlotService.View:input_type -> pb.Id
	2,  // 7: cores.SlotService.Name:input_type -> cores.SlotNameRequest
	10, // 8: cores.SlotService.NameFull:input_type -> pb.Name
	9,  // 9: cores.SlotService.Delete:input_type -> pb.Id
	0,  // 10: cores.SlotService.List:input_type -> cores.SlotListRequest
	3,  // 11: cores.SlotService.Link:input_type -> cores.SlotLinkRequest
	4,  // 12: cores.SlotService.Clone:input_type -> cores.SlotCloneRequest
	9,  // 13: cores.SlotService.ViewWithDeleted:input_type -> pb.Id
	5,  // 14: cores.SlotService.Pull:input_type -> cores.SlotPullRequest
	8,  // 15: cores.SlotService.Sync:input_type -> pb.Slot
	8,  // 16: cores.SlotService.Create:output_type -> pb.Slot
	8,  // 17: cores.SlotService.Update:output_type -> pb.Slot
	8,  // 18: cores.SlotService.View:output_type -> pb.Slot
	8,  // 19: cores.SlotService.Name:output_type -> pb.Slot
	8,  // 20: cores.SlotService.NameFull:output_type -> pb.Slot
	11, // 21: cores.SlotService.Delete:output_type -> pb.MyBool
	1,  // 22: cores.SlotService.List:output_type -> cores.SlotListResponse
	11, // 23: cores.SlotService.Link:output_type -> pb.MyBool
	11, // 24: cores.SlotService.Clone:output_type -> pb.MyBool
	8,  // 25: cores.SlotService.ViewWithDeleted:output_type -> pb.Slot
	6,  // 26: cores.SlotService.Pull:output_type -> cores.SlotPullResponse
	11, // 27: cores.SlotService.Sync:output_type -> pb.MyBool
	16, // [16:28] is the sub-list for method output_type
	4,  // [4:16] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_cores_slot_service_proto_init() }
func file_cores_slot_service_proto_init() {
	if File_cores_slot_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_cores_slot_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_cores_slot_service_proto_goTypes,
		DependencyIndexes: file_cores_slot_service_proto_depIdxs,
		MessageInfos:      file_cores_slot_service_proto_msgTypes,
	}.Build()
	File_cores_slot_service_proto = out.File
	file_cores_slot_service_proto_rawDesc = nil
	file_cores_slot_service_proto_goTypes = nil
	file_cores_slot_service_proto_depIdxs = nil
}

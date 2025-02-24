// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v3.12.4
// source: edges/slot_service.proto

package edges

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
	state protoimpl.MessageState `protogen:"open.v1"`
	Page  *pb.Page               `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	// string device_id = 2;
	Tags          string `protobuf:"bytes,3,opt,name=tags,proto3" json:"tags,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotListRequest) Reset() {
	*x = SlotListRequest{}
	mi := &file_edges_slot_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotListRequest) ProtoMessage() {}

func (x *SlotListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[0]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{0}
}

func (x *SlotListRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
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
	mi := &file_edges_slot_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotListResponse) ProtoMessage() {}

func (x *SlotListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[1]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{1}
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

type SlotLinkRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Status        int32                  `protobuf:"zigzag32,2,opt,name=status,proto3" json:"status,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotLinkRequest) Reset() {
	*x = SlotLinkRequest{}
	mi := &file_edges_slot_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotLinkRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotLinkRequest) ProtoMessage() {}

func (x *SlotLinkRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[2]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{2}
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
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"` // string device_id = 2;
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotCloneRequest) Reset() {
	*x = SlotCloneRequest{}
	mi := &file_edges_slot_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotCloneRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotCloneRequest) ProtoMessage() {}

func (x *SlotCloneRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[3]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{3}
}

func (x *SlotCloneRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type SlotPullRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	After         int64                  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit         uint32                 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"` // string device_id = 3;
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SlotPullRequest) Reset() {
	*x = SlotPullRequest{}
	mi := &file_edges_slot_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotPullRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotPullRequest) ProtoMessage() {}

func (x *SlotPullRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[4]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{4}
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
	mi := &file_edges_slot_service_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SlotPullResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SlotPullResponse) ProtoMessage() {}

func (x *SlotPullResponse) ProtoReflect() protoreflect.Message {
	mi := &file_edges_slot_service_proto_msgTypes[5]
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
	return file_edges_slot_service_proto_rawDescGZIP(), []int{5}
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

var File_edges_slot_service_proto protoreflect.FileDescriptor

var file_edges_slot_service_proto_rawDesc = []byte{
	0x0a, 0x18, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2f, 0x73, 0x6c, 0x6f, 0x74, 0x5f, 0x73, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x65, 0x64, 0x67, 0x65,
	0x73, 0x1a, 0x12, 0x73, 0x6c, 0x6f, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63, 0x5f, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x43, 0x0a, 0x0f,
	0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e,
	0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x61, 0x67,
	0x73, 0x22, 0x64, 0x0a, 0x10, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70,
	0x61, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0d, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x73, 0x6c, 0x6f,
	0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f,
	0x74, 0x52, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x22, 0x39, 0x0a, 0x0f, 0x53, 0x6c, 0x6f, 0x74, 0x4c,
	0x69, 0x6e, 0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x11, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74,
	0x75, 0x73, 0x22, 0x22, 0x0a, 0x10, 0x53, 0x6c, 0x6f, 0x74, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x3d, 0x0a, 0x0f, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75,
	0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74,
	0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12,
	0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x22, 0x5c, 0x0a, 0x10, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75, 0x6c,
	0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74,
	0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12,
	0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x03, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x52, 0x04, 0x73,
	0x6c, 0x6f, 0x74, 0x32, 0xc2, 0x03, 0x0a, 0x0b, 0x53, 0x6c, 0x6f, 0x74, 0x53, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x12, 0x1e, 0x0a, 0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x12, 0x08, 0x2e,
	0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f,
	0x74, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x08, 0x2e,
	0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f,
	0x74, 0x22, 0x00, 0x12, 0x1a, 0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06, 0x2e, 0x70, 0x62,
	0x2e, 0x49, 0x64, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12,
	0x1c, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x61, 0x6d,
	0x65, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x1e, 0x0a,
	0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a,
	0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x39, 0x0a,
	0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x16, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c,
	0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e,
	0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x2c, 0x0a, 0x04, 0x4c, 0x69, 0x6e, 0x6b,
	0x12, 0x16, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x4c, 0x69, 0x6e,
	0x6b, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79,
	0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x2e, 0x0a, 0x05, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x12,
	0x17, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x43, 0x6c, 0x6f, 0x6e,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79,
	0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x25, 0x0a, 0x0f, 0x56, 0x69, 0x65, 0x77, 0x57, 0x69,
	0x74, 0x68, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49,
	0x64, 0x1a, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x39, 0x0a,
	0x04, 0x50, 0x75, 0x6c, 0x6c, 0x12, 0x16, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c,
	0x6f, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x17, 0x2e,
	0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x1e, 0x0a, 0x04, 0x53, 0x79, 0x6e, 0x63,
	0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e,
	0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x62, 0x65, 0x61,
	0x63, 0x6f, 0x6e, 0x2f, 0x70, 0x62, 0x2f, 0x65, 0x64, 0x67, 0x65, 0x73, 0x3b, 0x65, 0x64, 0x67,
	0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_edges_slot_service_proto_rawDescOnce sync.Once
	file_edges_slot_service_proto_rawDescData = file_edges_slot_service_proto_rawDesc
)

func file_edges_slot_service_proto_rawDescGZIP() []byte {
	file_edges_slot_service_proto_rawDescOnce.Do(func() {
		file_edges_slot_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_edges_slot_service_proto_rawDescData)
	})
	return file_edges_slot_service_proto_rawDescData
}

var file_edges_slot_service_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_edges_slot_service_proto_goTypes = []any{
	(*SlotListRequest)(nil),  // 0: edges.SlotListRequest
	(*SlotListResponse)(nil), // 1: edges.SlotListResponse
	(*SlotLinkRequest)(nil),  // 2: edges.SlotLinkRequest
	(*SlotCloneRequest)(nil), // 3: edges.SlotCloneRequest
	(*SlotPullRequest)(nil),  // 4: edges.SlotPullRequest
	(*SlotPullResponse)(nil), // 5: edges.SlotPullResponse
	(*pb.Page)(nil),          // 6: pb.Page
	(*pb.Slot)(nil),          // 7: pb.Slot
	(*pb.Id)(nil),            // 8: pb.Id
	(*pb.Name)(nil),          // 9: pb.Name
	(*pb.MyBool)(nil),        // 10: pb.MyBool
}
var file_edges_slot_service_proto_depIdxs = []int32{
	6,  // 0: edges.SlotListRequest.page:type_name -> pb.Page
	6,  // 1: edges.SlotListResponse.page:type_name -> pb.Page
	7,  // 2: edges.SlotListResponse.slot:type_name -> pb.Slot
	7,  // 3: edges.SlotPullResponse.slot:type_name -> pb.Slot
	7,  // 4: edges.SlotService.Create:input_type -> pb.Slot
	7,  // 5: edges.SlotService.Update:input_type -> pb.Slot
	8,  // 6: edges.SlotService.View:input_type -> pb.Id
	9,  // 7: edges.SlotService.Name:input_type -> pb.Name
	8,  // 8: edges.SlotService.Delete:input_type -> pb.Id
	0,  // 9: edges.SlotService.List:input_type -> edges.SlotListRequest
	2,  // 10: edges.SlotService.Link:input_type -> edges.SlotLinkRequest
	3,  // 11: edges.SlotService.Clone:input_type -> edges.SlotCloneRequest
	8,  // 12: edges.SlotService.ViewWithDeleted:input_type -> pb.Id
	4,  // 13: edges.SlotService.Pull:input_type -> edges.SlotPullRequest
	7,  // 14: edges.SlotService.Sync:input_type -> pb.Slot
	7,  // 15: edges.SlotService.Create:output_type -> pb.Slot
	7,  // 16: edges.SlotService.Update:output_type -> pb.Slot
	7,  // 17: edges.SlotService.View:output_type -> pb.Slot
	7,  // 18: edges.SlotService.Name:output_type -> pb.Slot
	10, // 19: edges.SlotService.Delete:output_type -> pb.MyBool
	1,  // 20: edges.SlotService.List:output_type -> edges.SlotListResponse
	10, // 21: edges.SlotService.Link:output_type -> pb.MyBool
	10, // 22: edges.SlotService.Clone:output_type -> pb.MyBool
	7,  // 23: edges.SlotService.ViewWithDeleted:output_type -> pb.Slot
	5,  // 24: edges.SlotService.Pull:output_type -> edges.SlotPullResponse
	10, // 25: edges.SlotService.Sync:output_type -> pb.MyBool
	15, // [15:26] is the sub-list for method output_type
	4,  // [4:15] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_edges_slot_service_proto_init() }
func file_edges_slot_service_proto_init() {
	if File_edges_slot_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_edges_slot_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_edges_slot_service_proto_goTypes,
		DependencyIndexes: file_edges_slot_service_proto_depIdxs,
		MessageInfos:      file_edges_slot_service_proto_msgTypes,
	}.Build()
	File_edges_slot_service_proto = out.File
	file_edges_slot_service_proto_rawDesc = nil
	file_edges_slot_service_proto_goTypes = nil
	file_edges_slot_service_proto_depIdxs = nil
}

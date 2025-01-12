// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.1
// 	protoc        v3.12.4
// source: edges/const_service.proto

package edges

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

type ConstListRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	Page  *pb.Page               `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	// string device_id = 2;
	Tags          string `protobuf:"bytes,3,opt,name=tags,proto3" json:"tags,omitempty"`
	Type          string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConstListRequest) Reset() {
	*x = ConstListRequest{}
	mi := &file_edges_const_service_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConstListRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstListRequest) ProtoMessage() {}

func (x *ConstListRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_const_service_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstListRequest.ProtoReflect.Descriptor instead.
func (*ConstListRequest) Descriptor() ([]byte, []int) {
	return file_edges_const_service_proto_rawDescGZIP(), []int{0}
}

func (x *ConstListRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ConstListRequest) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *ConstListRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type ConstListResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Page          *pb.Page               `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Count         uint32                 `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Const         []*pb.Const            `protobuf:"bytes,3,rep,name=const,proto3" json:"const,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConstListResponse) Reset() {
	*x = ConstListResponse{}
	mi := &file_edges_const_service_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConstListResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstListResponse) ProtoMessage() {}

func (x *ConstListResponse) ProtoReflect() protoreflect.Message {
	mi := &file_edges_const_service_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstListResponse.ProtoReflect.Descriptor instead.
func (*ConstListResponse) Descriptor() ([]byte, []int) {
	return file_edges_const_service_proto_rawDescGZIP(), []int{1}
}

func (x *ConstListResponse) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ConstListResponse) GetCount() uint32 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ConstListResponse) GetConst() []*pb.Const {
	if x != nil {
		return x.Const
	}
	return nil
}

type ConstCloneRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"` // string device_id = 2;
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConstCloneRequest) Reset() {
	*x = ConstCloneRequest{}
	mi := &file_edges_const_service_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConstCloneRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstCloneRequest) ProtoMessage() {}

func (x *ConstCloneRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_const_service_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstCloneRequest.ProtoReflect.Descriptor instead.
func (*ConstCloneRequest) Descriptor() ([]byte, []int) {
	return file_edges_const_service_proto_rawDescGZIP(), []int{2}
}

func (x *ConstCloneRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

type ConstPullRequest struct {
	state protoimpl.MessageState `protogen:"open.v1"`
	After int64                  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit uint32                 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	// string device_id = 3;
	Type          string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConstPullRequest) Reset() {
	*x = ConstPullRequest{}
	mi := &file_edges_const_service_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConstPullRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstPullRequest) ProtoMessage() {}

func (x *ConstPullRequest) ProtoReflect() protoreflect.Message {
	mi := &file_edges_const_service_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstPullRequest.ProtoReflect.Descriptor instead.
func (*ConstPullRequest) Descriptor() ([]byte, []int) {
	return file_edges_const_service_proto_rawDescGZIP(), []int{3}
}

func (x *ConstPullRequest) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *ConstPullRequest) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ConstPullRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type ConstPullResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	After         int64                  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit         uint32                 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	Const         []*pb.Const            `protobuf:"bytes,3,rep,name=const,proto3" json:"const,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ConstPullResponse) Reset() {
	*x = ConstPullResponse{}
	mi := &file_edges_const_service_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ConstPullResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ConstPullResponse) ProtoMessage() {}

func (x *ConstPullResponse) ProtoReflect() protoreflect.Message {
	mi := &file_edges_const_service_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ConstPullResponse.ProtoReflect.Descriptor instead.
func (*ConstPullResponse) Descriptor() ([]byte, []int) {
	return file_edges_const_service_proto_rawDescGZIP(), []int{4}
}

func (x *ConstPullResponse) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *ConstPullResponse) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *ConstPullResponse) GetConst() []*pb.Const {
	if x != nil {
		return x.Const
	}
	return nil
}

var File_edges_const_service_proto protoreflect.FileDescriptor

var file_edges_const_service_proto_rawDesc = []byte{
	0x0a, 0x19, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2f, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x5f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x65, 0x64, 0x67,
	0x65, 0x73, 0x1a, 0x13, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63,
	0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x58,
	0x0a, 0x10, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x74, 0x61, 0x67, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x68, 0x0a, 0x11, 0x43, 0x6f, 0x6e, 0x73,
	0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a,
	0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62,
	0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x63,
	0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x63, 0x6f, 0x75, 0x6e,
	0x74, 0x12, 0x1f, 0x0a, 0x05, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x05, 0x63, 0x6f, 0x6e,
	0x73, 0x74, 0x22, 0x23, 0x0a, 0x11, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x43, 0x6c, 0x6f, 0x6e, 0x65,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x22, 0x52, 0x0a, 0x10, 0x43, 0x6f, 0x6e, 0x73, 0x74,
	0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x61,
	0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65,
	0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d,
	0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x60, 0x0a, 0x11, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x1f, 0x0a, 0x05,
	0x63, 0x6f, 0x6e, 0x73, 0x74, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x70, 0x62,
	0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x52, 0x05, 0x63, 0x6f, 0x6e, 0x73, 0x74, 0x32, 0xd8, 0x04,
	0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x20,
	0x0a, 0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x12, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f,
	0x6e, 0x73, 0x74, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00,
	0x12, 0x20, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x09, 0x2e, 0x70, 0x62, 0x2e,
	0x43, 0x6f, 0x6e, 0x73, 0x74, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74,
	0x22, 0x00, 0x12, 0x1b, 0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e,
	0x49, 0x64, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00, 0x12,
	0x1d, 0x0a, 0x04, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x61, 0x6d,
	0x65, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x22, 0x00, 0x12, 0x1e,
	0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64,
	0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x3b,
	0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x17, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x4c, 0x69, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x18, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4c, 0x69, 0x73,
	0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x2f, 0x0a, 0x05, 0x43,
	0x6c, 0x6f, 0x6e, 0x65, 0x12, 0x18, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x43, 0x6f, 0x6e,
	0x73, 0x74, 0x43, 0x6c, 0x6f, 0x6e, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0a,
	0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x12, 0x26, 0x0a, 0x0f,
	0x56, 0x69, 0x65, 0x77, 0x57, 0x69, 0x74, 0x68, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x12,
	0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e,
	0x73, 0x74, 0x22, 0x00, 0x12, 0x3b, 0x0a, 0x04, 0x50, 0x75, 0x6c, 0x6c, 0x12, 0x17, 0x2e, 0x65,
	0x64, 0x67, 0x65, 0x73, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x65, 0x64, 0x67, 0x65, 0x73, 0x2e, 0x43, 0x6f,
	0x6e, 0x73, 0x74, 0x50, 0x75, 0x6c, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x1f, 0x0a, 0x04, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x09, 0x2e, 0x70, 0x62, 0x2e, 0x43,
	0x6f, 0x6e, 0x73, 0x74, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c,
	0x22, 0x00, 0x12, 0x24, 0x0a, 0x08, 0x47, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x06,
	0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0e, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73,
	0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x00, 0x12, 0x28, 0x0a, 0x08, 0x53, 0x65, 0x74, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x0e, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c,
	0x22, 0x00, 0x12, 0x30, 0x0a, 0x0e, 0x47, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x79,
	0x4e, 0x61, 0x6d, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x12,
	0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x22, 0x00, 0x12, 0x32, 0x0a, 0x0e, 0x53, 0x65, 0x74, 0x56, 0x61, 0x6c, 0x75, 0x65,
	0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x2e, 0x70, 0x62, 0x2e, 0x43, 0x6f, 0x6e, 0x73,
	0x74, 0x4e, 0x61, 0x6d, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e,
	0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c, 0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x6b, 0x6f, 0x6b,
	0x6f, 0x6d, 0x69, 0x2f, 0x70, 0x62, 0x2f, 0x65, 0x64, 0x67, 0x65, 0x73, 0x3b, 0x65, 0x64, 0x67,
	0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_edges_const_service_proto_rawDescOnce sync.Once
	file_edges_const_service_proto_rawDescData = file_edges_const_service_proto_rawDesc
)

func file_edges_const_service_proto_rawDescGZIP() []byte {
	file_edges_const_service_proto_rawDescOnce.Do(func() {
		file_edges_const_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_edges_const_service_proto_rawDescData)
	})
	return file_edges_const_service_proto_rawDescData
}

var file_edges_const_service_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_edges_const_service_proto_goTypes = []any{
	(*ConstListRequest)(nil),  // 0: edges.ConstListRequest
	(*ConstListResponse)(nil), // 1: edges.ConstListResponse
	(*ConstCloneRequest)(nil), // 2: edges.ConstCloneRequest
	(*ConstPullRequest)(nil),  // 3: edges.ConstPullRequest
	(*ConstPullResponse)(nil), // 4: edges.ConstPullResponse
	(*pb.Page)(nil),           // 5: pb.Page
	(*pb.Const)(nil),          // 6: pb.Const
	(*pb.Id)(nil),             // 7: pb.Id
	(*pb.Name)(nil),           // 8: pb.Name
	(*pb.ConstValue)(nil),     // 9: pb.ConstValue
	(*pb.ConstNameValue)(nil), // 10: pb.ConstNameValue
	(*pb.MyBool)(nil),         // 11: pb.MyBool
}
var file_edges_const_service_proto_depIdxs = []int32{
	5,  // 0: edges.ConstListRequest.page:type_name -> pb.Page
	5,  // 1: edges.ConstListResponse.page:type_name -> pb.Page
	6,  // 2: edges.ConstListResponse.const:type_name -> pb.Const
	6,  // 3: edges.ConstPullResponse.const:type_name -> pb.Const
	6,  // 4: edges.ConstService.Create:input_type -> pb.Const
	6,  // 5: edges.ConstService.Update:input_type -> pb.Const
	7,  // 6: edges.ConstService.View:input_type -> pb.Id
	8,  // 7: edges.ConstService.Name:input_type -> pb.Name
	7,  // 8: edges.ConstService.Delete:input_type -> pb.Id
	0,  // 9: edges.ConstService.List:input_type -> edges.ConstListRequest
	2,  // 10: edges.ConstService.Clone:input_type -> edges.ConstCloneRequest
	7,  // 11: edges.ConstService.ViewWithDeleted:input_type -> pb.Id
	3,  // 12: edges.ConstService.Pull:input_type -> edges.ConstPullRequest
	6,  // 13: edges.ConstService.Sync:input_type -> pb.Const
	7,  // 14: edges.ConstService.GetValue:input_type -> pb.Id
	9,  // 15: edges.ConstService.SetValue:input_type -> pb.ConstValue
	8,  // 16: edges.ConstService.GetValueByName:input_type -> pb.Name
	10, // 17: edges.ConstService.SetValueByName:input_type -> pb.ConstNameValue
	6,  // 18: edges.ConstService.Create:output_type -> pb.Const
	6,  // 19: edges.ConstService.Update:output_type -> pb.Const
	6,  // 20: edges.ConstService.View:output_type -> pb.Const
	6,  // 21: edges.ConstService.Name:output_type -> pb.Const
	11, // 22: edges.ConstService.Delete:output_type -> pb.MyBool
	1,  // 23: edges.ConstService.List:output_type -> edges.ConstListResponse
	11, // 24: edges.ConstService.Clone:output_type -> pb.MyBool
	6,  // 25: edges.ConstService.ViewWithDeleted:output_type -> pb.Const
	4,  // 26: edges.ConstService.Pull:output_type -> edges.ConstPullResponse
	11, // 27: edges.ConstService.Sync:output_type -> pb.MyBool
	9,  // 28: edges.ConstService.GetValue:output_type -> pb.ConstValue
	11, // 29: edges.ConstService.SetValue:output_type -> pb.MyBool
	10, // 30: edges.ConstService.GetValueByName:output_type -> pb.ConstNameValue
	11, // 31: edges.ConstService.SetValueByName:output_type -> pb.MyBool
	18, // [18:32] is the sub-list for method output_type
	4,  // [4:18] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_edges_const_service_proto_init() }
func file_edges_const_service_proto_init() {
	if File_edges_const_service_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_edges_const_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_edges_const_service_proto_goTypes,
		DependencyIndexes: file_edges_const_service_proto_depIdxs,
		MessageInfos:      file_edges_const_service_proto_msgTypes,
	}.Build()
	File_edges_const_service_proto = out.File
	file_edges_const_service_proto_rawDesc = nil
	file_edges_const_service_proto_goTypes = nil
	file_edges_const_service_proto_depIdxs = nil
}

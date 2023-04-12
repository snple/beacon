// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.12.4
// source: slots/option_service.proto

package slots

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

type ListOptionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page *pb.Page `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	// string device_id = 2;
	Tags string `protobuf:"bytes,3,opt,name=tags,proto3" json:"tags,omitempty"`
	Type string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *ListOptionRequest) Reset() {
	*x = ListOptionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_slots_option_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOptionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOptionRequest) ProtoMessage() {}

func (x *ListOptionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_slots_option_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListOptionRequest.ProtoReflect.Descriptor instead.
func (*ListOptionRequest) Descriptor() ([]byte, []int) {
	return file_slots_option_service_proto_rawDescGZIP(), []int{0}
}

func (x *ListOptionRequest) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListOptionRequest) GetTags() string {
	if x != nil {
		return x.Tags
	}
	return ""
}

func (x *ListOptionRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type ListOptionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Page   *pb.Page     `protobuf:"bytes,1,opt,name=page,proto3" json:"page,omitempty"`
	Count  uint32       `protobuf:"varint,2,opt,name=count,proto3" json:"count,omitempty"`
	Option []*pb.Option `protobuf:"bytes,3,rep,name=option,proto3" json:"option,omitempty"`
}

func (x *ListOptionResponse) Reset() {
	*x = ListOptionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_slots_option_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListOptionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListOptionResponse) ProtoMessage() {}

func (x *ListOptionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_slots_option_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListOptionResponse.ProtoReflect.Descriptor instead.
func (*ListOptionResponse) Descriptor() ([]byte, []int) {
	return file_slots_option_service_proto_rawDescGZIP(), []int{1}
}

func (x *ListOptionResponse) GetPage() *pb.Page {
	if x != nil {
		return x.Page
	}
	return nil
}

func (x *ListOptionResponse) GetCount() uint32 {
	if x != nil {
		return x.Count
	}
	return 0
}

func (x *ListOptionResponse) GetOption() []*pb.Option {
	if x != nil {
		return x.Option
	}
	return nil
}

type PullOptionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	After int64  `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit uint32 `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	// string device_id = 3;
	Type string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *PullOptionRequest) Reset() {
	*x = PullOptionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_slots_option_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullOptionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullOptionRequest) ProtoMessage() {}

func (x *PullOptionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_slots_option_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullOptionRequest.ProtoReflect.Descriptor instead.
func (*PullOptionRequest) Descriptor() ([]byte, []int) {
	return file_slots_option_service_proto_rawDescGZIP(), []int{2}
}

func (x *PullOptionRequest) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *PullOptionRequest) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *PullOptionRequest) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type PullOptionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	After  int64        `protobuf:"varint,1,opt,name=after,proto3" json:"after,omitempty"`
	Limit  uint32       `protobuf:"varint,2,opt,name=limit,proto3" json:"limit,omitempty"`
	Option []*pb.Option `protobuf:"bytes,3,rep,name=option,proto3" json:"option,omitempty"`
}

func (x *PullOptionResponse) Reset() {
	*x = PullOptionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_slots_option_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PullOptionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PullOptionResponse) ProtoMessage() {}

func (x *PullOptionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_slots_option_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PullOptionResponse.ProtoReflect.Descriptor instead.
func (*PullOptionResponse) Descriptor() ([]byte, []int) {
	return file_slots_option_service_proto_rawDescGZIP(), []int{3}
}

func (x *PullOptionResponse) GetAfter() int64 {
	if x != nil {
		return x.After
	}
	return 0
}

func (x *PullOptionResponse) GetLimit() uint32 {
	if x != nil {
		return x.Limit
	}
	return 0
}

func (x *PullOptionResponse) GetOption() []*pb.Option {
	if x != nil {
		return x.Option
	}
	return nil
}

var File_slots_option_service_proto protoreflect.FileDescriptor

var file_slots_option_service_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x73, 0x6c, 0x6f, 0x74, 0x73, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x73, 0x6c,
	0x6f, 0x74, 0x73, 0x1a, 0x14, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x15, 0x67, 0x65, 0x6e, 0x65, 0x72,
	0x69, 0x63, 0x5f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x59, 0x0a, 0x11, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70,
	0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x61, 0x67, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x61, 0x67, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x6c, 0x0a, 0x12, 0x4c,
	0x69, 0x73, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x1c, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x08, 0x2e, 0x70, 0x62, 0x2e, 0x50, 0x61, 0x67, 0x65, 0x52, 0x04, 0x70, 0x61, 0x67, 0x65, 0x12,
	0x14, 0x0a, 0x05, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x22, 0x0a, 0x06, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18,
	0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x52, 0x06, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x53, 0x0a, 0x11, 0x50, 0x75, 0x6c,
	0x6c, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x14,
	0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x61,
	0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79,
	0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x64,
	0x0a, 0x12, 0x50, 0x75, 0x6c, 0x6c, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x05, 0x61, 0x66, 0x74, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x69,
	0x6d, 0x69, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x05, 0x6c, 0x69, 0x6d, 0x69, 0x74,
	0x12, 0x22, 0x0a, 0x06, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x6f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x32, 0xe2, 0x02, 0x0a, 0x0d, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x53,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x22, 0x0a, 0x06, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65,
	0x12, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x0a, 0x2e, 0x70,
	0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x12, 0x22, 0x0a, 0x06, 0x55, 0x70,
	0x64, 0x61, 0x74, 0x65, 0x12, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x12, 0x1c,
	0x0a, 0x04, 0x56, 0x69, 0x65, 0x77, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a,
	0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x12, 0x24, 0x0a, 0x0a,
	0x56, 0x69, 0x65, 0x77, 0x42, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x08, 0x2e, 0x70, 0x62, 0x2e,
	0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x22, 0x00, 0x12, 0x1e, 0x0a, 0x06, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x12, 0x06, 0x2e, 0x70,
	0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70, 0x62, 0x2e, 0x4d, 0x79, 0x42, 0x6f, 0x6f, 0x6c,
	0x22, 0x00, 0x12, 0x3d, 0x0a, 0x04, 0x4c, 0x69, 0x73, 0x74, 0x12, 0x18, 0x2e, 0x73, 0x6c, 0x6f,
	0x74, 0x73, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e, 0x73, 0x6c, 0x6f, 0x74, 0x73, 0x2e, 0x4c, 0x69, 0x73,
	0x74, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22,
	0x00, 0x12, 0x27, 0x0a, 0x0f, 0x56, 0x69, 0x65, 0x77, 0x57, 0x69, 0x74, 0x68, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x64, 0x12, 0x06, 0x2e, 0x70, 0x62, 0x2e, 0x49, 0x64, 0x1a, 0x0a, 0x2e, 0x70,
	0x62, 0x2e, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x12, 0x3d, 0x0a, 0x04, 0x50, 0x75,
	0x6c, 0x6c, 0x12, 0x18, 0x2e, 0x73, 0x6c, 0x6f, 0x74, 0x73, 0x2e, 0x50, 0x75, 0x6c, 0x6c, 0x4f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x19, 0x2e, 0x73,
	0x6c, 0x6f, 0x74, 0x73, 0x2e, 0x50, 0x75, 0x6c, 0x6c, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x28, 0x5a, 0x26, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x6e, 0x70, 0x6c, 0x65, 0x2f, 0x6b, 0x6f,
	0x6b, 0x6f, 0x6d, 0x69, 0x2f, 0x70, 0x62, 0x2f, 0x73, 0x6c, 0x6f, 0x74, 0x73, 0x3b, 0x73, 0x6c,
	0x6f, 0x74, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_slots_option_service_proto_rawDescOnce sync.Once
	file_slots_option_service_proto_rawDescData = file_slots_option_service_proto_rawDesc
)

func file_slots_option_service_proto_rawDescGZIP() []byte {
	file_slots_option_service_proto_rawDescOnce.Do(func() {
		file_slots_option_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_slots_option_service_proto_rawDescData)
	})
	return file_slots_option_service_proto_rawDescData
}

var file_slots_option_service_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_slots_option_service_proto_goTypes = []interface{}{
	(*ListOptionRequest)(nil),  // 0: slots.ListOptionRequest
	(*ListOptionResponse)(nil), // 1: slots.ListOptionResponse
	(*PullOptionRequest)(nil),  // 2: slots.PullOptionRequest
	(*PullOptionResponse)(nil), // 3: slots.PullOptionResponse
	(*pb.Page)(nil),            // 4: pb.Page
	(*pb.Option)(nil),          // 5: pb.Option
	(*pb.Id)(nil),              // 6: pb.Id
	(*pb.Name)(nil),            // 7: pb.Name
	(*pb.MyBool)(nil),          // 8: pb.MyBool
}
var file_slots_option_service_proto_depIdxs = []int32{
	4,  // 0: slots.ListOptionRequest.page:type_name -> pb.Page
	4,  // 1: slots.ListOptionResponse.page:type_name -> pb.Page
	5,  // 2: slots.ListOptionResponse.option:type_name -> pb.Option
	5,  // 3: slots.PullOptionResponse.option:type_name -> pb.Option
	5,  // 4: slots.OptionService.Create:input_type -> pb.Option
	5,  // 5: slots.OptionService.Update:input_type -> pb.Option
	6,  // 6: slots.OptionService.View:input_type -> pb.Id
	7,  // 7: slots.OptionService.ViewByName:input_type -> pb.Name
	6,  // 8: slots.OptionService.Delete:input_type -> pb.Id
	0,  // 9: slots.OptionService.List:input_type -> slots.ListOptionRequest
	6,  // 10: slots.OptionService.ViewWithDeleted:input_type -> pb.Id
	2,  // 11: slots.OptionService.Pull:input_type -> slots.PullOptionRequest
	5,  // 12: slots.OptionService.Create:output_type -> pb.Option
	5,  // 13: slots.OptionService.Update:output_type -> pb.Option
	5,  // 14: slots.OptionService.View:output_type -> pb.Option
	5,  // 15: slots.OptionService.ViewByName:output_type -> pb.Option
	8,  // 16: slots.OptionService.Delete:output_type -> pb.MyBool
	1,  // 17: slots.OptionService.List:output_type -> slots.ListOptionResponse
	5,  // 18: slots.OptionService.ViewWithDeleted:output_type -> pb.Option
	3,  // 19: slots.OptionService.Pull:output_type -> slots.PullOptionResponse
	12, // [12:20] is the sub-list for method output_type
	4,  // [4:12] is the sub-list for method input_type
	4,  // [4:4] is the sub-list for extension type_name
	4,  // [4:4] is the sub-list for extension extendee
	0,  // [0:4] is the sub-list for field type_name
}

func init() { file_slots_option_service_proto_init() }
func file_slots_option_service_proto_init() {
	if File_slots_option_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_slots_option_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListOptionRequest); i {
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
		file_slots_option_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListOptionResponse); i {
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
		file_slots_option_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullOptionRequest); i {
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
		file_slots_option_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PullOptionResponse); i {
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
			RawDescriptor: file_slots_option_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_slots_option_service_proto_goTypes,
		DependencyIndexes: file_slots_option_service_proto_depIdxs,
		MessageInfos:      file_slots_option_service_proto_msgTypes,
	}.Build()
	File_slots_option_service_proto = out.File
	file_slots_option_service_proto_rawDesc = nil
	file_slots_option_service_proto_goTypes = nil
	file_slots_option_service_proto_depIdxs = nil
}

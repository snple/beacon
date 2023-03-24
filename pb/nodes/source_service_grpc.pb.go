// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.12.4
// source: nodes/source_service.proto

package nodes

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	pb "snple.com/kokomi/pb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// SourceServiceClient is the client API for SourceService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SourceServiceClient interface {
	Create(ctx context.Context, in *pb.Source, opts ...grpc.CallOption) (*pb.Source, error)
	Update(ctx context.Context, in *pb.Source, opts ...grpc.CallOption) (*pb.Source, error)
	View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Source, error)
	ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Source, error)
	Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	List(ctx context.Context, in *ListSourceRequest, opts ...grpc.CallOption) (*ListSourceResponse, error)
	Link(ctx context.Context, in *LinkSourceRequest, opts ...grpc.CallOption) (*pb.MyBool, error)
	ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Source, error)
	Pull(ctx context.Context, in *PullSourceRequest, opts ...grpc.CallOption) (*PullSourceResponse, error)
}

type sourceServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSourceServiceClient(cc grpc.ClientConnInterface) SourceServiceClient {
	return &sourceServiceClient{cc}
}

func (c *sourceServiceClient) Create(ctx context.Context, in *pb.Source, opts ...grpc.CallOption) (*pb.Source, error) {
	out := new(pb.Source)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) Update(ctx context.Context, in *pb.Source, opts ...grpc.CallOption) (*pb.Source, error) {
	out := new(pb.Source)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Source, error) {
	out := new(pb.Source)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/View", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Source, error) {
	out := new(pb.Source)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/ViewByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) List(ctx context.Context, in *ListSourceRequest, opts ...grpc.CallOption) (*ListSourceResponse, error) {
	out := new(ListSourceResponse)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) Link(ctx context.Context, in *LinkSourceRequest, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/Link", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Source, error) {
	out := new(pb.Source)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/ViewWithDeleted", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *sourceServiceClient) Pull(ctx context.Context, in *PullSourceRequest, opts ...grpc.CallOption) (*PullSourceResponse, error) {
	out := new(PullSourceResponse)
	err := c.cc.Invoke(ctx, "/nodes.SourceService/Pull", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SourceServiceServer is the server API for SourceService service.
// All implementations must embed UnimplementedSourceServiceServer
// for forward compatibility
type SourceServiceServer interface {
	Create(context.Context, *pb.Source) (*pb.Source, error)
	Update(context.Context, *pb.Source) (*pb.Source, error)
	View(context.Context, *pb.Id) (*pb.Source, error)
	ViewByName(context.Context, *pb.Name) (*pb.Source, error)
	Delete(context.Context, *pb.Id) (*pb.MyBool, error)
	List(context.Context, *ListSourceRequest) (*ListSourceResponse, error)
	Link(context.Context, *LinkSourceRequest) (*pb.MyBool, error)
	ViewWithDeleted(context.Context, *pb.Id) (*pb.Source, error)
	Pull(context.Context, *PullSourceRequest) (*PullSourceResponse, error)
	mustEmbedUnimplementedSourceServiceServer()
}

// UnimplementedSourceServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSourceServiceServer struct {
}

func (UnimplementedSourceServiceServer) Create(context.Context, *pb.Source) (*pb.Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedSourceServiceServer) Update(context.Context, *pb.Source) (*pb.Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedSourceServiceServer) View(context.Context, *pb.Id) (*pb.Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method View not implemented")
}
func (UnimplementedSourceServiceServer) ViewByName(context.Context, *pb.Name) (*pb.Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewByName not implemented")
}
func (UnimplementedSourceServiceServer) Delete(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedSourceServiceServer) List(context.Context, *ListSourceRequest) (*ListSourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedSourceServiceServer) Link(context.Context, *LinkSourceRequest) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Link not implemented")
}
func (UnimplementedSourceServiceServer) ViewWithDeleted(context.Context, *pb.Id) (*pb.Source, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewWithDeleted not implemented")
}
func (UnimplementedSourceServiceServer) Pull(context.Context, *PullSourceRequest) (*PullSourceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pull not implemented")
}
func (UnimplementedSourceServiceServer) mustEmbedUnimplementedSourceServiceServer() {}

// UnsafeSourceServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SourceServiceServer will
// result in compilation errors.
type UnsafeSourceServiceServer interface {
	mustEmbedUnimplementedSourceServiceServer()
}

func RegisterSourceServiceServer(s grpc.ServiceRegistrar, srv SourceServiceServer) {
	s.RegisterService(&SourceService_ServiceDesc, srv)
}

func _SourceService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Source)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).Create(ctx, req.(*pb.Source))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Source)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).Update(ctx, req.(*pb.Source))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_View_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).View(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/View",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).View(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_ViewByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).ViewByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/ViewByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).ViewByName(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).Delete(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListSourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).List(ctx, req.(*ListSourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_Link_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LinkSourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).Link(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/Link",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).Link(ctx, req.(*LinkSourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_ViewWithDeleted_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).ViewWithDeleted(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/ViewWithDeleted",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).ViewWithDeleted(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SourceService_Pull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullSourceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SourceServiceServer).Pull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.SourceService/Pull",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SourceServiceServer).Pull(ctx, req.(*PullSourceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SourceService_ServiceDesc is the grpc.ServiceDesc for SourceService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SourceService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nodes.SourceService",
	HandlerType: (*SourceServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _SourceService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _SourceService_Update_Handler,
		},
		{
			MethodName: "View",
			Handler:    _SourceService_View_Handler,
		},
		{
			MethodName: "ViewByName",
			Handler:    _SourceService_ViewByName_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _SourceService_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _SourceService_List_Handler,
		},
		{
			MethodName: "Link",
			Handler:    _SourceService_Link_Handler,
		},
		{
			MethodName: "ViewWithDeleted",
			Handler:    _SourceService_ViewWithDeleted_Handler,
		},
		{
			MethodName: "Pull",
			Handler:    _SourceService_Pull_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodes/source_service.proto",
}

// TagServiceClient is the client API for TagService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TagServiceClient interface {
	Create(ctx context.Context, in *pb.Tag, opts ...grpc.CallOption) (*pb.Tag, error)
	Update(ctx context.Context, in *pb.Tag, opts ...grpc.CallOption) (*pb.Tag, error)
	View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Tag, error)
	ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Tag, error)
	Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	List(ctx context.Context, in *ListTagRequest, opts ...grpc.CallOption) (*ListTagResponse, error)
	GetValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.TagValue, error)
	SetValue(ctx context.Context, in *pb.TagValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	SetValueUnchecked(ctx context.Context, in *pb.TagValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetValueByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.TagNameValue, error)
	SetValueByName(ctx context.Context, in *pb.TagNameValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	SetValueByNameUnchecked(ctx context.Context, in *pb.TagNameValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Tag, error)
	Pull(ctx context.Context, in *PullTagRequest, opts ...grpc.CallOption) (*PullTagResponse, error)
	ViewValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.TagValueUpdated, error)
	DeleteValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	PullValue(ctx context.Context, in *PullTagValueRequest, opts ...grpc.CallOption) (*PullTagValueResponse, error)
}

type tagServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewTagServiceClient(cc grpc.ClientConnInterface) TagServiceClient {
	return &tagServiceClient{cc}
}

func (c *tagServiceClient) Create(ctx context.Context, in *pb.Tag, opts ...grpc.CallOption) (*pb.Tag, error) {
	out := new(pb.Tag)
	err := c.cc.Invoke(ctx, "/nodes.TagService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) Update(ctx context.Context, in *pb.Tag, opts ...grpc.CallOption) (*pb.Tag, error) {
	out := new(pb.Tag)
	err := c.cc.Invoke(ctx, "/nodes.TagService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Tag, error) {
	out := new(pb.Tag)
	err := c.cc.Invoke(ctx, "/nodes.TagService/View", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Tag, error) {
	out := new(pb.Tag)
	err := c.cc.Invoke(ctx, "/nodes.TagService/ViewByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) List(ctx context.Context, in *ListTagRequest, opts ...grpc.CallOption) (*ListTagResponse, error) {
	out := new(ListTagResponse)
	err := c.cc.Invoke(ctx, "/nodes.TagService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) GetValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.TagValue, error) {
	out := new(pb.TagValue)
	err := c.cc.Invoke(ctx, "/nodes.TagService/GetValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) SetValue(ctx context.Context, in *pb.TagValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/SetValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) SetValueUnchecked(ctx context.Context, in *pb.TagValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/SetValueUnchecked", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) GetValueByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.TagNameValue, error) {
	out := new(pb.TagNameValue)
	err := c.cc.Invoke(ctx, "/nodes.TagService/GetValueByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) SetValueByName(ctx context.Context, in *pb.TagNameValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/SetValueByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) SetValueByNameUnchecked(ctx context.Context, in *pb.TagNameValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/SetValueByNameUnchecked", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Tag, error) {
	out := new(pb.Tag)
	err := c.cc.Invoke(ctx, "/nodes.TagService/ViewWithDeleted", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) Pull(ctx context.Context, in *PullTagRequest, opts ...grpc.CallOption) (*PullTagResponse, error) {
	out := new(PullTagResponse)
	err := c.cc.Invoke(ctx, "/nodes.TagService/Pull", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) ViewValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.TagValueUpdated, error) {
	out := new(pb.TagValueUpdated)
	err := c.cc.Invoke(ctx, "/nodes.TagService/ViewValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) DeleteValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.TagService/DeleteValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *tagServiceClient) PullValue(ctx context.Context, in *PullTagValueRequest, opts ...grpc.CallOption) (*PullTagValueResponse, error) {
	out := new(PullTagValueResponse)
	err := c.cc.Invoke(ctx, "/nodes.TagService/PullValue", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TagServiceServer is the server API for TagService service.
// All implementations must embed UnimplementedTagServiceServer
// for forward compatibility
type TagServiceServer interface {
	Create(context.Context, *pb.Tag) (*pb.Tag, error)
	Update(context.Context, *pb.Tag) (*pb.Tag, error)
	View(context.Context, *pb.Id) (*pb.Tag, error)
	ViewByName(context.Context, *pb.Name) (*pb.Tag, error)
	Delete(context.Context, *pb.Id) (*pb.MyBool, error)
	List(context.Context, *ListTagRequest) (*ListTagResponse, error)
	GetValue(context.Context, *pb.Id) (*pb.TagValue, error)
	SetValue(context.Context, *pb.TagValue) (*pb.MyBool, error)
	SetValueUnchecked(context.Context, *pb.TagValue) (*pb.MyBool, error)
	GetValueByName(context.Context, *pb.Name) (*pb.TagNameValue, error)
	SetValueByName(context.Context, *pb.TagNameValue) (*pb.MyBool, error)
	SetValueByNameUnchecked(context.Context, *pb.TagNameValue) (*pb.MyBool, error)
	ViewWithDeleted(context.Context, *pb.Id) (*pb.Tag, error)
	Pull(context.Context, *PullTagRequest) (*PullTagResponse, error)
	ViewValue(context.Context, *pb.Id) (*pb.TagValueUpdated, error)
	DeleteValue(context.Context, *pb.Id) (*pb.MyBool, error)
	PullValue(context.Context, *PullTagValueRequest) (*PullTagValueResponse, error)
	mustEmbedUnimplementedTagServiceServer()
}

// UnimplementedTagServiceServer must be embedded to have forward compatible implementations.
type UnimplementedTagServiceServer struct {
}

func (UnimplementedTagServiceServer) Create(context.Context, *pb.Tag) (*pb.Tag, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedTagServiceServer) Update(context.Context, *pb.Tag) (*pb.Tag, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedTagServiceServer) View(context.Context, *pb.Id) (*pb.Tag, error) {
	return nil, status.Errorf(codes.Unimplemented, "method View not implemented")
}
func (UnimplementedTagServiceServer) ViewByName(context.Context, *pb.Name) (*pb.Tag, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewByName not implemented")
}
func (UnimplementedTagServiceServer) Delete(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedTagServiceServer) List(context.Context, *ListTagRequest) (*ListTagResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedTagServiceServer) GetValue(context.Context, *pb.Id) (*pb.TagValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetValue not implemented")
}
func (UnimplementedTagServiceServer) SetValue(context.Context, *pb.TagValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValue not implemented")
}
func (UnimplementedTagServiceServer) SetValueUnchecked(context.Context, *pb.TagValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueUnchecked not implemented")
}
func (UnimplementedTagServiceServer) GetValueByName(context.Context, *pb.Name) (*pb.TagNameValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetValueByName not implemented")
}
func (UnimplementedTagServiceServer) SetValueByName(context.Context, *pb.TagNameValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueByName not implemented")
}
func (UnimplementedTagServiceServer) SetValueByNameUnchecked(context.Context, *pb.TagNameValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueByNameUnchecked not implemented")
}
func (UnimplementedTagServiceServer) ViewWithDeleted(context.Context, *pb.Id) (*pb.Tag, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewWithDeleted not implemented")
}
func (UnimplementedTagServiceServer) Pull(context.Context, *PullTagRequest) (*PullTagResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pull not implemented")
}
func (UnimplementedTagServiceServer) ViewValue(context.Context, *pb.Id) (*pb.TagValueUpdated, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewValue not implemented")
}
func (UnimplementedTagServiceServer) DeleteValue(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteValue not implemented")
}
func (UnimplementedTagServiceServer) PullValue(context.Context, *PullTagValueRequest) (*PullTagValueResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PullValue not implemented")
}
func (UnimplementedTagServiceServer) mustEmbedUnimplementedTagServiceServer() {}

// UnsafeTagServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to TagServiceServer will
// result in compilation errors.
type UnsafeTagServiceServer interface {
	mustEmbedUnimplementedTagServiceServer()
}

func RegisterTagServiceServer(s grpc.ServiceRegistrar, srv TagServiceServer) {
	s.RegisterService(&TagService_ServiceDesc, srv)
}

func _TagService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Tag)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).Create(ctx, req.(*pb.Tag))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Tag)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).Update(ctx, req.(*pb.Tag))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_View_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).View(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/View",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).View(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_ViewByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).ViewByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/ViewByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).ViewByName(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).Delete(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListTagRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).List(ctx, req.(*ListTagRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_GetValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).GetValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/GetValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).GetValue(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_SetValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.TagValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).SetValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/SetValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).SetValue(ctx, req.(*pb.TagValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_SetValueUnchecked_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.TagValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).SetValueUnchecked(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/SetValueUnchecked",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).SetValueUnchecked(ctx, req.(*pb.TagValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_GetValueByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).GetValueByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/GetValueByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).GetValueByName(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_SetValueByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.TagNameValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).SetValueByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/SetValueByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).SetValueByName(ctx, req.(*pb.TagNameValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_SetValueByNameUnchecked_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.TagNameValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).SetValueByNameUnchecked(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/SetValueByNameUnchecked",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).SetValueByNameUnchecked(ctx, req.(*pb.TagNameValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_ViewWithDeleted_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).ViewWithDeleted(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/ViewWithDeleted",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).ViewWithDeleted(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_Pull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullTagRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).Pull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/Pull",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).Pull(ctx, req.(*PullTagRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_ViewValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).ViewValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/ViewValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).ViewValue(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_DeleteValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).DeleteValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/DeleteValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).DeleteValue(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _TagService_PullValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullTagValueRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TagServiceServer).PullValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.TagService/PullValue",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TagServiceServer).PullValue(ctx, req.(*PullTagValueRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// TagService_ServiceDesc is the grpc.ServiceDesc for TagService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var TagService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nodes.TagService",
	HandlerType: (*TagServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _TagService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _TagService_Update_Handler,
		},
		{
			MethodName: "View",
			Handler:    _TagService_View_Handler,
		},
		{
			MethodName: "ViewByName",
			Handler:    _TagService_ViewByName_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _TagService_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _TagService_List_Handler,
		},
		{
			MethodName: "GetValue",
			Handler:    _TagService_GetValue_Handler,
		},
		{
			MethodName: "SetValue",
			Handler:    _TagService_SetValue_Handler,
		},
		{
			MethodName: "SetValueUnchecked",
			Handler:    _TagService_SetValueUnchecked_Handler,
		},
		{
			MethodName: "GetValueByName",
			Handler:    _TagService_GetValueByName_Handler,
		},
		{
			MethodName: "SetValueByName",
			Handler:    _TagService_SetValueByName_Handler,
		},
		{
			MethodName: "SetValueByNameUnchecked",
			Handler:    _TagService_SetValueByNameUnchecked_Handler,
		},
		{
			MethodName: "ViewWithDeleted",
			Handler:    _TagService_ViewWithDeleted_Handler,
		},
		{
			MethodName: "Pull",
			Handler:    _TagService_Pull_Handler,
		},
		{
			MethodName: "ViewValue",
			Handler:    _TagService_ViewValue_Handler,
		},
		{
			MethodName: "DeleteValue",
			Handler:    _TagService_DeleteValue_Handler,
		},
		{
			MethodName: "PullValue",
			Handler:    _TagService_PullValue_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodes/source_service.proto",
}

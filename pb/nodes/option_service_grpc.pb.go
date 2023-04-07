// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.12.4
// source: nodes/option_service.proto

package nodes

import (
	context "context"
	pb "github.com/snple/kokomi/pb"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// OptionServiceClient is the client API for OptionService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type OptionServiceClient interface {
	Create(ctx context.Context, in *pb.Option, opts ...grpc.CallOption) (*pb.Option, error)
	Update(ctx context.Context, in *pb.Option, opts ...grpc.CallOption) (*pb.Option, error)
	View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Option, error)
	ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Option, error)
	Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	List(ctx context.Context, in *ListOptionRequest, opts ...grpc.CallOption) (*ListOptionResponse, error)
	ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Option, error)
	Pull(ctx context.Context, in *PullOptionRequest, opts ...grpc.CallOption) (*PullOptionResponse, error)
}

type optionServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewOptionServiceClient(cc grpc.ClientConnInterface) OptionServiceClient {
	return &optionServiceClient{cc}
}

func (c *optionServiceClient) Create(ctx context.Context, in *pb.Option, opts ...grpc.CallOption) (*pb.Option, error) {
	out := new(pb.Option)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) Update(ctx context.Context, in *pb.Option, opts ...grpc.CallOption) (*pb.Option, error) {
	out := new(pb.Option)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Option, error) {
	out := new(pb.Option)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/View", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) ViewByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Option, error) {
	out := new(pb.Option)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/ViewByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) List(ctx context.Context, in *ListOptionRequest, opts ...grpc.CallOption) (*ListOptionResponse, error) {
	out := new(ListOptionResponse)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Option, error) {
	out := new(pb.Option)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/ViewWithDeleted", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *optionServiceClient) Pull(ctx context.Context, in *PullOptionRequest, opts ...grpc.CallOption) (*PullOptionResponse, error) {
	out := new(PullOptionResponse)
	err := c.cc.Invoke(ctx, "/nodes.OptionService/Pull", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OptionServiceServer is the server API for OptionService service.
// All implementations must embed UnimplementedOptionServiceServer
// for forward compatibility
type OptionServiceServer interface {
	Create(context.Context, *pb.Option) (*pb.Option, error)
	Update(context.Context, *pb.Option) (*pb.Option, error)
	View(context.Context, *pb.Id) (*pb.Option, error)
	ViewByName(context.Context, *pb.Name) (*pb.Option, error)
	Delete(context.Context, *pb.Id) (*pb.MyBool, error)
	List(context.Context, *ListOptionRequest) (*ListOptionResponse, error)
	ViewWithDeleted(context.Context, *pb.Id) (*pb.Option, error)
	Pull(context.Context, *PullOptionRequest) (*PullOptionResponse, error)
	mustEmbedUnimplementedOptionServiceServer()
}

// UnimplementedOptionServiceServer must be embedded to have forward compatible implementations.
type UnimplementedOptionServiceServer struct {
}

func (UnimplementedOptionServiceServer) Create(context.Context, *pb.Option) (*pb.Option, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedOptionServiceServer) Update(context.Context, *pb.Option) (*pb.Option, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedOptionServiceServer) View(context.Context, *pb.Id) (*pb.Option, error) {
	return nil, status.Errorf(codes.Unimplemented, "method View not implemented")
}
func (UnimplementedOptionServiceServer) ViewByName(context.Context, *pb.Name) (*pb.Option, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewByName not implemented")
}
func (UnimplementedOptionServiceServer) Delete(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedOptionServiceServer) List(context.Context, *ListOptionRequest) (*ListOptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedOptionServiceServer) ViewWithDeleted(context.Context, *pb.Id) (*pb.Option, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewWithDeleted not implemented")
}
func (UnimplementedOptionServiceServer) Pull(context.Context, *PullOptionRequest) (*PullOptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pull not implemented")
}
func (UnimplementedOptionServiceServer) mustEmbedUnimplementedOptionServiceServer() {}

// UnsafeOptionServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OptionServiceServer will
// result in compilation errors.
type UnsafeOptionServiceServer interface {
	mustEmbedUnimplementedOptionServiceServer()
}

func RegisterOptionServiceServer(s grpc.ServiceRegistrar, srv OptionServiceServer) {
	s.RegisterService(&OptionService_ServiceDesc, srv)
}

func _OptionService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Option)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).Create(ctx, req.(*pb.Option))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Option)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).Update(ctx, req.(*pb.Option))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_View_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).View(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/View",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).View(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_ViewByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).ViewByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/ViewByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).ViewByName(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).Delete(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListOptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).List(ctx, req.(*ListOptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_ViewWithDeleted_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).ViewWithDeleted(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/ViewWithDeleted",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).ViewWithDeleted(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _OptionService_Pull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullOptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OptionServiceServer).Pull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodes.OptionService/Pull",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OptionServiceServer).Pull(ctx, req.(*PullOptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// OptionService_ServiceDesc is the grpc.ServiceDesc for OptionService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OptionService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nodes.OptionService",
	HandlerType: (*OptionServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _OptionService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _OptionService_Update_Handler,
		},
		{
			MethodName: "View",
			Handler:    _OptionService_View_Handler,
		},
		{
			MethodName: "ViewByName",
			Handler:    _OptionService_ViewByName_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _OptionService_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _OptionService_List_Handler,
		},
		{
			MethodName: "ViewWithDeleted",
			Handler:    _OptionService_ViewWithDeleted_Handler,
		},
		{
			MethodName: "Pull",
			Handler:    _OptionService_Pull_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodes/option_service.proto",
}

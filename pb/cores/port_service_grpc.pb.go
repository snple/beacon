// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.12.4
// source: cores/port_service.proto

package cores

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

// PortServiceClient is the client API for PortService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PortServiceClient interface {
	Create(ctx context.Context, in *pb.Port, opts ...grpc.CallOption) (*pb.Port, error)
	Update(ctx context.Context, in *pb.Port, opts ...grpc.CallOption) (*pb.Port, error)
	View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Port, error)
	ViewByName(ctx context.Context, in *ViewPortByNameRequest, opts ...grpc.CallOption) (*pb.Port, error)
	ViewByNameFull(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Port, error)
	Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	List(ctx context.Context, in *ListPortRequest, opts ...grpc.CallOption) (*ListPortResponse, error)
	Link(ctx context.Context, in *LinkPortRequest, opts ...grpc.CallOption) (*pb.MyBool, error)
	Clone(ctx context.Context, in *ClonePortRequest, opts ...grpc.CallOption) (*pb.MyBool, error)
	ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Port, error)
	Pull(ctx context.Context, in *PullPortRequest, opts ...grpc.CallOption) (*PullPortResponse, error)
}

type portServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewPortServiceClient(cc grpc.ClientConnInterface) PortServiceClient {
	return &portServiceClient{cc}
}

func (c *portServiceClient) Create(ctx context.Context, in *pb.Port, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) Update(ctx context.Context, in *pb.Port, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/View", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) ViewByName(ctx context.Context, in *ViewPortByNameRequest, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/ViewByName", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) ViewByNameFull(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/ViewByNameFull", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/cores.PortService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) List(ctx context.Context, in *ListPortRequest, opts ...grpc.CallOption) (*ListPortResponse, error) {
	out := new(ListPortResponse)
	err := c.cc.Invoke(ctx, "/cores.PortService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) Link(ctx context.Context, in *LinkPortRequest, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/cores.PortService/Link", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) Clone(ctx context.Context, in *ClonePortRequest, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, "/cores.PortService/Clone", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Port, error) {
	out := new(pb.Port)
	err := c.cc.Invoke(ctx, "/cores.PortService/ViewWithDeleted", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *portServiceClient) Pull(ctx context.Context, in *PullPortRequest, opts ...grpc.CallOption) (*PullPortResponse, error) {
	out := new(PullPortResponse)
	err := c.cc.Invoke(ctx, "/cores.PortService/Pull", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PortServiceServer is the server API for PortService service.
// All implementations must embed UnimplementedPortServiceServer
// for forward compatibility
type PortServiceServer interface {
	Create(context.Context, *pb.Port) (*pb.Port, error)
	Update(context.Context, *pb.Port) (*pb.Port, error)
	View(context.Context, *pb.Id) (*pb.Port, error)
	ViewByName(context.Context, *ViewPortByNameRequest) (*pb.Port, error)
	ViewByNameFull(context.Context, *pb.Name) (*pb.Port, error)
	Delete(context.Context, *pb.Id) (*pb.MyBool, error)
	List(context.Context, *ListPortRequest) (*ListPortResponse, error)
	Link(context.Context, *LinkPortRequest) (*pb.MyBool, error)
	Clone(context.Context, *ClonePortRequest) (*pb.MyBool, error)
	ViewWithDeleted(context.Context, *pb.Id) (*pb.Port, error)
	Pull(context.Context, *PullPortRequest) (*PullPortResponse, error)
	mustEmbedUnimplementedPortServiceServer()
}

// UnimplementedPortServiceServer must be embedded to have forward compatible implementations.
type UnimplementedPortServiceServer struct {
}

func (UnimplementedPortServiceServer) Create(context.Context, *pb.Port) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedPortServiceServer) Update(context.Context, *pb.Port) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedPortServiceServer) View(context.Context, *pb.Id) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method View not implemented")
}
func (UnimplementedPortServiceServer) ViewByName(context.Context, *ViewPortByNameRequest) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewByName not implemented")
}
func (UnimplementedPortServiceServer) ViewByNameFull(context.Context, *pb.Name) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewByNameFull not implemented")
}
func (UnimplementedPortServiceServer) Delete(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedPortServiceServer) List(context.Context, *ListPortRequest) (*ListPortResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedPortServiceServer) Link(context.Context, *LinkPortRequest) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Link not implemented")
}
func (UnimplementedPortServiceServer) Clone(context.Context, *ClonePortRequest) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Clone not implemented")
}
func (UnimplementedPortServiceServer) ViewWithDeleted(context.Context, *pb.Id) (*pb.Port, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewWithDeleted not implemented")
}
func (UnimplementedPortServiceServer) Pull(context.Context, *PullPortRequest) (*PullPortResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pull not implemented")
}
func (UnimplementedPortServiceServer) mustEmbedUnimplementedPortServiceServer() {}

// UnsafePortServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PortServiceServer will
// result in compilation errors.
type UnsafePortServiceServer interface {
	mustEmbedUnimplementedPortServiceServer()
}

func RegisterPortServiceServer(s grpc.ServiceRegistrar, srv PortServiceServer) {
	s.RegisterService(&PortService_ServiceDesc, srv)
}

func _PortService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Port)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Create(ctx, req.(*pb.Port))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Port)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Update(ctx, req.(*pb.Port))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_View_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).View(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/View",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).View(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_ViewByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ViewPortByNameRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).ViewByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/ViewByName",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).ViewByName(ctx, req.(*ViewPortByNameRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_ViewByNameFull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).ViewByNameFull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/ViewByNameFull",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).ViewByNameFull(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Delete(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListPortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).List(ctx, req.(*ListPortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_Link_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(LinkPortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Link(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Link",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Link(ctx, req.(*LinkPortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_Clone_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClonePortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Clone(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Clone",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Clone(ctx, req.(*ClonePortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_ViewWithDeleted_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).ViewWithDeleted(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/ViewWithDeleted",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).ViewWithDeleted(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _PortService_Pull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PullPortRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PortServiceServer).Pull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/cores.PortService/Pull",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PortServiceServer).Pull(ctx, req.(*PullPortRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// PortService_ServiceDesc is the grpc.ServiceDesc for PortService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PortService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "cores.PortService",
	HandlerType: (*PortServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _PortService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _PortService_Update_Handler,
		},
		{
			MethodName: "View",
			Handler:    _PortService_View_Handler,
		},
		{
			MethodName: "ViewByName",
			Handler:    _PortService_ViewByName_Handler,
		},
		{
			MethodName: "ViewByNameFull",
			Handler:    _PortService_ViewByNameFull_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _PortService_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _PortService_List_Handler,
		},
		{
			MethodName: "Link",
			Handler:    _PortService_Link_Handler,
		},
		{
			MethodName: "Clone",
			Handler:    _PortService_Clone_Handler,
		},
		{
			MethodName: "ViewWithDeleted",
			Handler:    _PortService_ViewWithDeleted_Handler,
		},
		{
			MethodName: "Pull",
			Handler:    _PortService_Pull_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "cores/port_service.proto",
}

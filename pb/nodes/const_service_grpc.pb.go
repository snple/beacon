// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v3.12.4
// source: nodes/const_service.proto

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

const (
	ConstService_Create_FullMethodName                  = "/nodes.ConstService/Create"
	ConstService_Update_FullMethodName                  = "/nodes.ConstService/Update"
	ConstService_View_FullMethodName                    = "/nodes.ConstService/View"
	ConstService_Name_FullMethodName                    = "/nodes.ConstService/Name"
	ConstService_Delete_FullMethodName                  = "/nodes.ConstService/Delete"
	ConstService_List_FullMethodName                    = "/nodes.ConstService/List"
	ConstService_ViewWithDeleted_FullMethodName         = "/nodes.ConstService/ViewWithDeleted"
	ConstService_Pull_FullMethodName                    = "/nodes.ConstService/Pull"
	ConstService_Sync_FullMethodName                    = "/nodes.ConstService/Sync"
	ConstService_GetValue_FullMethodName                = "/nodes.ConstService/GetValue"
	ConstService_SetValue_FullMethodName                = "/nodes.ConstService/SetValue"
	ConstService_SetValueUnchecked_FullMethodName       = "/nodes.ConstService/SetValueUnchecked"
	ConstService_GetValueByName_FullMethodName          = "/nodes.ConstService/GetValueByName"
	ConstService_SetValueByName_FullMethodName          = "/nodes.ConstService/SetValueByName"
	ConstService_SetValueByNameUnchecked_FullMethodName = "/nodes.ConstService/SetValueByNameUnchecked"
)

// ConstServiceClient is the client API for ConstService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ConstServiceClient interface {
	Create(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.Const, error)
	Update(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.Const, error)
	View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Const, error)
	Name(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Const, error)
	Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error)
	List(ctx context.Context, in *ConstListRequest, opts ...grpc.CallOption) (*ConstListResponse, error)
	ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Const, error)
	Pull(ctx context.Context, in *ConstPullRequest, opts ...grpc.CallOption) (*ConstPullResponse, error)
	Sync(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.ConstValue, error)
	SetValue(ctx context.Context, in *pb.ConstValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	SetValueUnchecked(ctx context.Context, in *pb.ConstValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetValueByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.ConstNameValue, error)
	SetValueByName(ctx context.Context, in *pb.ConstNameValue, opts ...grpc.CallOption) (*pb.MyBool, error)
	SetValueByNameUnchecked(ctx context.Context, in *pb.ConstNameValue, opts ...grpc.CallOption) (*pb.MyBool, error)
}

type constServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewConstServiceClient(cc grpc.ClientConnInterface) ConstServiceClient {
	return &constServiceClient{cc}
}

func (c *constServiceClient) Create(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.Const, error) {
	out := new(pb.Const)
	err := c.cc.Invoke(ctx, ConstService_Create_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) Update(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.Const, error) {
	out := new(pb.Const)
	err := c.cc.Invoke(ctx, ConstService_Update_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) View(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Const, error) {
	out := new(pb.Const)
	err := c.cc.Invoke(ctx, ConstService_View_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) Name(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.Const, error) {
	out := new(pb.Const)
	err := c.cc.Invoke(ctx, ConstService_Name_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) Delete(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_Delete_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) List(ctx context.Context, in *ConstListRequest, opts ...grpc.CallOption) (*ConstListResponse, error) {
	out := new(ConstListResponse)
	err := c.cc.Invoke(ctx, ConstService_List_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) ViewWithDeleted(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.Const, error) {
	out := new(pb.Const)
	err := c.cc.Invoke(ctx, ConstService_ViewWithDeleted_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) Pull(ctx context.Context, in *ConstPullRequest, opts ...grpc.CallOption) (*ConstPullResponse, error) {
	out := new(ConstPullResponse)
	err := c.cc.Invoke(ctx, ConstService_Pull_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) Sync(ctx context.Context, in *pb.Const, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_Sync_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) GetValue(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*pb.ConstValue, error) {
	out := new(pb.ConstValue)
	err := c.cc.Invoke(ctx, ConstService_GetValue_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) SetValue(ctx context.Context, in *pb.ConstValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_SetValue_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) SetValueUnchecked(ctx context.Context, in *pb.ConstValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_SetValueUnchecked_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) GetValueByName(ctx context.Context, in *pb.Name, opts ...grpc.CallOption) (*pb.ConstNameValue, error) {
	out := new(pb.ConstNameValue)
	err := c.cc.Invoke(ctx, ConstService_GetValueByName_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) SetValueByName(ctx context.Context, in *pb.ConstNameValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_SetValueByName_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *constServiceClient) SetValueByNameUnchecked(ctx context.Context, in *pb.ConstNameValue, opts ...grpc.CallOption) (*pb.MyBool, error) {
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, ConstService_SetValueByNameUnchecked_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ConstServiceServer is the server API for ConstService service.
// All implementations must embed UnimplementedConstServiceServer
// for forward compatibility
type ConstServiceServer interface {
	Create(context.Context, *pb.Const) (*pb.Const, error)
	Update(context.Context, *pb.Const) (*pb.Const, error)
	View(context.Context, *pb.Id) (*pb.Const, error)
	Name(context.Context, *pb.Name) (*pb.Const, error)
	Delete(context.Context, *pb.Id) (*pb.MyBool, error)
	List(context.Context, *ConstListRequest) (*ConstListResponse, error)
	ViewWithDeleted(context.Context, *pb.Id) (*pb.Const, error)
	Pull(context.Context, *ConstPullRequest) (*ConstPullResponse, error)
	Sync(context.Context, *pb.Const) (*pb.MyBool, error)
	GetValue(context.Context, *pb.Id) (*pb.ConstValue, error)
	SetValue(context.Context, *pb.ConstValue) (*pb.MyBool, error)
	SetValueUnchecked(context.Context, *pb.ConstValue) (*pb.MyBool, error)
	GetValueByName(context.Context, *pb.Name) (*pb.ConstNameValue, error)
	SetValueByName(context.Context, *pb.ConstNameValue) (*pb.MyBool, error)
	SetValueByNameUnchecked(context.Context, *pb.ConstNameValue) (*pb.MyBool, error)
	mustEmbedUnimplementedConstServiceServer()
}

// UnimplementedConstServiceServer must be embedded to have forward compatible implementations.
type UnimplementedConstServiceServer struct {
}

func (UnimplementedConstServiceServer) Create(context.Context, *pb.Const) (*pb.Const, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedConstServiceServer) Update(context.Context, *pb.Const) (*pb.Const, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedConstServiceServer) View(context.Context, *pb.Id) (*pb.Const, error) {
	return nil, status.Errorf(codes.Unimplemented, "method View not implemented")
}
func (UnimplementedConstServiceServer) Name(context.Context, *pb.Name) (*pb.Const, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Name not implemented")
}
func (UnimplementedConstServiceServer) Delete(context.Context, *pb.Id) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedConstServiceServer) List(context.Context, *ConstListRequest) (*ConstListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedConstServiceServer) ViewWithDeleted(context.Context, *pb.Id) (*pb.Const, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ViewWithDeleted not implemented")
}
func (UnimplementedConstServiceServer) Pull(context.Context, *ConstPullRequest) (*ConstPullResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Pull not implemented")
}
func (UnimplementedConstServiceServer) Sync(context.Context, *pb.Const) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Sync not implemented")
}
func (UnimplementedConstServiceServer) GetValue(context.Context, *pb.Id) (*pb.ConstValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetValue not implemented")
}
func (UnimplementedConstServiceServer) SetValue(context.Context, *pb.ConstValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValue not implemented")
}
func (UnimplementedConstServiceServer) SetValueUnchecked(context.Context, *pb.ConstValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueUnchecked not implemented")
}
func (UnimplementedConstServiceServer) GetValueByName(context.Context, *pb.Name) (*pb.ConstNameValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetValueByName not implemented")
}
func (UnimplementedConstServiceServer) SetValueByName(context.Context, *pb.ConstNameValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueByName not implemented")
}
func (UnimplementedConstServiceServer) SetValueByNameUnchecked(context.Context, *pb.ConstNameValue) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetValueByNameUnchecked not implemented")
}
func (UnimplementedConstServiceServer) mustEmbedUnimplementedConstServiceServer() {}

// UnsafeConstServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ConstServiceServer will
// result in compilation errors.
type UnsafeConstServiceServer interface {
	mustEmbedUnimplementedConstServiceServer()
}

func RegisterConstServiceServer(s grpc.ServiceRegistrar, srv ConstServiceServer) {
	s.RegisterService(&ConstService_ServiceDesc, srv)
}

func _ConstService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Const)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Create_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Create(ctx, req.(*pb.Const))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Const)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Update_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Update(ctx, req.(*pb.Const))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_View_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).View(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_View_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).View(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_Name_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Name(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Name_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Name(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Delete(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConstListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_List_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).List(ctx, req.(*ConstListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_ViewWithDeleted_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).ViewWithDeleted(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_ViewWithDeleted_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).ViewWithDeleted(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_Pull_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConstPullRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Pull(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Pull_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Pull(ctx, req.(*ConstPullRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_Sync_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Const)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).Sync(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_Sync_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).Sync(ctx, req.(*pb.Const))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_GetValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).GetValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_GetValue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).GetValue(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_SetValue_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.ConstValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).SetValue(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_SetValue_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).SetValue(ctx, req.(*pb.ConstValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_SetValueUnchecked_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.ConstValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).SetValueUnchecked(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_SetValueUnchecked_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).SetValueUnchecked(ctx, req.(*pb.ConstValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_GetValueByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Name)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).GetValueByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_GetValueByName_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).GetValueByName(ctx, req.(*pb.Name))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_SetValueByName_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.ConstNameValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).SetValueByName(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_SetValueByName_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).SetValueByName(ctx, req.(*pb.ConstNameValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _ConstService_SetValueByNameUnchecked_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.ConstNameValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ConstServiceServer).SetValueByNameUnchecked(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ConstService_SetValueByNameUnchecked_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ConstServiceServer).SetValueByNameUnchecked(ctx, req.(*pb.ConstNameValue))
	}
	return interceptor(ctx, in, info, handler)
}

// ConstService_ServiceDesc is the grpc.ServiceDesc for ConstService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ConstService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "nodes.ConstService",
	HandlerType: (*ConstServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _ConstService_Create_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _ConstService_Update_Handler,
		},
		{
			MethodName: "View",
			Handler:    _ConstService_View_Handler,
		},
		{
			MethodName: "Name",
			Handler:    _ConstService_Name_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _ConstService_Delete_Handler,
		},
		{
			MethodName: "List",
			Handler:    _ConstService_List_Handler,
		},
		{
			MethodName: "ViewWithDeleted",
			Handler:    _ConstService_ViewWithDeleted_Handler,
		},
		{
			MethodName: "Pull",
			Handler:    _ConstService_Pull_Handler,
		},
		{
			MethodName: "Sync",
			Handler:    _ConstService_Sync_Handler,
		},
		{
			MethodName: "GetValue",
			Handler:    _ConstService_GetValue_Handler,
		},
		{
			MethodName: "SetValue",
			Handler:    _ConstService_SetValue_Handler,
		},
		{
			MethodName: "SetValueUnchecked",
			Handler:    _ConstService_SetValueUnchecked_Handler,
		},
		{
			MethodName: "GetValueByName",
			Handler:    _ConstService_GetValueByName_Handler,
		},
		{
			MethodName: "SetValueByName",
			Handler:    _ConstService_SetValueByName_Handler,
		},
		{
			MethodName: "SetValueByNameUnchecked",
			Handler:    _ConstService_SetValueByNameUnchecked_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodes/const_service.proto",
}

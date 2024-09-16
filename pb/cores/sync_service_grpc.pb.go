// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v3.12.4
// source: cores/sync_service.proto

package cores

import (
	context "context"
	pb "github.com/snple/kokomi/pb"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	SyncService_SetDeviceUpdated_FullMethodName    = "/cores.SyncService/SetDeviceUpdated"
	SyncService_GetDeviceUpdated_FullMethodName    = "/cores.SyncService/GetDeviceUpdated"
	SyncService_WaitDeviceUpdated_FullMethodName   = "/cores.SyncService/WaitDeviceUpdated"
	SyncService_SetTagValueUpdated_FullMethodName  = "/cores.SyncService/SetTagValueUpdated"
	SyncService_GetTagValueUpdated_FullMethodName  = "/cores.SyncService/GetTagValueUpdated"
	SyncService_WaitTagValueUpdated_FullMethodName = "/cores.SyncService/WaitTagValueUpdated"
)

// SyncServiceClient is the client API for SyncService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SyncServiceClient interface {
	SetDeviceUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetDeviceUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error)
	WaitDeviceUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error)
	SetTagValueUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetTagValueUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error)
	WaitTagValueUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error)
}

type syncServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncServiceClient(cc grpc.ClientConnInterface) SyncServiceClient {
	return &syncServiceClient{cc}
}

func (c *syncServiceClient) SetDeviceUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, SyncService_SetDeviceUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncServiceClient) GetDeviceUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SyncUpdated)
	err := c.cc.Invoke(ctx, SyncService_GetDeviceUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncServiceClient) WaitDeviceUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &SyncService_ServiceDesc.Streams[0], SyncService_WaitDeviceUpdated_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[pb.Id, pb.MyBool]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncService_WaitDeviceUpdatedClient = grpc.ServerStreamingClient[pb.MyBool]

func (c *syncServiceClient) SetTagValueUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, SyncService_SetTagValueUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncServiceClient) GetTagValueUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SyncUpdated)
	err := c.cc.Invoke(ctx, SyncService_GetTagValueUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncServiceClient) WaitTagValueUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &SyncService_ServiceDesc.Streams[1], SyncService_WaitTagValueUpdated_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[pb.Id, pb.MyBool]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncService_WaitTagValueUpdatedClient = grpc.ServerStreamingClient[pb.MyBool]

// SyncServiceServer is the server API for SyncService service.
// All implementations must embed UnimplementedSyncServiceServer
// for forward compatibility.
type SyncServiceServer interface {
	SetDeviceUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error)
	GetDeviceUpdated(context.Context, *pb.Id) (*SyncUpdated, error)
	WaitDeviceUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error
	SetTagValueUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error)
	GetTagValueUpdated(context.Context, *pb.Id) (*SyncUpdated, error)
	WaitTagValueUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error
	mustEmbedUnimplementedSyncServiceServer()
}

// UnimplementedSyncServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSyncServiceServer struct{}

func (UnimplementedSyncServiceServer) SetDeviceUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetDeviceUpdated not implemented")
}
func (UnimplementedSyncServiceServer) GetDeviceUpdated(context.Context, *pb.Id) (*SyncUpdated, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDeviceUpdated not implemented")
}
func (UnimplementedSyncServiceServer) WaitDeviceUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error {
	return status.Errorf(codes.Unimplemented, "method WaitDeviceUpdated not implemented")
}
func (UnimplementedSyncServiceServer) SetTagValueUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetTagValueUpdated not implemented")
}
func (UnimplementedSyncServiceServer) GetTagValueUpdated(context.Context, *pb.Id) (*SyncUpdated, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTagValueUpdated not implemented")
}
func (UnimplementedSyncServiceServer) WaitTagValueUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error {
	return status.Errorf(codes.Unimplemented, "method WaitTagValueUpdated not implemented")
}
func (UnimplementedSyncServiceServer) mustEmbedUnimplementedSyncServiceServer() {}
func (UnimplementedSyncServiceServer) testEmbeddedByValue()                     {}

// UnsafeSyncServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SyncServiceServer will
// result in compilation errors.
type UnsafeSyncServiceServer interface {
	mustEmbedUnimplementedSyncServiceServer()
}

func RegisterSyncServiceServer(s grpc.ServiceRegistrar, srv SyncServiceServer) {
	// If the following call pancis, it indicates UnimplementedSyncServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SyncService_ServiceDesc, srv)
}

func _SyncService_SetDeviceUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncUpdated)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncServiceServer).SetDeviceUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncService_SetDeviceUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncServiceServer).SetDeviceUpdated(ctx, req.(*SyncUpdated))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncService_GetDeviceUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncServiceServer).GetDeviceUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncService_GetDeviceUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncServiceServer).GetDeviceUpdated(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncService_WaitDeviceUpdated_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(pb.Id)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SyncServiceServer).WaitDeviceUpdated(m, &grpc.GenericServerStream[pb.Id, pb.MyBool]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncService_WaitDeviceUpdatedServer = grpc.ServerStreamingServer[pb.MyBool]

func _SyncService_SetTagValueUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncUpdated)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncServiceServer).SetTagValueUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncService_SetTagValueUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncServiceServer).SetTagValueUpdated(ctx, req.(*SyncUpdated))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncService_GetTagValueUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncServiceServer).GetTagValueUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncService_GetTagValueUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncServiceServer).GetTagValueUpdated(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncService_WaitTagValueUpdated_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(pb.Id)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SyncServiceServer).WaitTagValueUpdated(m, &grpc.GenericServerStream[pb.Id, pb.MyBool]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncService_WaitTagValueUpdatedServer = grpc.ServerStreamingServer[pb.MyBool]

// SyncService_ServiceDesc is the grpc.ServiceDesc for SyncService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SyncService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "cores.SyncService",
	HandlerType: (*SyncServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetDeviceUpdated",
			Handler:    _SyncService_SetDeviceUpdated_Handler,
		},
		{
			MethodName: "GetDeviceUpdated",
			Handler:    _SyncService_GetDeviceUpdated_Handler,
		},
		{
			MethodName: "SetTagValueUpdated",
			Handler:    _SyncService_SetTagValueUpdated_Handler,
		},
		{
			MethodName: "GetTagValueUpdated",
			Handler:    _SyncService_GetTagValueUpdated_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "WaitDeviceUpdated",
			Handler:       _SyncService_WaitDeviceUpdated_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "WaitTagValueUpdated",
			Handler:       _SyncService_WaitTagValueUpdated_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "cores/sync_service.proto",
}

const (
	SyncGlobalService_SetUpdated_FullMethodName  = "/cores.SyncGlobalService/SetUpdated"
	SyncGlobalService_GetUpdated_FullMethodName  = "/cores.SyncGlobalService/GetUpdated"
	SyncGlobalService_WaitUpdated_FullMethodName = "/cores.SyncGlobalService/WaitUpdated"
)

// SyncGlobalServiceClient is the client API for SyncGlobalService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SyncGlobalServiceClient interface {
	SetUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error)
	GetUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error)
	WaitUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error)
}

type syncGlobalServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSyncGlobalServiceClient(cc grpc.ClientConnInterface) SyncGlobalServiceClient {
	return &syncGlobalServiceClient{cc}
}

func (c *syncGlobalServiceClient) SetUpdated(ctx context.Context, in *SyncUpdated, opts ...grpc.CallOption) (*pb.MyBool, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(pb.MyBool)
	err := c.cc.Invoke(ctx, SyncGlobalService_SetUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncGlobalServiceClient) GetUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (*SyncUpdated, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SyncUpdated)
	err := c.cc.Invoke(ctx, SyncGlobalService_GetUpdated_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *syncGlobalServiceClient) WaitUpdated(ctx context.Context, in *pb.Id, opts ...grpc.CallOption) (grpc.ServerStreamingClient[pb.MyBool], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &SyncGlobalService_ServiceDesc.Streams[0], SyncGlobalService_WaitUpdated_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[pb.Id, pb.MyBool]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncGlobalService_WaitUpdatedClient = grpc.ServerStreamingClient[pb.MyBool]

// SyncGlobalServiceServer is the server API for SyncGlobalService service.
// All implementations must embed UnimplementedSyncGlobalServiceServer
// for forward compatibility.
type SyncGlobalServiceServer interface {
	SetUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error)
	GetUpdated(context.Context, *pb.Id) (*SyncUpdated, error)
	WaitUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error
	mustEmbedUnimplementedSyncGlobalServiceServer()
}

// UnimplementedSyncGlobalServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedSyncGlobalServiceServer struct{}

func (UnimplementedSyncGlobalServiceServer) SetUpdated(context.Context, *SyncUpdated) (*pb.MyBool, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetUpdated not implemented")
}
func (UnimplementedSyncGlobalServiceServer) GetUpdated(context.Context, *pb.Id) (*SyncUpdated, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUpdated not implemented")
}
func (UnimplementedSyncGlobalServiceServer) WaitUpdated(*pb.Id, grpc.ServerStreamingServer[pb.MyBool]) error {
	return status.Errorf(codes.Unimplemented, "method WaitUpdated not implemented")
}
func (UnimplementedSyncGlobalServiceServer) mustEmbedUnimplementedSyncGlobalServiceServer() {}
func (UnimplementedSyncGlobalServiceServer) testEmbeddedByValue()                           {}

// UnsafeSyncGlobalServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SyncGlobalServiceServer will
// result in compilation errors.
type UnsafeSyncGlobalServiceServer interface {
	mustEmbedUnimplementedSyncGlobalServiceServer()
}

func RegisterSyncGlobalServiceServer(s grpc.ServiceRegistrar, srv SyncGlobalServiceServer) {
	// If the following call pancis, it indicates UnimplementedSyncGlobalServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&SyncGlobalService_ServiceDesc, srv)
}

func _SyncGlobalService_SetUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncUpdated)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncGlobalServiceServer).SetUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncGlobalService_SetUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncGlobalServiceServer).SetUpdated(ctx, req.(*SyncUpdated))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncGlobalService_GetUpdated_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(pb.Id)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SyncGlobalServiceServer).GetUpdated(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SyncGlobalService_GetUpdated_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SyncGlobalServiceServer).GetUpdated(ctx, req.(*pb.Id))
	}
	return interceptor(ctx, in, info, handler)
}

func _SyncGlobalService_WaitUpdated_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(pb.Id)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(SyncGlobalServiceServer).WaitUpdated(m, &grpc.GenericServerStream[pb.Id, pb.MyBool]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type SyncGlobalService_WaitUpdatedServer = grpc.ServerStreamingServer[pb.MyBool]

// SyncGlobalService_ServiceDesc is the grpc.ServiceDesc for SyncGlobalService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SyncGlobalService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "cores.SyncGlobalService",
	HandlerType: (*SyncGlobalServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetUpdated",
			Handler:    _SyncGlobalService_SetUpdated_Handler,
		},
		{
			MethodName: "GetUpdated",
			Handler:    _SyncGlobalService_GetUpdated_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "WaitUpdated",
			Handler:       _SyncGlobalService_WaitUpdated_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "cores/sync_service.proto",
}

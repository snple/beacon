package node

import (
	"context"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/pb/nodes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	ns *NodeService

	nodes.UnimplementedPinServiceServer
}

func newPinService(ns *NodeService) *PinService {
	return &PinService{
		ns: ns,
	}
}

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinNameRequest{NodeId: nodeID, Name: in.Name}

	reply, err := s.ns.Core().GetPin().Name(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *PinService) List(ctx context.Context, in *nodes.PinListRequest) (*nodes.PinListResponse, error) {
	var err error
	var output nodes.PinListResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinListRequest{
		Page:   in.GetPage(),
		NodeId: nodeID,
		WireId: in.WireId,
		Tags:   in.Tags,
	}

	reply, err := s.ns.Core().GetPin().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Count = reply.Count
	output.Page = reply.Page
	output.Pins = reply.Pins

	return &output, nil
}

// pinPushStreamWrapper 包装 node 端的流以添加 NodeID 并转发到 Core
type pinPushStreamWrapper struct {
	grpc.ClientStreamingServer[pb.Pin, pb.MyBool]
	nodeID string
}

func (w *pinPushStreamWrapper) Recv() (*pb.Pin, error) {
	in, err := w.ClientStreamingServer.Recv()
	if err != nil {
		return nil, err
	}
	in.NodeId = w.nodeID
	return in, nil
}

func (s *PinService) Push(stream grpc.ClientStreamingServer[pb.Pin, pb.MyBool]) error {
	ctx := stream.Context()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return err
	}

	// 创建包装器流，自动添加 NodeID
	wrapper := &pinPushStreamWrapper{
		ClientStreamingServer: stream,
		nodeID:                nodeID,
	}

	// 直接调用 Core 的 Push 方法
	return s.ns.Core().GetPin().Push(wrapper)
}

// value

func (s *PinService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetPin().GetValue(ctx, in)
}

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().GetValueByName(ctx,
		&cores.PinGetValueByNameRequest{NodeId: nodeID, Name: in.Name})
	if err != nil {
		return &output, err
	}

	output.Id = reply.Id
	output.Name = reply.Name
	output.Value = reply.Value
	output.Updated = reply.Updated

	return &output, nil
}

func (s *PinService) ViewValue(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewValue(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

// pinPushValueStreamWrapper 包装 node 端的流以添加验证并转发到 Core
type pinPushValueStreamWrapper struct {
	grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]
	nodeID string
	ns     *NodeService
}

func (w *pinPushValueStreamWrapper) Recv() (*pb.PinValue, error) {
	in, err := w.ClientStreamingServer.Recv()
	if err != nil {
		return nil, err
	}
	return in, nil
}

func (s *PinService) PushValue(stream grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]) error {
	ctx := stream.Context()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return err
	}

	// 创建包装器流
	wrapper := &pinPushValueStreamWrapper{
		ClientStreamingServer: stream,
		nodeID:                nodeID,
		ns:                    s.ns,
	}

	// 直接调用 Core 的 PushValue 方法
	return s.ns.Core().GetPin().PushValue(wrapper)
}

// write

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetPin().GetWrite(ctx, in)
}

func (s *PinService) SetWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.Id}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetPin().SetWrite(ctx, in)
}

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().GetWriteByName(ctx,
		&cores.PinGetValueByNameRequest{NodeId: nodeID, Name: in.Name})
	if err != nil {
		return &output, err
	}

	output.Id = reply.Id
	output.Name = reply.Name
	output.Value = reply.Value
	output.Updated = reply.Updated

	return &output, nil
}

func (s *PinService) SetWriteByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPin().SetWriteByName(ctx,
		&cores.PinNameValue{NodeId: nodeID, Name: in.Name, Value: in.Value})
}

func (s *PinService) ViewWrite(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewWrite(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *PinService) DeleteWrite(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewWrite(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetPin().DeleteWrite(ctx, in)
}

// pinPullWriteStreamWrapper 包装服务端流以转发 PullWrite 响应
type pinPullWriteStreamWrapper struct {
	grpc.ServerStreamingServer[pb.PinValueUpdated]
	nodeID string
}

func (w *pinPullWriteStreamWrapper) Send(item *pb.PinValueUpdated) error {
	return w.ServerStreamingServer.Send(item)
}

func (s *PinService) PullWrite(in *nodes.PinPullWriteRequest, stream grpc.ServerStreamingServer[pb.PinValueUpdated]) error {
	ctx := stream.Context()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return err
	}

	// 创建 Core 端的请求
	request := &cores.PinPullWriteRequest{
		After:  in.After,
		Limit:  in.Limit,
		NodeId: nodeID,
		WireId: in.WireId,
	}

	// 创建包装器流
	wrapper := &pinPullWriteStreamWrapper{
		ServerStreamingServer: stream,
		nodeID:                nodeID,
	}

	// 直接调用 Core 的 PullWrite 方法
	return s.ns.Core().GetPin().PullWrite(request, wrapper)
}

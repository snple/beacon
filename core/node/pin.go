package node

import (
	"context"
	"io"

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

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	return reply, nil
}

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
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

	return reply, nil
}

func (s *PinService) List(ctx context.Context, in *nodes.PinListRequest) (*nodes.PinListResponse, error) {
	var output nodes.PinListResponse

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinListRequest{
		NodeId: nodeID,
		WireId: in.WireId,
	}

	reply, err := s.ns.Core().GetPin().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Pins = reply.Pins

	return &output, nil
}

// value

func (s *PinService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	// 验证 Pin 属于当前节点
	pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
	_, err = s.ns.Core().GetPin().View(ctx, pinRequest)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPinValue().GetValue(ctx, in)
}

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinNameRequest{NodeId: nodeID, Name: in.Name}
	return s.ns.Core().GetPinValue().GetValueByName(ctx, request)
}

func (s *PinService) PushValue(stream grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]) error {
	ctx := stream.Context()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return err
	}

	var output pb.MyBool

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&output)
		}
		if err != nil {
			return err
		}

		// 验证 Pin 属于当前节点
		pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
		_, err = s.ns.Core().GetPin().View(ctx, pinRequest)
		if err != nil {
			return err
		}

		// 设置值
		_, err = s.ns.Core().GetPinValue().SetValue(ctx, in)
		if err != nil {
			return err
		}
	}
}

// write

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	// 验证 Pin 属于当前节点
	pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
	_, err = s.ns.Core().GetPin().View(ctx, pinRequest)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPinWrite().GetWrite(ctx, in)
}

func (s *PinService) SetWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	// 验证 Pin 属于当前节点
	pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
	_, err = s.ns.Core().GetPin().View(ctx, pinRequest)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPinWrite().SetWrite(ctx, in)
}

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinNameRequest{NodeId: nodeID, Name: in.Name}
	return s.ns.Core().GetPinWrite().GetWriteByName(ctx, request)
}

func (s *PinService) SetWriteByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinNameValueRequest{NodeId: nodeID, Name: in.Name, Value: in.Value}
	return s.ns.Core().GetPinWrite().SetWriteByName(ctx, request)
}

func (s *PinService) DeleteWrite(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	// 验证 Pin 属于当前节点
	pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: in.Id}
	_, err = s.ns.Core().GetPin().View(ctx, pinRequest)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPinWrite().DeleteWrite(ctx, in)
}

func (s *PinService) PullWrite(in *nodes.PinPullWriteRequest, stream grpc.ServerStreamingServer[pb.PinValue]) error {
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
	}

	// 创建包装器流
	wrapper := &pinPullWriteStreamWrapper{
		ServerStreamingServer: stream,
	}

	// 直接调用 Core 的 PullWrite 方法
	return s.ns.Core().GetPinWrite().PullWrite(request, wrapper)
}

// pinPullWriteStreamWrapper 包装服务端流以转发 PullWrite 响应
type pinPullWriteStreamWrapper struct {
	grpc.ServerStreamingServer[pb.PinValue]
}

func (w *pinPullWriteStreamWrapper) Send(item *pb.PinValue) error {
	return w.ServerStreamingServer.Send(item)
}

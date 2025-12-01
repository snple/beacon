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

type WireService struct {
	ns *NodeService

	nodes.UnimplementedWireServiceServer
}

func newWireService(ns *NodeService) *WireService {
	return &WireService{
		ns: ns,
	}
}

func (s *WireService) View(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
	var output pb.Wire
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

	reply, err := s.ns.Core().GetWire().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *WireService) Name(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire
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

	request := &cores.WireNameRequest{NodeId: nodeID, Name: in.Name}

	reply, err := s.ns.Core().GetWire().Name(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *WireService) List(ctx context.Context, in *nodes.WireListRequest) (*nodes.WireListResponse, error) {
	var err error
	var output nodes.WireListResponse

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

	request := &cores.WireListRequest{
		Page:   in.GetPage(),
		NodeId: nodeID,
	}

	reply, err := s.ns.Core().GetWire().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Count = reply.Count
	output.Page = reply.Page
	output.Wires = reply.Wires

	return &output, nil
}

// wirePushStreamWrapper 包装 node 端的流以添加 NodeID 并转发到 Core
type wirePushStreamWrapper struct {
	grpc.ClientStreamingServer[pb.Wire, pb.MyBool]
	nodeID string
}

func (w *wirePushStreamWrapper) Recv() (*pb.Wire, error) {
	in, err := w.ClientStreamingServer.Recv()
	if err != nil {
		return nil, err
	}
	in.NodeId = w.nodeID
	return in, nil
}

func (s *WireService) Push(stream grpc.ClientStreamingServer[pb.Wire, pb.MyBool]) error {
	ctx := stream.Context()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return err
	}

	// 创建包装器流，自动添加 NodeID
	wrapper := &wirePushStreamWrapper{
		ClientStreamingServer: stream,
		nodeID:                nodeID,
	}

	// 直接调用 Core 的 Push 方法
	return s.ns.Core().GetWire().Push(wrapper)
}

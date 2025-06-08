package node

import (
	"context"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/pb/nodes"
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

func (s *WireService) Create(ctx context.Context, in *pb.Wire) (*pb.Wire, error) {
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

	in.NodeId = nodeID

	return s.ns.Core().GetWire().Create(ctx, in)
}

func (s *WireService) Update(ctx context.Context, in *pb.Wire) (*pb.Wire, error) {
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

	request := &pb.Id{Id: in.Id}

	reply, err := s.ns.Core().GetWire().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetWire().Update(ctx, in)
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

func (s *WireService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
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

	reply, err := s.ns.Core().GetWire().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return s.ns.Core().GetWire().Delete(ctx, in)
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
		Tags:   in.Tags,
		Source: in.Source,
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

func (s *WireService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
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

	reply, err := s.ns.Core().GetWire().ViewWithDeleted(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.NodeId != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.NodeId != nodeID")
	}

	return reply, nil
}

func (s *WireService) Pull(ctx context.Context, in *nodes.WirePullRequest) (*nodes.WirePullResponse, error) {
	var err error
	var output nodes.WirePullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.After
	output.Limit = in.Limit

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.WirePullRequest{
		After:  in.After,
		Limit:  in.Limit,
		NodeId: nodeID,
		Source: in.Source,
	}

	reply, err := s.ns.Core().GetWire().Pull(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Wires = reply.Wires

	return &output, nil
}

func (s *WireService) Sync(ctx context.Context, in *pb.Wire) (*pb.MyBool, error) {
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

	in.NodeId = nodeID

	return s.ns.Core().GetWire().Sync(ctx, in)
}

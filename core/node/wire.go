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

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetWire().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
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

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
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

	request := &cores.WireNameRequest{NodeId: nodeID, Name: in.GetName()}

	reply, err := s.ns.Core().GetWire().Name(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
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

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
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
		Tags:   in.GetTags(),
		Source: in.GetSource(),
	}

	reply, err := s.ns.Core().GetWire().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Count = reply.Count
	output.Page = reply.GetPage()
	output.Wire = reply.GetWire()

	return &output, nil
}

func (s *WireService) Link(ctx context.Context, in *nodes.WireLinkRequest) (*pb.MyBool, error) {
	var output pb.MyBool
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

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetWire().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	request2 := &cores.WireLinkRequest{Id: in.GetId(), Status: in.GetStatus()}

	return s.ns.Core().GetWire().Link(ctx, request2)
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

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
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

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.WirePullRequest{
		After:  in.GetAfter(),
		Limit:  in.GetLimit(),
		NodeId: nodeID,
		Source: in.GetSource(),
	}

	reply, err := s.ns.Core().GetWire().Pull(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Wire = reply.GetWire()

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

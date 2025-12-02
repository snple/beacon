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

func (s *WireService) View(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.WireViewRequest{NodeId: nodeID, WireId: in.Id}
	reply, err := s.ns.Core().GetWire().View(ctx, request)
	if err != nil {
		return &output, err
	}

	return reply, nil
}

func (s *WireService) Name(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
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

	return reply, nil
}

func (s *WireService) List(ctx context.Context, in *nodes.WireListRequest) (*nodes.WireListResponse, error) {
	var output nodes.WireListResponse

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.WireListRequest{
		NodeId: nodeID,
	}

	reply, err := s.ns.Core().GetWire().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Wires = reply.Wires

	return &output, nil
}

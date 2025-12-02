package core

import (
	"context"
	"strings"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WireService struct {
	cs *CoreService

	cores.UnimplementedWireServiceServer
}

func newWireService(cs *CoreService) *WireService {
	return &WireService{
		cs: cs,
	}
}

func (s *WireService) View(ctx context.Context, in *cores.WireViewRequest) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	if in == nil || in.NodeId == "" || in.WireId == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and WireId")
	}

	wire, err := s.cs.GetStorage().GetWireByID(in.WireId)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) Name(ctx context.Context, in *cores.WireNameRequest) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and Name")
	}

	wire, err := s.cs.GetStorage().GetWireByName(in.NodeId, in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) NameFull(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	if in == nil || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Name")
	}

	nodeName := consts.DEFAULT_NODE
	wireName := in.Name

	if strings.Contains(wireName, ".") {
		parts := strings.Split(wireName, ".")
		if len(parts) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Invalid wire full name format, expected NodeName.WireName")
		}
		nodeName = parts[0]
		wireName = parts[1]
	}

	node, err := s.cs.GetStorage().GetNodeByName(nodeName)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Node not found: %v", err)
	}

	wire, err := s.cs.GetStorage().GetWireByName(node.ID, wireName)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) List(ctx context.Context, in *cores.WireListRequest) (*cores.WireListResponse, error) {
	var output cores.WireListResponse

	// basic validation
	if in == nil || in.NodeId == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
	}

	wires, err := s.cs.GetStorage().ListWires(in.NodeId)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "List wires failed: %v", err)
	}

	for _, wire := range wires {
		item := pb.Wire{}
		s.copyStorageToOutput(&item, wire)
		output.Wires = append(output.Wires, &item)
	}

	return &output, nil
}

func (s *WireService) copyStorageToOutput(output *pb.Wire, wire *storage.Wire) {
	output.Id = wire.ID
	output.Name = wire.Name
	output.Type = wire.Type
	output.Tags = wire.Tags
	output.Clusters = wire.Clusters

	for i := range wire.Pins {
		pin := &wire.Pins[i]
		pbPin := &pb.Pin{
			Id:   pin.ID,
			Name: pin.Name,
			Addr: pin.Addr,
			Type: pin.Type,
			Rw:   pin.Rw,
			Tags: pin.Tags,
		}
		output.Pins = append(output.Pins, pbPin)
	}
}

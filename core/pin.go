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

type PinService struct {
	cs *CoreService

	cores.UnimplementedPinServiceServer
}

func newPinService(cs *CoreService) *PinService {
	return &PinService{
		cs: cs,
	}
}

func (s *PinService) View(ctx context.Context, in *cores.PinViewRequest) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	if in == nil || in.NodeId == "" || in.PinId == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and PinId")
	}

	pin, err := s.cs.GetStorage().GetPinByID(in.PinId)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, in *cores.PinNameRequest) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) NameFull(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	if in == nil || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Name")
	}

	nodeName := consts.DEFAULT_NODE
	wireName := consts.DEFAULT_WIRE
	pinName := in.Name

	if strings.Contains(pinName, ".") {
		parts := strings.Split(pinName, ".")
		switch len(parts) {
		case 2:
			wireName = parts[0]
			pinName = parts[1]
		case 3:
			nodeName = parts[0]
			wireName = parts[1]
			pinName = parts[2]
		default:
			return &output, status.Error(codes.InvalidArgument, "Invalid pin full name format")
		}
	}

	node, err := s.cs.GetStorage().GetNodeByName(nodeName)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Node not found: %v", err)
	}

	// 使用 WireName.PinName 作为 pinName 查询
	pin, err := s.cs.GetStorage().GetPinByName(node.ID, wireName+"."+pinName)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) List(ctx context.Context, in *cores.PinListRequest) (*cores.PinListResponse, error) {
	var output cores.PinListResponse

	// basic validation
	if in == nil || in.NodeId == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
	}

	var pins []*storage.Pin
	var err error

	if in.WireId != "" {
		pins, err = s.cs.GetStorage().ListPins(in.WireId)
	} else {
		pins, err = s.cs.GetStorage().ListPinsByNode(in.NodeId)
	}

	if err != nil {
		return &output, status.Errorf(codes.Internal, "List pins failed: %v", err)
	}

	for _, pin := range pins {
		item := pb.Pin{}
		s.copyStorageToOutput(&item, pin)
		output.Pins = append(output.Pins, &item)
	}

	return &output, nil
}

func (s *PinService) copyStorageToOutput(output *pb.Pin, pin *storage.Pin) {
	output.Id = pin.ID
	output.Name = pin.Name
	output.Addr = pin.Addr
	output.Type = pin.Type
	output.Rw = pin.Rw
	output.Tags = pin.Tags
}

package edge

import (
	"context"

	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WireService struct {
	es *EdgeService

	edges.UnimplementedWireServiceServer
}

func newWireService(es *EdgeService) *WireService {
	return &WireService{
		es: es,
	}
}

func (s *WireService) View(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
		}
	}

	wire, err := s.es.GetStorage().GetWireByID(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	s.copyModelToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) Name(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	wire, err := s.es.GetStorage().GetWireByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	s.copyModelToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) List(ctx context.Context, in *pb.MyEmpty) (*edges.WireListResponse, error) {
	var output edges.WireListResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	wires := s.es.GetStorage().ListWires()

	for _, wire := range wires {
		item := pb.Wire{}
		s.copyModelToOutput(&item, wire)
		output.Wires = append(output.Wires, &item)
	}

	return &output, nil
}

func (s *WireService) ViewByID(ctx context.Context, id string) (*storage.Wire, error) {
	wire, err := s.es.GetStorage().GetWireByID(id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	return wire, nil
}

func (s *WireService) ViewByName(ctx context.Context, name string) (*storage.Wire, error) {
	wire, err := s.es.GetStorage().GetWireByName(name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Wire not found: %v", err)
	}

	return wire, nil
}

func (s *WireService) copyModelToOutput(output *pb.Wire, wire *storage.Wire) {
	output.Id = wire.ID
	output.Name = wire.Name
	output.Type = wire.Type
	output.Tags = wire.Tags

	// 复制 Pins
	for i := range wire.Pins {
		pin := &wire.Pins[i]
		output.Pins = append(output.Pins, &pb.Pin{
			Id:   pin.ID,
			Name: pin.Name,
			Addr: pin.Addr,
			Type: pin.Type,
			Rw:   pin.Rw,
			Tags: pin.Tags,
		})
	}
}

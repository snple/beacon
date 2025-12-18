package edge

import (
	"context"
	"errors"

	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/pb"
)

type WireService struct {
	es *EdgeService
}

func newWireService(es *EdgeService) *WireService {
	return &WireService{
		es: es,
	}
}

func (s *WireService) View(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
	var output pb.Wire

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Wire.ID")
	}

	wire, err := s.es.GetStorage().GetWireByID(in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) Name(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Wire.Name")
	}

	wire, err := s.es.GetStorage().GetWireByName(in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) List(ctx context.Context) ([]*pb.Wire, error) {
	wires := s.es.GetStorage().ListWires()

	var output []*pb.Wire
	for _, wire := range wires {
		item := &pb.Wire{}
		s.copyModelToOutput(item, wire)
		output = append(output, item)
	}

	return output, nil
}

func (s *WireService) ViewByID(ctx context.Context, id string) (*storage.Wire, error) {
	return s.es.GetStorage().GetWireByID(id)
}

func (s *WireService) ViewByName(ctx context.Context, name string) (*storage.Wire, error) {
	return s.es.GetStorage().GetWireByName(name)
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

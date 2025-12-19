package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
)

type WireService struct {
	cs *CoreService
}

func newWireService(cs *CoreService) *WireService {
	return &WireService{
		cs: cs,
	}
}

func (s *WireService) View(ctx context.Context, in *WireViewRequest) (*Wire, error) {
	var output Wire

	// basic validation
	if in == nil || in.NodeId == "" || in.WireId == "" {
		return &output, fmt.Errorf("please supply valid NodeId and WireId")
	}

	wire, err := s.cs.GetStorage().GetWireByID(in.WireId)
	if err != nil {
		return &output, fmt.Errorf("wire not found: %w", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) Name(ctx context.Context, in *WireNameRequest) (*Wire, error) {
	var output Wire

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, fmt.Errorf("please supply valid NodeId and Name")
	}

	wire, err := s.cs.GetStorage().GetWireByName(in.NodeId, in.Name)
	if err != nil {
		return &output, fmt.Errorf("wire not found: %w", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) NameFull(ctx context.Context, in *Name) (*Wire, error) {
	var output Wire

	// basic validation
	if in == nil || in.Name == "" {
		return &output, fmt.Errorf("please supply valid Name")
	}

	nodeName := device.DEFAULT_NODE
	wireName := in.Name

	if strings.Contains(wireName, ".") {
		parts := strings.Split(wireName, ".")
		if len(parts) != 2 {
			return &output, fmt.Errorf("invalid wire full name format, expected NodeName.WireName")
		}
		nodeName = parts[0]
		wireName = parts[1]
	}

	node, err := s.cs.GetStorage().GetNodeByName(nodeName)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	wire, err := s.cs.GetStorage().GetWireByName(node.ID, wireName)
	if err != nil {
		return &output, fmt.Errorf("wire not found: %w", err)
	}

	s.copyStorageToOutput(&output, wire)

	return &output, nil
}

func (s *WireService) List(ctx context.Context, in *WireListRequest) (*WireListResponse, error) {
	var output WireListResponse

	// basic validation
	if in == nil || in.NodeId == "" {
		return &output, fmt.Errorf("please supply valid NodeId")
	}

	wires, err := s.cs.GetStorage().ListWires(in.NodeId)
	if err != nil {
		return &output, fmt.Errorf("list wires failed: %w", err)
	}

	for _, wire := range wires {
		item := Wire{}
		s.copyStorageToOutput(&item, wire)
		output.Wires = append(output.Wires, &item)
	}

	return &output, nil
}

func (s *WireService) copyStorageToOutput(output *Wire, wire *dt.Wire) {
	output.Id = wire.ID
	output.Name = wire.Name
	output.Type = wire.Type
	output.Tags = wire.Tags

	for i := range wire.Pins {
		pin := &wire.Pins[i]
		p := &Pin{
			Id:   pin.ID,
			Name: pin.Name,
			Addr: pin.Addr,
			Type: pin.Type,
			Rw:   pin.Rw,
			Tags: pin.Tags,
		}
		output.Pins = append(output.Pins, p)
	}
}

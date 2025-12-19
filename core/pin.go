package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
)

type PinService struct {
	cs *CoreService
}

func newPinService(cs *CoreService) *PinService {
	return &PinService{
		cs: cs,
	}
}

func (s *PinService) View(ctx context.Context, in *PinViewRequest) (*Pin, error) {
	var output Pin

	// basic validation
	if in == nil || in.NodeId == "" || in.PinId == "" {
		return &output, fmt.Errorf("please supply valid NodeId and PinId")
	}

	pin, err := s.cs.GetStorage().GetPinByID(in.PinId)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, in *PinNameRequest) (*Pin, error) {
	var output Pin

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) NameFull(ctx context.Context, in *Name) (*Pin, error) {
	var output Pin

	// basic validation
	if in == nil || in.Name == "" {
		return &output, fmt.Errorf("please supply valid Name")
	}

	nodeName := device.DEFAULT_NODE
	wireName := device.DEFAULT_WIRE
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
			return &output, fmt.Errorf("invalid pin full name format")
		}
	}

	node, err := s.cs.GetStorage().GetNodeByName(nodeName)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	// 使用 WireName.PinName 作为 pinName 查询
	pin, err := s.cs.GetStorage().GetPinByName(node.ID, wireName+"."+pinName)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) List(ctx context.Context, in *PinListRequest) (*PinListResponse, error) {
	var output PinListResponse

	// basic validation
	if in == nil || in.NodeId == "" {
		return &output, fmt.Errorf("please supply valid NodeId")
	}

	var pins []*dt.Pin
	var err error

	if in.WireId != "" {
		pins, err = s.cs.GetStorage().ListPins(in.WireId)
	} else {
		pins, err = s.cs.GetStorage().ListPinsByNode(in.NodeId)
	}

	if err != nil {
		return &output, fmt.Errorf("list pins failed: %w", err)
	}

	for _, pin := range pins {
		item := Pin{}
		s.copyStorageToOutput(&item, pin)
		output.Pins = append(output.Pins, &item)
	}

	return &output, nil
}

func (s *PinService) copyStorageToOutput(output *Pin, pin *dt.Pin) {
	output.Id = pin.ID
	output.Name = pin.Name
	output.Addr = pin.Addr
	output.Type = pin.Type
	output.Rw = pin.Rw
	output.Tags = pin.Tags
}

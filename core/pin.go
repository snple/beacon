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

func (s *PinService) View(ctx context.Context, nodeID, pinID string) (*Pin, error) {
	var output Pin

	// basic validation
	if nodeID == "" || pinID == "" {
		return &output, fmt.Errorf("please supply valid NodeId and PinId")
	}

	pin, err := s.cs.GetStorage().GetPinByID(pinID)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, nodeID, name string) (*Pin, error) {
	var output Pin

	// basic validation
	if nodeID == "" || name == "" {
		return &output, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(nodeID, name)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	s.copyStorageToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) NameFull(ctx context.Context, fullName string) (*Pin, error) {
	var output Pin

	// basic validation
	if fullName == "" {
		return &output, fmt.Errorf("please supply valid Name")
	}

	nodeName := device.DEFAULT_NODE
	wireName := device.DEFAULT_WIRE
	pinName := fullName

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

func (s *PinService) List(ctx context.Context, nodeID, wireID string) ([]*Pin, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid NodeId")
	}

	var pins []*dt.Pin
	var err error

	if wireID != "" {
		pins, err = s.cs.GetStorage().ListPins(wireID)
	} else {
		pins, err = s.cs.GetStorage().ListPinsByNode(nodeID)
	}

	if err != nil {
		return nil, fmt.Errorf("list pins failed: %w", err)
	}

	result := make([]*Pin, 0, len(pins))
	for _, pin := range pins {
		item := &Pin{}
		s.copyStorageToOutput(item, pin)
		result = append(result, item)
	}

	return result, nil
}

func (s *PinService) copyStorageToOutput(output *Pin, pin *dt.Pin) {
	output.Id = pin.ID
	output.Name = pin.Name
	output.Addr = pin.Addr
	output.Type = pin.Type
	output.Rw = pin.Rw
	output.Tags = pin.Tags
}

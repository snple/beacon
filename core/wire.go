package core

import (
	"context"
	"fmt"

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

func (s *WireService) View(ctx context.Context, nodeID, wireID string) (*dt.Wire, error) {
	// basic validation
	if nodeID == "" || wireID == "" {
		return nil, fmt.Errorf("please supply valid NodeId and WireId")
	}

	wire, err := s.cs.GetStorage().GetWireByID(wireID)
	if err != nil {
		return nil, fmt.Errorf("wire not found: %w", err)
	}

	return wire, nil
}

func (s *WireService) List(ctx context.Context, nodeID string) ([]dt.Wire, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid NodeId")
	}

	wires, err := s.cs.GetStorage().ListWires(nodeID)
	if err != nil {
		return nil, fmt.Errorf("list wires failed: %w", err)
	}

	return wires, nil
}

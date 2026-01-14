package core

import (
	"fmt"

	"github.com/snple/beacon/dt"
)

// Wire operations

func (cs *CoreService) ViewWire(nodeID, wireID string) (*dt.Wire, error) {
	// basic validation
	if nodeID == "" || wireID == "" {
		return nil, fmt.Errorf("please supply valid NodeId and WireId")
	}

	wire, err := cs.GetStorage().GetWireByID(wireID)
	if err != nil {
		return nil, fmt.Errorf("wire not found: %w", err)
	}

	return wire, nil
}

func (cs *CoreService) ListWires(nodeID string) ([]dt.Wire, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid NodeId")
	}

	wires, err := cs.GetStorage().ListWires(nodeID)
	if err != nil {
		return nil, fmt.Errorf("list wires failed: %w", err)
	}

	return wires, nil
}

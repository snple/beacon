package core

import (
	"fmt"

	"github.com/snple/beacon/dt"
)

// Pin operations

func (cs *CoreService) ViewPin(pinID string) (*dt.Pin, error) {
	// basic validation
	if pinID == "" {
		return nil, fmt.Errorf("please supply valid PinId")
	}

	pin, err := cs.GetStorage().GetPinByID(pinID)
	if err != nil {
		return nil, fmt.Errorf("pin not found: %w", err)
	}

	return pin, nil
}

func (cs *CoreService) ListPins(nodeID, wireID string) ([]dt.Pin, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid NodeId")
	}

	var pins []dt.Pin
	var err error

	if wireID != "" {
		pins, err = cs.GetStorage().ListPins(wireID)
	} else {
		pins, err = cs.GetStorage().ListPinsByNode(nodeID)
	}

	if err != nil {
		return nil, fmt.Errorf("list pins failed: %w", err)
	}

	return pins, nil
}

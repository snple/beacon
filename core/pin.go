package core

import (
	"context"
	"fmt"

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

func (s *PinService) View(ctx context.Context, nodeID, pinID string) (*dt.Pin, error) {
	// basic validation
	if nodeID == "" || pinID == "" {
		return nil, fmt.Errorf("please supply valid NodeId and PinId")
	}

	pin, err := s.cs.GetStorage().GetPinByID(pinID)
	if err != nil {
		return nil, fmt.Errorf("pin not found: %w", err)
	}

	return pin, nil
}

func (s *PinService) List(ctx context.Context, nodeID, wireID string) ([]dt.Pin, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid NodeId")
	}

	var pins []dt.Pin
	var err error

	if wireID != "" {
		pins, err = s.cs.GetStorage().ListPins(wireID)
	} else {
		pins, err = s.cs.GetStorage().ListPinsByNode(nodeID)
	}

	if err != nil {
		return nil, fmt.Errorf("list pins failed: %w", err)
	}

	return pins, nil
}

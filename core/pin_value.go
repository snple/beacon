package core

import (
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
)

type PinValueService struct {
	cs *CoreService
}

func newPinValueService(cs *CoreService) *PinValueService {
	return &PinValueService{
		cs: cs,
	}
}

func (s *PinValueService) GetValue(ctx context.Context, pinID string) (nson.Value, time.Time, error) {
	// basic validation
	if pinID == "" {
		return nil, time.Time{}, fmt.Errorf("please supply valid Pin.Id")
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(pinID)
	if err != nil {
		return nil, time.Time{}, nil
	}

	value, updated, err := s.cs.GetStorage().GetPinValue(nodeID, pinID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (s *PinValueService) SetValue(ctx context.Context, pinID string, value nson.Value, updated time.Time) error {
	// basic validation
	if pinID == "" {
		return fmt.Errorf("please supply valid Pin.Id")
	}

	// 验证 Pin 存在并获取 nodeID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(pinID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, nodeID, pinID, value, updated)
	if err != nil {
		return fmt.Errorf("setPinValue failed: %w", err)
	}

	// TODO: 通知同步服务

	return nil
}

func (s *PinValueService) GetValueByName(ctx context.Context, nodeID, name string) (nson.Value, time.Time, error) {
	// basic validation
	if nodeID == "" || name == "" {
		return nil, time.Time{}, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(nodeID, name)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("pin not found: %w", err)
	}

	value, updated, err := s.cs.GetStorage().GetPinValue(nodeID, pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (s *PinValueService) SetValueByName(ctx context.Context, nodeID, name string, value nson.Value) error {
	// basic validation
	if nodeID == "" || name == "" {
		return fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(nodeID, name)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, nodeID, pin.ID, value, time.Now())
	if err != nil {
		return fmt.Errorf("setPinValue failed: %w", err)
	}

	// TODO: 通知同步服务

	return nil
}

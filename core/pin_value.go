package core

import (
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
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

func (s *PinValueService) setValue(ctx context.Context, value dt.PinValue) error {
	// basic validation
	if value.ID == "" || value.Value == nil {
		return fmt.Errorf("please supply valid Pin.Id and Value")
	}

	if value.Updated.IsZero() {
		value.Updated = time.Now()
	}

	// 验证 Pin 存在并获取 nodeID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(value.ID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, nodeID, value)
	if err != nil {
		return fmt.Errorf("setPinValue failed: %w", err)
	}

	// TODO: 通知同步服务

	return nil
}

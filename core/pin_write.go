package core

import (
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
)

type PinWriteService struct {
	cs *CoreService
}

func newPinWriteService(cs *CoreService) *PinWriteService {
	return &PinWriteService{
		cs: cs,
	}
}

func (s *PinWriteService) GetWrite(ctx context.Context, pinID string) (nson.Value, time.Time, error) {
	// basic validation
	if pinID == "" {
		return nil, time.Time{}, fmt.Errorf("please supply valid Pin.Id")
	}

	value, updated, err := s.cs.GetStorage().GetPinWrite(pinID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (s *PinWriteService) SetWrite(ctx context.Context, value dt.PinValue, realtime bool) error {
	// basic validation
	if value.ID == "" || value.Value == nil {
		return fmt.Errorf("please supply valid Pin.Id and Value")
	}

	if value.Updated.IsZero() {
		value.Updated = time.Now()
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(value.ID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByID(value.ID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	if uint8(value.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, value)
	if err != nil {
		return fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := s.cs.NotifyPinWrite(nodeID, pin.ID, value.Value, realtime); err != nil {
		s.cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	return nil
}

func (s *PinWriteService) DeleteWrite(ctx context.Context, pinID string) error {
	// basic validation
	if pinID == "" {
		return fmt.Errorf("please supply valid Pin.Id")
	}

	err := s.cs.GetStorage().DeletePinWrite(ctx, pinID)
	if err != nil {
		return fmt.Errorf("deletePinWrite failed: %w", err)
	}

	return nil
}

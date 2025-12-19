package core

import (
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
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

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(pinID)
	if err != nil {
		return nil, time.Time{}, nil
	}

	value, updated, err := s.cs.GetStorage().GetPinWrite(nodeID, pinID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (s *PinWriteService) SetWrite(ctx context.Context, pinID string, value nson.Value, realtime bool) error {
	// basic validation
	if pinID == "" || value == nil {
		return fmt.Errorf("please supply valid Pin.Id and Value")
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(pinID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByID(pinID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, nodeID, pinID, value, time.Now())
	if err != nil {
		return fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := s.cs.NotifyPinWrite(nodeID, pinID, pin.Name, value, realtime); err != nil {
		s.cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	return nil
}

func (s *PinWriteService) GetWriteByName(ctx context.Context, nodeID, name string) (nson.Value, time.Time, error) {
	// basic validation
	if nodeID == "" || name == "" {
		return nil, time.Time{}, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(nodeID, name)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("pin not found: %w", err)
	}

	value, updated, err := s.cs.GetStorage().GetPinWrite(nodeID, pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (s *PinWriteService) SetWriteByName(ctx context.Context, nodeID, name string, value nson.Value, realtime bool) error {
	// basic validation
	if nodeID == "" || name == "" || value == nil {
		return fmt.Errorf("please supply valid NodeId, Name and Value")
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByName(nodeID, name)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, nodeID, pin.ID, value, time.Now())
	if err != nil {
		return fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := s.cs.NotifyPinWrite(nodeID, pin.ID, pin.Name, value, realtime); err != nil {
		s.cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	return nil
}

func (s *PinWriteService) DeleteWrite(ctx context.Context, pinID string) error {
	// basic validation
	if pinID == "" {
		return fmt.Errorf("please supply valid Pin.Id")
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(pinID)
	if err != nil {
		// Pin 不存在，返回成功
		return nil
	}

	err = s.cs.GetStorage().DeletePinWrite(ctx, nodeID, pinID)
	if err != nil {
		return fmt.Errorf("deletePinWrite failed: %w", err)
	}

	return nil
}

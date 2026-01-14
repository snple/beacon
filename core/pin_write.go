package core

import (
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
)

// PinWrite operations

func (cs *CoreService) GetPinWrite(pinID string) (nson.Value, time.Time, error) {
	// basic validation
	if pinID == "" {
		return nil, time.Time{}, fmt.Errorf("please supply valid Pin.Id")
	}

	value, updated, err := cs.GetStorage().GetPinWrite(pinID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return nil, time.Time{}, nil
	}

	return value, updated, nil
}

func (cs *CoreService) SetPinWrite(value dt.PinValue, realtime bool) error {
	// basic validation
	if value.ID == "" || value.Value == nil {
		return fmt.Errorf("please supply valid Pin.Id and Value")
	}

	if value.Updated.IsZero() {
		value.Updated = time.Now()
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := cs.GetStorage().GetPinNodeID(value.ID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	// 验证 Pin 存在且可写
	pin, err := cs.GetStorage().GetPinByID(value.ID)
	if err != nil {
		return fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return fmt.Errorf("pin is not writable")
	}

	if uint8(value.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	err = cs.GetStorage().SetPinWrite(value)
	if err != nil {
		return fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := cs.NotifyPinWrite(nodeID, pin.ID, value.Value, realtime); err != nil {
		cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	return nil
}

func (cs *CoreService) DeletePinWrite(pinID string) error {
	// basic validation
	if pinID == "" {
		return fmt.Errorf("please supply valid Pin.Id")
	}

	err := cs.GetStorage().DeletePinWrite(pinID)
	if err != nil {
		return fmt.Errorf("deletePinWrite failed: %w", err)
	}

	return nil
}

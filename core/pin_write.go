package core

import (
	"context"
	"fmt"
	"time"

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

func (s *PinWriteService) GetWrite(ctx context.Context, in *Id) (*PinValue, error) {
	var output PinValue

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Pin.Id")
	}

	output.Id = in.Id

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(in.Id)
	if err != nil {
		return &output, nil
	}

	value, updated, err := s.cs.GetStorage().GetPinWrite(nodeID, in.Id)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinWriteService) SetWrite(ctx context.Context, in *PinValue) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" || in.Value == nil {
		return &output, fmt.Errorf("please supply valid Pin.Id and Value")
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(in.Id)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return &output, fmt.Errorf("pin is not writable")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, nodeID, in.Id, in.Value, time.Now())
	if err != nil {
		return &output, fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := s.cs.NotifyPinWrite(nodeID, in.Id, pin.Name, in.Value); err != nil {
		s.cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *PinWriteService) GetWriteByName(ctx context.Context, in *PinNameRequest) (*PinNameValue, error) {
	var output PinNameValue

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	output.Name = in.Name

	value, updated, err := s.cs.GetStorage().GetPinWrite(in.NodeId, pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinWriteService) SetWriteByName(ctx context.Context, in *PinNameValueRequest) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" || in.Value == nil {
		return &output, fmt.Errorf("please supply valid NodeId, Name and Value")
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	if pin.Rw == device.RO {
		return &output, fmt.Errorf("pin is not writable")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, in.NodeId, pin.ID, in.Value, time.Now())
	if err != nil {
		return &output, fmt.Errorf("setPinWrite failed: %w", err)
	}

	// 通知节点有新的 Pin 写入
	if err := s.cs.NotifyPinWrite(in.NodeId, pin.ID, pin.Name, in.Value); err != nil {
		s.cs.Logger().Sugar().Warnf("NotifyPinWrite failed: %v", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *PinWriteService) DeleteWrite(ctx context.Context, in *Id) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Pin.Id")
	}

	// 获取 Pin 所属的 Node ID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(in.Id)
	if err != nil {
		// Pin 不存在，返回成功
		output.Bool = true
		return &output, nil
	}

	err = s.cs.GetStorage().DeletePinWrite(ctx, nodeID, in.Id)
	if err != nil {
		return &output, fmt.Errorf("deletePinWrite failed: %w", err)
	}

	output.Bool = true
	return &output, nil
}

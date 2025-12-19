package core

import (
	"context"
	"fmt"
	"time"
)

type PinValueService struct {
	cs *CoreService
}

func newPinValueService(cs *CoreService) *PinValueService {
	return &PinValueService{
		cs: cs,
	}
}

func (s *PinValueService) GetValue(ctx context.Context, in *Id) (*PinValue, error) {
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

	value, updated, err := s.cs.GetStorage().GetPinValue(nodeID, in.Id)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinValueService) SetValue(ctx context.Context, in *PinValue) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Pin.Id")
	}

	// 验证 Pin 存在并获取 nodeID
	nodeID, err := s.cs.GetStorage().GetPinNodeID(in.Id)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, nodeID, in.Id, in.Value, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, fmt.Errorf("setPinValue failed: %w", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *PinValueService) GetValueByName(ctx context.Context, in *PinNameRequest) (*PinNameValue, error) {
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

	value, updated, err := s.cs.GetStorage().GetPinValue(in.NodeId, pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinValueService) SetValueByName(ctx context.Context, in *PinNameValueRequest) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, fmt.Errorf("please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, fmt.Errorf("pin not found: %w", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, in.NodeId, pin.ID, in.Value, time.Now())
	if err != nil {
		return &output, fmt.Errorf("setPinValue failed: %w", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

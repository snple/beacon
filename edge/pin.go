package edge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/pb"
)

type PinService struct {
	es *EdgeService
}

func newPinService(es *EdgeService) *PinService {
	return &PinService{
		es: es,
	}
}

// --- PinService ---

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Pin.Id")
	}

	pin, err := s.es.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Pin.Name")
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, pin)
	output.Name = in.Name

	return &output, nil
}

func (s *PinService) List(ctx context.Context, wireId string) ([]*pb.Pin, error) {
	var pins []*storage.Pin

	if wireId != "" {
		// 按 Wire 过滤
		wirePins, err := s.es.GetStorage().ListPinsByWire(wireId)
		if err != nil {
			return nil, err
		}
		pins = wirePins
	} else {
		pins = s.es.GetStorage().ListPins()
	}

	var output []*pb.Pin
	for _, pin := range pins {
		item := &pb.Pin{}
		s.copyModelToOutput(item, pin)
		output = append(output, item)
	}

	return output, nil
}

func (s *PinService) ViewByID(ctx context.Context, id string) (*storage.Pin, error) {
	return s.es.GetStorage().GetPinByID(id)
}

func (s *PinService) ViewByName(ctx context.Context, name string) (*storage.Pin, error) {
	return s.es.GetStorage().GetPinByName(name)
}

func (s *PinService) copyModelToOutput(output *pb.Pin, pin *storage.Pin) {
	output.Id = pin.ID
	output.Name = pin.Name
	output.Addr = pin.Addr
	output.Type = pin.Type
	output.Rw = pin.Rw
	output.Tags = pin.Tags
}

// --- PinValueService ---

func (s *PinService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Pin.Id")
	}

	value, updated, err := s.es.GetStorage().GetPinValue(in.Id)
	if err != nil {
		return &output, err
	}

	output.Id = in.Id
	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var output pb.MyBool

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Pin.Id")
	}

	if in.Value == nil {
		return &output, errors.New("please supply valid Pin.Value")
	}

	// 验证 Pin 存在
	pin, err := s.es.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, err
	}

	// 验证值类型
	if !dt.ValidateNsonValue(in.Value, pin.Type) {
		return &output, errors.New("invalid value for Pin.Type")
	}

	updated := time.Now()
	if in.Updated > 0 {
		updated = time.UnixMicro(in.Updated)
	}

	err = s.es.GetStorage().SetPinValue(ctx, in.Id, in.Value, updated)
	if err != nil {
		return &output, err
	}

	// 更新同步时间戳
	if err = s.es.GetSync().setPinValueUpdated(ctx, updated); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Pin.Name")
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, err
	}

	output.Name = in.Name

	value, updated, err := s.es.GetStorage().GetPinValue(pin.ID)
	if err != nil {
		// 没有值也返回成功
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetValueByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var output pb.MyBool

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Pin.Name")
	}

	if in.Value == nil {
		return &output, errors.New("please supply valid Pin.Value")
	}

	// 解析名称获取 Wire 和 Pin
	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, err
	}

	// 验证值类型
	if !dt.ValidateNsonValue(in.Value, pin.Type) {
		return &output, errors.New("invalid value for Pin.Type")
	}

	updated := time.Now()
	if in.Updated > 0 {
		updated = time.UnixMicro(in.Updated)
	}

	err = s.es.GetStorage().SetPinValue(ctx, pin.ID, in.Value, updated)
	if err != nil {
		return &output, err
	}

	// 更新同步时间戳
	if err = s.es.GetSync().setPinValueUpdated(ctx, updated); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) DeleteValue(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Pin.Id")
	}

	err := s.es.GetStorage().DeletePinValue(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) ListValues(ctx context.Context, after time.Time, limit int) ([]*pb.PinValue, error) {
	if limit <= 0 {
		limit = 100
	}

	entries, err := s.es.GetStorage().ListPinValues(after, limit)
	if err != nil {
		return nil, err
	}

	var output []*pb.PinValue
	for _, entry := range entries {
		// 使用 dt.DecodeNsonValue 反序列化
		var value *pb.NsonValue
		if len(entry.Value) > 0 {
			value, err = dt.DecodeNsonValue(entry.Value)
			if err != nil {
				return nil, err
			}
		}

		output = append(output, &pb.PinValue{
			Id:      entry.ID,
			Value:   value,
			Updated: entry.Updated.UnixMicro(),
		})
	}

	return output, nil
}

// --- PinWriteService ---

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Id == "" {
		return &output, errors.New("please supply valid Pin.Id")
	}

	value, updated, err := s.es.GetStorage().GetPinWrite(in.Id)
	if err != nil {
		return &output, err
	}

	output.Id = in.Id
	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Pin.Name")
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, err
	}

	output.Name = in.Name

	value, updated, err := s.es.GetStorage().GetPinWrite(pin.ID)
	if err != nil {
		// 没有值也返回成功
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetWrite(ctx context.Context, in *pb.PinValue) error {
	if in.Id == "" {
		return errors.New("please supply valid Pin.Id")
	}

	// 验证 Pin 存在
	pin, err := s.es.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return err
	}

	// 检查可写性
	if pin.Rw == device.RO {
		return errors.New("pin is not writable")
	}

	updated := time.UnixMicro(in.Updated)
	if in.Updated == 0 {
		updated = time.Now()
	}

	// 设置写入值
	err = s.es.GetStorage().SetPinWrite(ctx, in.Id, in.Value, updated)
	if err != nil {
		return err
	}

	// 更新同步时间戳
	if err = s.es.GetSync().setPinWriteUpdated(ctx, updated); err != nil {
		return err
	}

	// 触发写入回调（如果有）
	if err = s.afterUpdateWrite(ctx, pin, in.Value); err != nil {
		return err
	}

	return nil
}

func (s *PinService) BatchSetWrite(ctx context.Context, values []*pb.PinValue) error {
	for _, in := range values {
		if err := s.SetWrite(ctx, in); err != nil {
			return err
		}
	}
	return nil
}

func (s *PinService) afterUpdateWrite(ctx context.Context, pin *storage.Pin, value *pb.NsonValue) error {
	// 可以在这里添加写入后的回调逻辑
	// 例如：通知驱动程序执行实际写入操作
	return nil
}

// --- 辅助方法 ---

// ParsePinName 解析 Pin 名称，支持 "wire.pin" 格式
func ParsePinName(name string) (wireName, pinName string, ok bool) {
	parts := strings.Split(name, ".")
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", name, false
}

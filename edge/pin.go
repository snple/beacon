package edge

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/snple/beacon/device"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	es *EdgeService

	edges.UnimplementedPinServiceServer
	edges.UnimplementedPinValueServiceServer
	edges.UnimplementedPinWriteServiceServer
}

func newPinService(es *EdgeService) *PinService {
	return &PinService{
		es: es,
	}
}

// --- PinService ---

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	pin, err := s.es.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	s.copyModelToOutput(&output, pin)

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	s.copyModelToOutput(&output, pin)
	output.Name = in.Name

	return &output, nil
}

func (s *PinService) List(ctx context.Context, in *edges.PinListRequest) (*edges.PinListResponse, error) {
	var output edges.PinListResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	var pins []*storage.Pin

	if in.WireId != "" {
		// 按 Wire 过滤
		wirePins, err := s.es.GetStorage().ListPinsByWire(in.WireId)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "ListPinsByWire: %v", err)
		}
		pins = wirePins
	} else {
		pins = s.es.GetStorage().ListPins()
	}

	for _, pin := range pins {
		item := pb.Pin{}
		s.copyModelToOutput(&item, pin)
		output.Pins = append(output.Pins, &item)
	}

	return &output, nil
}

func (s *PinService) ViewByID(ctx context.Context, id string) (*storage.Pin, error) {
	pin, err := s.es.GetStorage().GetPinByID(id)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	return pin, nil
}

func (s *PinService) ViewByName(ctx context.Context, name string) (*storage.Pin, error) {
	pin, err := s.es.GetStorage().GetPinByName(name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	return pin, nil
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

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	value, updated, err := s.es.GetStorage().GetPinValue(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "PinValue not found: %v", err)
	}

	output.Id = in.Id
	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		if in.Value == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// 验证 Pin 存在
	pin, err := s.es.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	// 验证值类型
	if !dt.ValidateNsonValue(in.Value, pin.Type) {
		return &output, status.Error(codes.InvalidArgument, "Invalid value for Pin.Type")
	}

	updated := time.Now()
	if in.Updated > 0 {
		updated = time.UnixMicro(in.Updated)
	}

	err = s.es.GetStorage().SetPinValue(ctx, in.Id, in.Value, updated)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinValue: %v", err)
	}

	// 更新同步时间戳
	if err = s.es.GetSync().setPinValueUpdated(ctx, updated); err != nil {
		return &output, status.Errorf(codes.Internal, "setPinValueUpdated: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
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

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if in.Value == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// 解析名称获取 Wire 和 Pin
	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	// 验证值类型
	if !dt.ValidateNsonValue(in.Value, pin.Type) {
		return &output, status.Error(codes.InvalidArgument, "Invalid value for Pin.Type")
	}

	updated := time.Now()
	if in.Updated > 0 {
		updated = time.UnixMicro(in.Updated)
	}

	err = s.es.GetStorage().SetPinValue(ctx, pin.ID, in.Value, updated)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinValue: %v", err)
	}

	// 更新同步时间戳
	if err = s.es.GetSync().setPinValueUpdated(ctx, updated); err != nil {
		return &output, status.Errorf(codes.Internal, "setPinValueUpdated: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) DeleteValue(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	err := s.es.GetStorage().DeletePinValue(ctx, in.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "DeletePinValue: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) PullValue(in *edges.PinPullValueRequest, stream grpc.ServerStreamingServer[pb.PinValue]) error {
	// basic validation
	if in == nil {
		return status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	after := time.UnixMicro(in.After)
	limit := int(in.Limit)
	if limit <= 0 {
		limit = 100
	}

	entries, err := s.es.GetStorage().ListPinValues(after, limit)
	if err != nil {
		return status.Errorf(codes.Internal, "ListPinValues: %v", err)
	}

	// 按更新时间排序
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Updated.Before(entries[j].Updated)
	})

	for _, entry := range entries {
		// 使用 dt.DecodeNsonValue 反序列化
		var value *pb.NsonValue
		if len(entry.Value) > 0 {
			var err error
			value, err = dt.DecodeNsonValue(entry.Value)
			if err != nil {
				return status.Errorf(codes.Internal, "DecodeNsonValue: %v", err)
			}
		}

		item := pb.PinValue{
			Id:      entry.ID,
			Value:   value,
			Updated: entry.Updated.UnixMicro(),
		}

		if err := stream.Send(&item); err != nil {
			return err
		}
	}

	return nil
}

// --- PinWriteService ---

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	value, updated, err := s.es.GetStorage().GetPinWrite(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "PinWrite not found: %v", err)
	}

	output.Id = in.Id
	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	pin, err := s.es.GetStorage().GetPinByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
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

func (s *PinService) PushWrite(stream grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]) error {
	var output pb.MyBool
	ctx := stream.Context()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&output)
		}
		if err != nil {
			return err
		}

		// basic validation
		if in.Id == "" {
			return status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		// 验证 Pin 存在
		pin, err := s.es.GetStorage().GetPinByID(in.Id)
		if err != nil {
			return status.Errorf(codes.NotFound, "Pin not found: %v", err)
		}

		// 检查可写性
		if pin.Rw == device.RO {
			return status.Error(codes.PermissionDenied, "Pin is not writable")
		}

		updated := time.UnixMicro(in.Updated)
		if in.Updated == 0 {
			updated = time.Now()
		}

		// 设置写入值
		err = s.es.GetStorage().SetPinWrite(ctx, in.Id, in.Value, updated)
		if err != nil {
			return status.Errorf(codes.Internal, "SetPinWrite: %v", err)
		}

		// 更新同步时间戳
		if err = s.es.GetSync().setPinWriteUpdated(ctx, updated); err != nil {
			return status.Errorf(codes.Internal, "setPinWriteUpdated: %v", err)
		}

		// 触发写入回调（如果有）
		if err = s.afterUpdateWrite(ctx, pin, in.Value); err != nil {
			return err
		}
	}
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

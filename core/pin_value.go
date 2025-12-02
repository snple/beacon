package core

import (
	"context"
	"io"
	"time"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinValueService struct {
	cs *CoreService

	cores.UnimplementedPinValueServiceServer
}

func newPinValueService(cs *CoreService) *PinValueService {
	return &PinValueService{
		cs: cs,
	}
}

func (s *PinValueService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
	}

	output.Id = in.Id

	value, updated, err := s.cs.GetStorage().GetPinValue(in.Id)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinValueService) SetValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
	}

	// 验证 Pin 存在
	_, err := s.cs.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, in.Id, in.Value, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinValue failed: %v", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *PinValueService) GetValueByName(ctx context.Context, in *cores.PinNameRequest) (*pb.PinNameValue, error) {
	var output pb.PinNameValue

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	output.Name = in.Name

	value, updated, err := s.cs.GetStorage().GetPinValue(pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinValueService) SetValueByName(ctx context.Context, in *cores.PinNameValueRequest) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId and Name")
	}

	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	err = s.cs.GetStorage().SetPinValue(ctx, pin.ID, in.Value, time.Now())
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinValue failed: %v", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *PinValueService) PushValue(stream grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]) error {
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
		_, err = s.cs.GetStorage().GetPinByID(in.Id)
		if err != nil {
			return status.Errorf(codes.NotFound, "Pin not found: %v", err)
		}

		err = s.cs.GetStorage().SetPinValue(ctx, in.Id, in.Value, time.UnixMicro(in.Updated))
		if err != nil {
			return status.Errorf(codes.Internal, "SetPinValue failed: %v", err)
		}
	}
}

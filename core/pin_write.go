package core

import (
	"context"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinWriteService struct {
	cs *CoreService

	cores.UnimplementedPinWriteServiceServer
}

func newPinWriteService(cs *CoreService) *PinWriteService {
	return &PinWriteService{
		cs: cs,
	}
}

func (s *PinWriteService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var output pb.PinValue

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
	}

	output.Id = in.Id

	value, updated, err := s.cs.GetStorage().GetPinWrite(in.Id)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinWriteService) SetWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" || in.Value == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id and Value")
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByID(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	if pin.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, in.Id, in.Value, time.Now())
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinWrite failed: %v", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *PinWriteService) GetWriteByName(ctx context.Context, in *cores.PinNameRequest) (*pb.PinNameValue, error) {
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

	value, updated, err := s.cs.GetStorage().GetPinWrite(pin.ID)
	if err != nil {
		// 如果未找到值，返回空值而不是错误
		return &output, nil
	}

	output.Value = value
	output.Updated = updated.UnixMicro()

	return &output, nil
}

func (s *PinWriteService) SetWriteByName(ctx context.Context, in *cores.PinNameValueRequest) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.NodeId == "" || in.Name == "" || in.Value == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId, Name and Value")
	}

	// 验证 Node 状态
	node, err := s.cs.GetStorage().GetNode(in.NodeId)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Node not found: %v", err)
	}

	if node.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
	}

	// 验证 Pin 存在且可写
	pin, err := s.cs.GetStorage().GetPinByName(in.NodeId, in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Pin not found: %v", err)
	}

	if pin.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	err = s.cs.GetStorage().SetPinWrite(ctx, pin.ID, in.Value, time.Now())
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetPinWrite failed: %v", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *PinWriteService) DeleteWrite(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
	}

	err := s.cs.GetStorage().DeletePinWrite(ctx, in.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "DeletePinWrite failed: %v", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *PinWriteService) PullWrite(in *cores.PinPullWriteRequest, stream grpc.ServerStreamingServer[pb.PinValue]) error {
	ctx := stream.Context()

	// basic validation
	if in == nil || in.NodeId == "" {
		return status.Error(codes.InvalidArgument, "Please supply valid NodeId")
	}

	writes, err := s.cs.GetStorage().ListPinWrites(in.NodeId, time.UnixMicro(in.After), int(in.Limit))
	if err != nil {
		return status.Errorf(codes.Internal, "ListPinWrites failed: %v", err)
	}

	for _, w := range writes {
		item := pb.PinValue{
			Id:      w.ID,
			Value:   w.Value,
			Updated: w.Updated.UnixMicro(),
		}

		if err := stream.Send(&item); err != nil {
			return err
		}
	}

	_ = ctx // 保持上下文引用
	return nil
}

package node

import (
	"context"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/pb/nodes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	ns *NodeService

	nodes.UnimplementedPinServiceServer
}

func newPinService(ns *NodeService) *PinService {
	return &PinService{
		ns: ns,
	}
}

func (s *PinService) Create(ctx context.Context, in *pb.Pin) (*pb.Pin, error) {
	var output pb.Pin

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	// source validation
	{
		nodeID, err := validateToken(ctx)
		if err != nil {
			return &output, err
		}

		request := &pb.Id{Id: in.GetSourceId()}

		reply, err := s.ns.Core().GetSource().View(ctx, request)
		if err != nil {
			return &output, err
		}

		if reply.GetNodeId() != nodeID {
			return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
		}
	}

	return s.ns.Core().GetPin().Create(ctx, in)
}

func (s *PinService) Update(ctx context.Context, in *pb.Pin) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().Update(ctx, in)
}

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return reply, nil
}

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinNameRequest{NodeId: nodeID, Name: in.GetName()}

	reply, err := s.ns.Core().GetPin().Name(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return reply, nil
}

func (s *PinService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().Delete(ctx, in)
}

func (s *PinService) List(ctx context.Context, in *nodes.PinListRequest) (*nodes.PinListResponse, error) {
	var err error
	var output nodes.PinListResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinListRequest{
		Page:     in.GetPage(),
		NodeId:   nodeID,
		SourceId: in.GetSourceId(),
		Tags:     in.GetTags(),
	}

	reply, err := s.ns.Core().GetPin().List(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Count = reply.Count
	output.Page = reply.GetPage()
	output.Pin = reply.GetPin()

	return &output, nil
}

func (s *PinService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewWithDeleted(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return reply, nil
}

func (s *PinService) Pull(ctx context.Context, in *nodes.PinPullRequest) (*nodes.PinPullResponse, error) {
	var err error
	var output nodes.PinPullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinPullRequest{
		After:    in.GetAfter(),
		Limit:    in.GetLimit(),
		NodeId:   nodeID,
		SourceId: in.GetSourceId(),
	}

	reply, err := s.ns.Core().GetPin().Pull(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Pin = reply.GetPin()

	return &output, nil
}

func (s *PinService) Sync(ctx context.Context, in *pb.Pin) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	in.NodeId = nodeID

	return s.ns.Core().GetPin().Sync(ctx, in)
}

// value

func (s *PinService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().GetValue(ctx, in)
}

func (s *PinService) SetValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().SetValue(ctx, in)
}

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().GetValueByName(ctx,
		&cores.PinGetValueByNameRequest{NodeId: nodeID, Name: in.GetName()})
	if err != nil {
		return &output, err
	}

	output.Id = reply.GetId()
	output.Name = reply.GetName()
	output.Value = reply.GetValue()
	output.Updated = reply.GetUpdated()

	return &output, nil
}

func (s *PinService) SetValueByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPin().SetValueByName(ctx,
		&cores.PinNameValue{NodeId: nodeID, Name: in.GetName(), Value: in.GetValue()})
}

func (s *PinService) ViewValue(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewValue(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return reply, nil
}

func (s *PinService) DeleteValue(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewValue(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().DeleteValue(ctx, in)
}

func (s *PinService) PullValue(ctx context.Context, in *nodes.PinPullValueRequest) (*nodes.PinPullValueResponse, error) {
	var err error
	var output nodes.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinPullValueRequest{
		After:    in.GetAfter(),
		Limit:    in.GetLimit(),
		NodeId:   nodeID,
		SourceId: in.GetSourceId(),
	}

	reply, err := s.ns.Core().GetPin().PullValue(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Pin = reply.GetPin()

	return &output, nil
}

func (s *PinService) SyncValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().SyncValue(ctx, in)
}

// write

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().View(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().GetWrite(ctx, in)
}

func (s *PinService) SetWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().SetWrite(ctx, in)
}

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().GetWriteByName(ctx,
		&cores.PinGetValueByNameRequest{NodeId: nodeID, Name: in.GetName()})
	if err != nil {
		return &output, err
	}

	output.Id = reply.GetId()
	output.Name = reply.GetName()
	output.Value = reply.GetValue()
	output.Updated = reply.GetUpdated()

	return &output, nil
}

func (s *PinService) SetWriteByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetPin().SetValueByName(ctx,
		&cores.PinNameValue{NodeId: nodeID, Name: in.GetName(), Value: in.GetValue()})
}

func (s *PinService) ViewWrite(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewWrite(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return reply, nil
}

func (s *PinService) DeleteWrite(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	reply, err := s.ns.Core().GetPin().ViewWrite(ctx, in)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().DeleteWrite(ctx, in)
}

func (s *PinService) PullWrite(ctx context.Context, in *nodes.PinPullValueRequest) (*nodes.PinPullValueResponse, error) {
	var err error
	var output nodes.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &cores.PinPullValueRequest{
		After:    in.GetAfter(),
		Limit:    in.GetLimit(),
		NodeId:   nodeID,
		SourceId: in.GetSourceId(),
	}

	reply, err := s.ns.Core().GetPin().PullWrite(ctx, request)
	if err != nil {
		return &output, err
	}

	output.Pin = reply.GetPin()

	return &output, nil
}

func (s *PinService) SyncWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	request := &pb.Id{Id: in.GetId()}

	reply, err := s.ns.Core().GetPin().View(ctx, request)
	if err != nil {
		return &output, err
	}

	if reply.GetNodeId() != nodeID {
		return &output, status.Error(codes.NotFound, "Query: reply.GetNodeId() != nodeID")
	}

	return s.ns.Core().GetPin().SyncWrite(ctx, in)
}

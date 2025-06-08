package node

import (
	"context"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/pb/nodes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SyncService struct {
	ns *NodeService

	nodes.UnimplementedSyncServiceServer
}

func newSyncService(ns *NodeService) *SyncService {
	return &SyncService{
		ns: ns,
	}
}

func (s *SyncService) SetNodeUpdated(ctx context.Context, in *nodes.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Updated")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetSync().SetNodeUpdated(ctx,
		&cores.SyncUpdated{Id: nodeID, Updated: in.Updated})
}

func (s *SyncService) GetNodeUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated
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

	reply, err := s.ns.Core().GetSync().GetNodeUpdated(ctx, &pb.Id{Id: nodeID})
	if err != nil {
		return &output, err
	}

	output.Updated = reply.Updated

	return &output, nil
}

func (s *SyncService) WaitNodeUpdated(in *pb.MyEmpty, stream nodes.SyncService_WaitNodeUpdatedServer) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitNodeUpdated(&pb.Id{Id: nodeID}, stream)
}

func (s *SyncService) SetPinValueUpdated(ctx context.Context, in *nodes.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value.Updated")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetSync().SetPinValueUpdated(ctx,
		&cores.SyncUpdated{Id: nodeID, Updated: in.Updated})
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated
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

	reply, err := s.ns.Core().GetSync().GetPinValueUpdated(ctx, &pb.Id{Id: nodeID})
	if err != nil {
		return &output, err
	}

	output.Updated = reply.Updated

	return &output, nil
}

func (s *SyncService) WaitPinValueUpdated(in *pb.MyEmpty, stream nodes.SyncService_WaitPinValueUpdatedServer) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitPinValueUpdated(&pb.Id{Id: nodeID}, stream)
}

func (s *SyncService) SetPinWriteUpdated(ctx context.Context, in *nodes.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Write.Updated")
		}
	}

	nodeID, err := validateToken(ctx)
	if err != nil {
		return &output, err
	}

	return s.ns.Core().GetSync().SetPinWriteUpdated(ctx,
		&cores.SyncUpdated{Id: nodeID, Updated: in.Updated})
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated
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

	reply, err := s.ns.Core().GetSync().GetPinWriteUpdated(ctx, &pb.Id{Id: nodeID})
	if err != nil {
		return &output, err
	}

	output.Updated = reply.Updated

	return &output, nil
}

func (s *SyncService) WaitPinWriteUpdated(in *pb.MyEmpty, stream nodes.SyncService_WaitPinWriteUpdatedServer) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitPinWriteUpdated(&pb.Id{Id: nodeID}, stream)
}

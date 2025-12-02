package node

import (
	"context"

	"github.com/snple/beacon/pb"
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

func (s *SyncService) GetNodeUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
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
	// basic validation
	if in == nil {
		return status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitNodeUpdated(&pb.Id{Id: nodeID}, stream)
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
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
	// basic validation
	if in == nil {
		return status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitPinValueUpdated(&pb.Id{Id: nodeID}, stream)
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context, in *pb.MyEmpty) (*nodes.SyncUpdated, error) {
	var output nodes.SyncUpdated

	// basic validation
	if in == nil {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
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
	// basic validation
	if in == nil {
		return status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	nodeID, err := validateToken(stream.Context())
	if err != nil {
		return err
	}

	return s.ns.Core().GetSync().WaitPinWriteUpdated(&pb.Id{Id: nodeID}, stream)
}

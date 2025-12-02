package core

import (
	"context"

	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	cs *CoreService

	cores.UnimplementedNodeServiceServer
}

func newNodeService(cs *CoreService) *NodeService {
	return &NodeService{
		cs: cs,
	}
}

func (s *NodeService) View(ctx context.Context, in *pb.Id) (*pb.Node, error) {
	var output pb.Node

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
	}

	node, err := s.cs.GetStorage().GetNode(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Node not found: %v", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) Name(ctx context.Context, in *pb.Name) (*pb.Node, error) {
	var output pb.Node

	// basic validation
	if in == nil || in.Name == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Name")
	}

	node, err := s.cs.GetStorage().GetNodeByName(in.Name)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Node not found: %v", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) List(ctx context.Context, in *pb.MyEmpty) (*cores.NodeListResponse, error) {
	var output cores.NodeListResponse

	nodes := s.cs.GetStorage().ListNodes()

	for _, node := range nodes {
		item := pb.Node{}
		s.copyStorageToOutput(&item, node)
		output.Nodes = append(output.Nodes, &item)
	}

	return &output, nil
}

func (s *NodeService) Push(ctx context.Context, in *cores.NodePushRequest) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" || len(in.Nson) == 0 {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID and NSON data")
	}

	err := s.cs.GetStorage().Push(ctx, in.Nson)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Push failed: %v", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *NodeService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
	}

	err := s.cs.GetStorage().DeleteNode(ctx, in.Id)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Delete failed: %v", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *NodeService) SetSecret(ctx context.Context, in *cores.NodeSecretRequest) (*pb.MyBool, error) {
	var output pb.MyBool

	// basic validation
	if in == nil || in.Id == "" || in.Secret == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID and Secret")
	}

	err := s.cs.GetStorage().SetSecret(ctx, in.Id, in.Secret)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "SetSecret failed: %v", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *NodeService) GetSecret(ctx context.Context, in *pb.Id) (*pb.Message, error) {
	var output pb.Message

	// basic validation
	if in == nil || in.Id == "" {
		return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
	}

	secret, err := s.cs.GetStorage().GetSecret(in.Id)
	if err != nil {
		return &output, status.Errorf(codes.NotFound, "Secret not found: %v", err)
	}

	output.Message = secret
	return &output, nil
}

func (s *NodeService) copyStorageToOutput(output *pb.Node, node *storage.Node) {
	output.Id = node.ID
	output.Name = node.Name
	output.Device = node.Device
	output.Tags = node.Tags
	output.Status = node.Status
	output.Updated = node.Updated.UnixMicro()

	for i := range node.Wires {
		wire := &node.Wires[i]
		pbWire := &pb.Wire{
			Id:   wire.ID,
			Name: wire.Name,
			Type: wire.Type,
			Tags: wire.Tags,
		}

		for j := range wire.Pins {
			pin := &wire.Pins[j]
			pbPin := &pb.Pin{
				Id:   pin.ID,
				Name: pin.Name,
				Addr: pin.Addr,
				Type: pin.Type,
				Rw:   pin.Rw,
				Tags: pin.Tags,
			}
			pbWire.Pins = append(pbWire.Pins, pbPin)
		}

		output.Wires = append(output.Wires, pbWire)
	}
}

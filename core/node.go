package core

import (
	"context"
	"fmt"

	"github.com/snple/beacon/dt"
)

type NodeService struct {
	cs *CoreService
}

func newNodeService(cs *CoreService) *NodeService {
	return &NodeService{
		cs: cs,
	}
}

func (s *NodeService) View(ctx context.Context, in *Id) (*Node, error) {
	var output Node

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	node, err := s.cs.GetStorage().GetNode(in.Id)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) Name(ctx context.Context, in *Name) (*Node, error) {
	var output Node

	// basic validation
	if in == nil || in.Name == "" {
		return &output, fmt.Errorf("please supply valid Node.Name")
	}

	node, err := s.cs.GetStorage().GetNodeByName(in.Name)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) List(ctx context.Context, in *MyEmpty) (*NodeListResponse, error) {
	var output NodeListResponse

	nodes := s.cs.GetStorage().ListNodes()

	for _, node := range nodes {
		item := Node{}
		s.copyStorageToOutput(&item, node)
		output.Nodes = append(output.Nodes, &item)
	}

	return &output, nil
}

func (s *NodeService) Push(ctx context.Context, in *NodePushRequest) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" || len(in.Nson) == 0 {
		return &output, fmt.Errorf("please supply valid Node.ID and NSON data")
	}

	err := s.cs.GetStorage().Push(ctx, in.Nson)
	if err != nil {
		return &output, fmt.Errorf("push failed: %w", err)
	}

	// TODO: 通知同步服务

	output.Bool = true
	return &output, nil
}

func (s *NodeService) Delete(ctx context.Context, in *Id) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	err := s.cs.GetStorage().DeleteNode(ctx, in.Id)
	if err != nil {
		return &output, fmt.Errorf("delete failed: %w", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *NodeService) SetSecret(ctx context.Context, in *NodeSecretRequest) (*MyBool, error) {
	var output MyBool

	// basic validation
	if in == nil || in.Id == "" || in.Secret == "" {
		return &output, fmt.Errorf("please supply valid Node.ID and Secret")
	}

	err := s.cs.GetStorage().SetSecret(ctx, in.Id, in.Secret)
	if err != nil {
		return &output, fmt.Errorf("setSecret failed: %w", err)
	}

	output.Bool = true
	return &output, nil
}

func (s *NodeService) GetSecret(ctx context.Context, in *Id) (*Message, error) {
	var output Message

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	secret, err := s.cs.GetStorage().GetSecret(in.Id)
	if err != nil {
		return &output, fmt.Errorf("secret not found: %w", err)
	}

	output.Message = secret
	return &output, nil
}

func (s *NodeService) copyStorageToOutput(output *Node, node *dt.Node) {
	output.Id = node.ID
	output.Name = node.Name
	output.Device = node.Device
	output.Tags = node.Tags
	output.Updated = node.Updated.UnixMicro()

	for i := range node.Wires {
		wire := &node.Wires[i]
		w := &Wire{
			Id:   wire.ID,
			Name: wire.Name,
			Type: wire.Type,
			Tags: wire.Tags,
		}

		for j := range wire.Pins {
			pin := &wire.Pins[j]
			p := &Pin{
				Id:   pin.ID,
				Name: pin.Name,
				Addr: pin.Addr,
				Type: pin.Type,
				Rw:   pin.Rw,
				Tags: pin.Tags,
			}
			w.Pins = append(w.Pins, p)
		}

		output.Wires = append(output.Wires, w)
	}
}

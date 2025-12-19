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

func (s *NodeService) View(ctx context.Context, nodeID string) (*Node, error) {
	var output Node

	// basic validation
	if nodeID == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	node, err := s.cs.GetStorage().GetNode(nodeID)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) Name(ctx context.Context, name string) (*Node, error) {
	var output Node

	// basic validation
	if name == "" {
		return &output, fmt.Errorf("please supply valid Node.Name")
	}

	node, err := s.cs.GetStorage().GetNodeByName(name)
	if err != nil {
		return &output, fmt.Errorf("node not found: %w", err)
	}

	s.copyStorageToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) List(ctx context.Context) ([]*Node, error) {
	nodes := s.cs.GetStorage().ListNodes()

	result := make([]*Node, 0, len(nodes))
	for _, node := range nodes {
		item := &Node{}
		s.copyStorageToOutput(item, node)
		result = append(result, item)
	}

	return result, nil
}

func (s *NodeService) Push(ctx context.Context, nodeID string, nsonData []byte) error {
	// basic validation
	if nodeID == "" || len(nsonData) == 0 {
		return fmt.Errorf("please supply valid Node.ID and NSON data")
	}

	err := s.cs.GetStorage().Push(ctx, nsonData)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// TODO: 通知同步服务

	return nil
}

func (s *NodeService) Delete(ctx context.Context, nodeID string) error {
	// basic validation
	if nodeID == "" {
		return fmt.Errorf("please supply valid Node.ID")
	}

	err := s.cs.GetStorage().DeleteNode(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func (s *NodeService) SetSecret(ctx context.Context, nodeID, secret string) error {
	// basic validation
	if nodeID == "" || secret == "" {
		return fmt.Errorf("please supply valid Node.ID and Secret")
	}

	err := s.cs.GetStorage().SetSecret(ctx, nodeID, secret)
	if err != nil {
		return fmt.Errorf("setSecret failed: %w", err)
	}

	return nil
}

func (s *NodeService) GetSecret(ctx context.Context, nodeID string) (string, error) {
	// basic validation
	if nodeID == "" {
		return "", fmt.Errorf("please supply valid Node.ID")
	}

	secret, err := s.cs.GetStorage().GetSecret(nodeID)
	if err != nil {
		return "", fmt.Errorf("secret not found: %w", err)
	}

	return secret, nil
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

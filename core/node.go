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

func (s *NodeService) View(nodeID string) (*dt.Node, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid Node.ID")
	}

	node, err := s.cs.GetStorage().GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	return node, nil
}

func (s *NodeService) List(ctx context.Context) ([]dt.Node, error) {
	nodes := s.cs.GetStorage().ListNodes()

	return nodes, nil
}

func (s *NodeService) Push(ctx context.Context, node *dt.Node) error {
	// basic validation
	if node == nil || node.ID == "" {
		return fmt.Errorf("please supply valid Node.ID")
	}

	err := s.cs.GetStorage().Push(node)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	// TODO: 通知同步服务

	return nil
}

func (s *NodeService) Delete(nodeID string) error {
	// basic validation
	if nodeID == "" {
		return fmt.Errorf("please supply valid Node.ID")
	}

	err := s.cs.GetStorage().DeleteNode(nodeID)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func (s *NodeService) SetSecret(nodeID, secret string) error {
	// basic validation
	if nodeID == "" || secret == "" {
		return fmt.Errorf("please supply valid Node.ID and Secret")
	}

	err := s.cs.GetStorage().SetSecret(nodeID, secret)
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

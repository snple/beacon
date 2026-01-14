package core

import (
	"fmt"

	"github.com/snple/beacon/dt"
)

// Node operations

func (cs *CoreService) ViewNode(nodeID string) (*dt.Node, error) {
	// basic validation
	if nodeID == "" {
		return nil, fmt.Errorf("please supply valid Node.ID")
	}

	node, err := cs.GetStorage().GetNode(nodeID)
	if err != nil {
		return nil, fmt.Errorf("node not found: %w", err)
	}

	return node, nil
}

func (cs *CoreService) ListNodes() ([]dt.Node, error) {
	nodes := cs.GetStorage().ListNodes()

	return nodes, nil
}

func (cs *CoreService) PushNode(node *dt.Node) error {
	// basic validation
	if node == nil || node.ID == "" {
		return fmt.Errorf("please supply valid Node.ID")
	}

	err := cs.GetStorage().Push(node)
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	return nil
}

func (cs *CoreService) DeleteNode(nodeID string) error {
	// basic validation
	if nodeID == "" {
		return fmt.Errorf("please supply valid Node.ID")
	}

	err := cs.GetStorage().DeleteNode(nodeID)
	if err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}

	return nil
}

func (cs *CoreService) SetNodeSecret(nodeID, secret string) error {
	// basic validation
	if nodeID == "" || secret == "" {
		return fmt.Errorf("please supply valid Node.ID and Secret")
	}

	err := cs.GetStorage().SetSecret(nodeID, secret)
	if err != nil {
		return fmt.Errorf("setSecret failed: %w", err)
	}

	return nil
}

func (cs *CoreService) GetNodeSecret(nodeID string) (string, error) {
	// basic validation
	if nodeID == "" {
		return "", fmt.Errorf("please supply valid Node.ID")
	}

	secret, err := cs.GetStorage().GetSecret(nodeID)
	if err != nil {
		return "", fmt.Errorf("secret not found: %w", err)
	}

	return secret, nil
}

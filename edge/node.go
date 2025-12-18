package edge

import (
	"context"
	"errors"
	"time"

	"github.com/snple/beacon/edge/storage"
	"github.com/snple/beacon/pb"
)

type NodeService struct {
	es *EdgeService
}

func newNodeService(es *EdgeService) *NodeService {
	return &NodeService{
		es: es,
	}
}

func (s *NodeService) Update(ctx context.Context, in *pb.Node) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	if in.Name == "" {
		return &output, errors.New("please supply valid Node.Name")
	}

	// name validation
	if len(in.Name) < 2 {
		return &output, errors.New("Node.Name min 2 character")
	}

	node, err := s.es.GetStorage().GetNode()
	if err != nil {
		return &output, err
	}

	// 更新名称
	if in.Name != node.Name {
		err = s.es.GetStorage().UpdateNodeName(ctx, in.Name)
		if err != nil {
			return &output, err
		}
	}

	// 重新获取更新后的节点
	node, err = s.es.GetStorage().GetNode()
	if err != nil {
		return &output, err
	}

	if err = s.afterUpdate(ctx); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) View(ctx context.Context, in *pb.MyEmpty) (*pb.Node, error) {
	var output pb.Node

	if in == nil {
		return &output, errors.New("please supply valid argument")
	}

	node, err := s.es.GetStorage().GetNode()
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, node)

	return &output, nil
}

func (s *NodeService) Destory(ctx context.Context, in *pb.MyEmpty) (*pb.MyBool, error) {
	var output pb.MyBool

	// 重置节点配置（清空 Wires）
	node, err := s.es.GetStorage().GetNode()
	if err != nil {
		return &output, err
	}

	// 保留节点基本信息，清空 Wires
	newNode := &storage.Node{
		ID:      node.ID,
		Name:    node.Name,
		Status:  node.Status,
		Updated: time.Now(),
		Wires:   nil,
	}

	err = s.es.GetStorage().SetNode(ctx, newNode)
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *NodeService) ViewByID(ctx context.Context) (*storage.Node, error) {
	node, err := s.es.GetStorage().GetNode()
	if err != nil {
		return nil, err
	}

	return node, nil
}

func (s *NodeService) copyModelToOutput(output *pb.Node, node *storage.Node) {
	output.Id = node.ID
	output.Name = node.Name
	output.Device = node.Device
	output.Tags = node.Tags
	output.Status = node.Status
	output.Updated = node.Updated.UnixMicro()

	// 复制 Wires
	for i := range node.Wires {
		wire := &node.Wires[i]
		pbWire := &pb.Wire{
			Id:   wire.ID,
			Name: wire.Name,
			Type: wire.Type,
			Tags: wire.Tags,
		}

		// 复制 Pins
		for j := range wire.Pins {
			pin := &wire.Pins[j]
			pbWire.Pins = append(pbWire.Pins, &pb.Pin{
				Id:   pin.ID,
				Name: pin.Name,
				Addr: pin.Addr,
				Type: pin.Type,
				Rw:   pin.Rw,
				Tags: pin.Tags,
			})
		}

		output.Wires = append(output.Wires, pbWire)
	}
}

func (s *NodeService) afterUpdate(ctx context.Context) error {
	err := s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return err
	}

	return nil
}

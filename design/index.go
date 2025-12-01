package design

import (
	"fmt"
	"sync"
)

type NodeStorage struct {
	mu sync.RWMutex

	Nodes        map[string]*Node  // id -> Node
	NodeNameToID map[string]string // name -> id

	Indexes map[string]*Index // node id -> Index（懒构建）
}

type Index struct {
	wireByID   map[string]*Wire // id -> Wire
	wireByName map[string]*Wire // name -> Wire

	pinByID   map[string]*Pin // id -> Pin
	pinByName map[string]*Pin // "wire_name.pin_name" -> Pin 或 "pin_name" -> Pin
	pinByAddr map[string]*Pin // "wire_id:address" -> Pin
}

// NewNodeStorage 创建存储
func NewNodeStorage() *NodeStorage {
	return &NodeStorage{
		Nodes:        make(map[string]*Node),
		NodeNameToID: make(map[string]string),
		Indexes:      make(map[string]*Index),
	}
}

// AddNode 添加节点
func (ns *NodeStorage) AddNode(node *Node) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if _, exists := ns.Nodes[node.ID]; exists {
		return fmt.Errorf("node already exists: %s", node.ID)
	}

	if _, exists := ns.NodeNameToID[node.Name]; exists {
		return fmt.Errorf("node name already exists: %s", node.Name)
	}

	ns.Nodes[node.ID] = node
	ns.NodeNameToID[node.Name] = node.ID

	return nil
}

// UpdateNode 更新节点（会清除索引）
func (ns *NodeStorage) UpdateNode(node *Node) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	oldNode, exists := ns.Nodes[node.ID]
	if !exists {
		return fmt.Errorf("node not found: %s", node.ID)
	}

	// 如果名称变了，更新名称索引
	if oldNode.Name != node.Name {
		if _, exists := ns.NodeNameToID[node.Name]; exists && ns.NodeNameToID[node.Name] != node.ID {
			return fmt.Errorf("node name already exists: %s", node.Name)
		}
		delete(ns.NodeNameToID, oldNode.Name)
		ns.NodeNameToID[node.Name] = node.ID
	}

	ns.Nodes[node.ID] = node

	// 清除索引
	delete(ns.Indexes, node.ID)

	return nil
}

// GetNode 获取节点
func (ns *NodeStorage) GetNode(nodeID string) (*Node, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	node, exists := ns.Nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	return node, nil
}

// GetNodeByName 按名称获取节点
func (ns *NodeStorage) GetNodeByName(name string) (*Node, error) {
	ns.mu.RLock()
	defer ns.mu.RUnlock()

	nodeID, exists := ns.NodeNameToID[name]
	if !exists {
		return nil, fmt.Errorf("node not found by name: %s", name)
	}

	return ns.Nodes[nodeID], nil
}

// GetWire 获取 Wire
func (ns *NodeStorage) GetWire(nodeID, wireID string) (*Wire, error) {
	ns.mu.Lock()
	// 确保索引存在
	if _, exists := ns.Indexes[nodeID]; !exists {
		if err := ns.buildIndexUnsafe(nodeID); err != nil {
			ns.mu.Unlock()
			return nil, err
		}
	}
	ns.mu.Unlock()

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	wire, exists := ns.Indexes[nodeID].wireByID[wireID]
	if !exists {
		return nil, fmt.Errorf("wire not found: %s", wireID)
	}

	return wire, nil
}

// GetWireByName 按名称获取 Wire
func (ns *NodeStorage) GetWireByName(nodeID, wireName string) (*Wire, error) {
	ns.mu.Lock()
	// 确保索引存在
	if _, exists := ns.Indexes[nodeID]; !exists {
		if err := ns.buildIndexUnsafe(nodeID); err != nil {
			ns.mu.Unlock()
			return nil, err
		}
	}
	ns.mu.Unlock()

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	wire, exists := ns.Indexes[nodeID].wireByName[wireName]
	if !exists {
		return nil, fmt.Errorf("wire not found by name: %s", wireName)
	}

	return wire, nil
}

// GetPin 获取 Pin
func (ns *NodeStorage) GetPin(nodeID, pinID string) (*Pin, error) {
	ns.mu.Lock()
	// 确保索引存在
	if _, exists := ns.Indexes[nodeID]; !exists {
		if err := ns.buildIndexUnsafe(nodeID); err != nil {
			ns.mu.Unlock()
			return nil, err
		}
	}
	ns.mu.Unlock()

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	pin, exists := ns.Indexes[nodeID].pinByID[pinID]
	if !exists {
		return nil, fmt.Errorf("pin not found: %s", pinID)
	}

	return pin, nil
}

// GetPinByName 按名称获取 Pin（支持 "wire.pin" 或 "pin"）
func (ns *NodeStorage) GetPinByName(nodeID, pinName string) (*Pin, error) {
	ns.mu.Lock()
	// 确保索引存在
	if _, exists := ns.Indexes[nodeID]; !exists {
		if err := ns.buildIndexUnsafe(nodeID); err != nil {
			ns.mu.Unlock()
			return nil, err
		}
	}
	ns.mu.Unlock()

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	pin, exists := ns.Indexes[nodeID].pinByName[pinName]
	if !exists {
		return nil, fmt.Errorf("pin not found by name: %s", pinName)
	}

	return pin, nil
}

// GetPinByAddr 按地址获取 Pin
func (ns *NodeStorage) GetPinByAddr(nodeID, wireID, addr string) (*Pin, error) {
	ns.mu.Lock()
	// 确保索引存在
	if _, exists := ns.Indexes[nodeID]; !exists {
		if err := ns.buildIndexUnsafe(nodeID); err != nil {
			ns.mu.Unlock()
			return nil, err
		}
	}
	ns.mu.Unlock()

	ns.mu.RLock()
	defer ns.mu.RUnlock()

	key := wireID + ":" + addr
	pin, exists := ns.Indexes[nodeID].pinByAddr[key]
	if !exists {
		return nil, fmt.Errorf("pin not found by addr: %s", key)
	}

	return pin, nil
}

// DeleteNode 删除节点
func (ns *NodeStorage) DeleteNode(nodeID string) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	node, exists := ns.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	delete(ns.Nodes, nodeID)
	delete(ns.NodeNameToID, node.Name)
	delete(ns.Indexes, nodeID)

	return nil
}

// buildIndexUnsafe 构建索引（无锁，内部调用）
func (ns *NodeStorage) buildIndexUnsafe(nodeID string) error {
	node, exists := ns.Nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	index := &Index{
		wireByID:   make(map[string]*Wire),
		wireByName: make(map[string]*Wire),
		pinByID:    make(map[string]*Pin),
		pinByName:  make(map[string]*Pin),
		pinByAddr:  make(map[string]*Pin),
	}

	// 构建 Wire 索引
	for i := range node.Wires {
		wire := &node.Wires[i]
		index.wireByID[wire.ID] = wire
		index.wireByName[wire.Name] = wire

		// 构建 Pin 索引
		for j := range wire.Pins {
			pin := &wire.Pins[j]

			// ID 索引
			index.pinByID[pin.ID] = pin

			// 名称索引：wire.pin
			fullName := wire.Name + "." + pin.Name
			index.pinByName[fullName] = pin

			// 短名称索引：pin（如果不冲突）
			if _, exists := index.pinByName[pin.Name]; !exists {
				index.pinByName[pin.Name] = pin
			}

			// 地址索引
			if pin.Addr != "" {
				key := wire.ID + ":" + pin.Addr
				index.pinByAddr[key] = pin
			}
		}
	}

	ns.Indexes[nodeID] = index
	return nil
}

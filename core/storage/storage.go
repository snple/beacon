// Package storage 实现 Beacon Core 端的存储抽象层。
//
// Storage 采用内存 + Badger 持久化的混合存储架构：
//   - 主存储：内存中维护全部节点和配置数据
//   - 持久化：通过 Badger 提供持久化支持
//   - 索引缓存：采用懒构建策略，按需构建查询索引
//
// 并发安全：所有公共方法使用读写锁保护，支持并发访问。
package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/pb"
)

// Node 节点配置
type Node struct {
	ID      string    `nson:"id"`
	Name    string    `nson:"name"`
	Status  int32     `nson:"status"`
	Updated time.Time `nson:"updated"`
	Wires   []Wire    `nson:"wires"`
}

// Wire 通道配置
type Wire struct {
	ID       string   `nson:"id"`
	Name     string   `nson:"name"`
	Type     string   `nson:"type"`
	Tags     []string `nson:"tags,omitempty"`
	Clusters []string `nson:"clusters,omitempty"`
	Pins     []Pin    `nson:"pins"`
}

// Pin 点位配置
type Pin struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Addr string   `nson:"addr"`
	Type uint32   `nson:"type"` // nson.DataType
	Rw   int32    `nson:"rw"`
	Tags []string `nson:"tags,omitempty"`
}

// Storage Core 端存储
type Storage struct {
	mu sync.RWMutex

	// 主存储
	nodes   map[string]*Node  // nodeID -> Node
	secrets map[string]string // nodeID -> secret

	// 索引/缓存
	index *index

	// Badger 持久化
	db *badger.DB
}

// index 查询索引（缓存）
type index struct {
	// 全局索引
	nodeByName map[string]*Node // nodeName -> Node
	wireByID   map[string]*Wire // wireID -> Wire（全局唯一）
	pinByID    map[string]*Pin  // pinID -> Pin（全局唯一）

	// 按节点的名称索引（懒构建）: nodeID -> (name -> item)
	wireByName map[string]map[string]*Wire // nodeID -> wireName -> Wire
	pinByName  map[string]map[string]*Pin  // nodeID -> pinName -> Pin
}

func newIndex() *index {
	return &index{
		nodeByName: make(map[string]*Node),
		wireByID:   make(map[string]*Wire),
		pinByID:    make(map[string]*Pin),
		wireByName: make(map[string]map[string]*Wire),
		pinByName:  make(map[string]map[string]*Pin),
	}
}

// getWireNameIndex 获取或创建节点的 wire 名称索引（无锁）
func (s *Storage) getWireNameIndex(nodeID string) map[string]*Wire {
	if m, ok := s.index.wireByName[nodeID]; ok {
		return m
	}
	m := make(map[string]*Wire)
	s.index.wireByName[nodeID] = m
	return m
}

// getPinNameIndex 获取或创建节点的 pin 名称索引（无锁）
func (s *Storage) getPinNameIndex(nodeID string) map[string]*Pin {
	if m, ok := s.index.pinByName[nodeID]; ok {
		return m
	}
	m := make(map[string]*Pin)
	s.index.pinByName[nodeID] = m
	return m
}

// New 创建一个新的 Storage 实例。
//
// 参数：
//   - db: Badger 数据库实例，用于持久化存储
//
// 返回：
//   - *Storage: 初始化后的存储实例
func New(db *badger.DB) *Storage {
	return &Storage{
		nodes:   make(map[string]*Node),
		secrets: make(map[string]string),
		index:   newIndex(),
		db:      db,
	}
}

// Load 启动时加载所有数据
// Load 从 Badger 数据库加载所有节点配置到内存。
//
// 此方法会遍历数据库中的所有节点数据，解析并加载到内存中。
// 同时会构建全局索引（节点名称、Wire ID、Pin ID）以加速查询。
//
// 参数：
//   - ctx: 上下文，用于取消操作
//
// 返回：
//   - error: 加载失败时返回错误
func (s *Storage) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 加载节点
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("node:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				node, err := decodeNode(val)
				if err != nil {
					return err
				}
				s.nodes[node.ID] = node
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 加载 secrets
	return s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte("secret:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.Key()[7:]) // 去掉 "secret:" 前缀
			err := item.Value(func(val []byte) error {
				s.secrets[key] = string(val)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Node 操作 ---

// GetNode 根据节点 ID 获取节点。
//
// 参数：
//   - nodeID: 节点 ID
//
// 返回：
//   - *Node: 节点对象
//   - error: 节点不存在时返回错误
func (s *Storage) GetNode(nodeID string) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	return node, nil
}

// GetNodeByName 按名称获取节点
func (s *Storage) GetNodeByName(name string) (*Node, error) {
	s.mu.RLock()
	if node, ok := s.index.nodeByName[name]; ok {
		s.mu.RUnlock()
		return node, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重检查
	if node, ok := s.index.nodeByName[name]; ok {
		return node, nil
	}

	for _, node := range s.nodes {
		if node.Name == name {
			s.index.nodeByName[name] = node
			return node, nil
		}
	}

	return nil, fmt.Errorf("node not found by name: %s", name)
}

// ListNodes 获取所有节点
func (s *Storage) ListNodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// --- Wire 操作 ---

// GetWireByID 按 ID 获取 Wire（全局唯一）
func (s *Storage) GetWireByID(wireID string) (*Wire, error) {
	s.mu.RLock()
	if wire, ok := s.index.wireByID[wireID]; ok {
		s.mu.RUnlock()
		return wire, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if wire, ok := s.index.wireByID[wireID]; ok {
		return wire, nil
	}

	for _, node := range s.nodes {
		for i := range node.Wires {
			if node.Wires[i].ID == wireID {
				wire := &node.Wires[i]
				s.index.wireByID[wireID] = wire
				return wire, nil
			}
		}
	}

	return nil, fmt.Errorf("wire not found: %s", wireID)
}

// GetWireByName 按名称获取 Wire
func (s *Storage) GetWireByName(nodeID, wireName string) (*Wire, error) {
	s.mu.RLock()
	if m, ok := s.index.wireByName[nodeID]; ok {
		if wire, ok := m[wireName]; ok {
			s.mu.RUnlock()
			return wire, nil
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if m, ok := s.index.wireByName[nodeID]; ok {
		if wire, ok := m[wireName]; ok {
			return wire, nil
		}
	}

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	for i := range node.Wires {
		if node.Wires[i].Name == wireName {
			wire := &node.Wires[i]
			s.getWireNameIndex(nodeID)[wireName] = wire
			return wire, nil
		}
	}

	return nil, fmt.Errorf("wire not found by name: %s", wireName)
}

// GetWireByFullName 按全名获取 Wire（格式：NodeName.WireName）
func (s *Storage) GetWireByFullName(fullName string) (*Wire, error) {
	parts := strings.Split(fullName, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid wire full name: %s, expected NodeName.WireName", fullName)
	}

	nodeName, wireName := parts[0], parts[1]

	node, err := s.GetNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	return s.GetWireByName(node.ID, wireName)
}

// ListWires 获取节点的所有 Wire
func (s *Storage) ListWires(nodeID string) ([]*Wire, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	wires := make([]*Wire, len(node.Wires))
	for i := range node.Wires {
		wires[i] = &node.Wires[i]
	}
	return wires, nil
}

// --- Pin 操作 ---

// GetPinByID 按 ID 获取 Pin（全局唯一）
func (s *Storage) GetPinByID(pinID string) (*Pin, error) {
	s.mu.RLock()
	if pin, ok := s.index.pinByID[pinID]; ok {
		s.mu.RUnlock()
		return pin, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if pin, ok := s.index.pinByID[pinID]; ok {
		return pin, nil
	}

	for _, node := range s.nodes {
		for i := range node.Wires {
			for j := range node.Wires[i].Pins {
				if node.Wires[i].Pins[j].ID == pinID {
					pin := &node.Wires[i].Pins[j]
					s.index.pinByID[pinID] = pin
					return pin, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("pin not found: %s", pinID)
}

// GetPinNodeID 获取 Pin 所属的 Node ID
func (s *Storage) GetPinNodeID(pinID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, node := range s.nodes {
		for i := range node.Wires {
			for j := range node.Wires[i].Pins {
				if node.Wires[i].Pins[j].ID == pinID {
					return node.ID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("pin not found: %s", pinID)
}

// GetPinByName 按名称获取 Pin（支持 "wire.pin"）
func (s *Storage) GetPinByName(nodeID, pinName string) (*Pin, error) {
	s.mu.RLock()
	if m, ok := s.index.pinByName[nodeID]; ok {
		if pin, ok := m[pinName]; ok {
			s.mu.RUnlock()
			return pin, nil
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if m, ok := s.index.pinByName[nodeID]; ok {
		if pin, ok := m[pinName]; ok {
			return pin, nil
		}
	}

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	for i := range node.Wires {
		wire := &node.Wires[i]
		for j := range wire.Pins {
			pin := &wire.Pins[j]
			fullName := wire.Name + "." + pin.Name
			if fullName == pinName {
				s.getPinNameIndex(nodeID)[pinName] = pin
				return pin, nil
			}
		}
	}

	return nil, fmt.Errorf("pin not found by name: %s", pinName)
}

// GetPinByFullName 按全名获取 Pin（格式：NodeName.WireName.PinName）
func (s *Storage) GetPinByFullName(fullName string) (*Pin, error) {
	parts := strings.Split(fullName, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid pin full name: %s, expected NodeName.WireName.PinName", fullName)
	}

	nodeName, wireName, pinName := parts[0], parts[1], parts[2]

	node, err := s.GetNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	// 使用 WireName.PinName 作为 pinName 查询
	return s.GetPinByName(node.ID, wireName+"."+pinName)
}

// ListPins 获取 Wire 的所有 Pin
func (s *Storage) ListPins(wireID string) ([]*Pin, error) {
	wire, err := s.GetWireByID(wireID)
	if err != nil {
		return nil, err
	}

	pins := make([]*Pin, len(wire.Pins))
	for i := range wire.Pins {
		pins[i] = &wire.Pins[i]
	}
	return pins, nil
}

// ListPinsByNode 获取节点的所有 Pin
func (s *Storage) ListPinsByNode(nodeID string) ([]*Pin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	var pins []*Pin
	for i := range node.Wires {
		for j := range node.Wires[i].Pins {
			pins = append(pins, &node.Wires[i].Pins[j])
		}
	}
	return pins, nil
}

// --- 同步操作 ---

// Push 接收 Edge 推送的节点配置（NSON 格式）
func (s *Storage) Push(ctx context.Context, data []byte) error {
	node, err := decodeNode(data)
	if err != nil {
		return fmt.Errorf("decode node: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 清除旧索引
	if old, ok := s.nodes[node.ID]; ok {
		s.clearNodeIndexUnsafe(old)
	}

	// 更新内存
	s.nodes[node.ID] = node
	s.index.nodeByName[node.Name] = node

	// 持久化
	return s.saveNodeUnsafe(node)
}

// DeleteNode 删除节点
func (s *Storage) DeleteNode(ctx context.Context, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	// 清除索引
	s.clearNodeIndexUnsafe(node)

	// 删除内存数据
	delete(s.nodes, nodeID)
	delete(s.secrets, nodeID)

	// 删除持久化数据
	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Delete([]byte("node:" + nodeID)); err != nil {
			return err
		}
		return txn.Delete([]byte("secret:" + nodeID))
	})
}

// --- Secret 操作 ---

// GetSecret 获取节点 Secret
func (s *Storage) GetSecret(nodeID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	secret, ok := s.secrets[nodeID]
	if !ok {
		return "", fmt.Errorf("secret not found: %s", nodeID)
	}
	return secret, nil
}

// SetSecret 设置节点 Secret
func (s *Storage) SetSecret(ctx context.Context, nodeID, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secrets[nodeID] = secret

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("secret:"+nodeID), []byte(secret))
	})
}

// --- PinValue 操作 ---

// PinValueEntry 点位值条目
type PinValueEntry struct {
	ID      string    `nson:"id"`
	Value   []byte    `nson:"value"` // NSON 序列化的 pb.NsonValue
	Updated time.Time `nson:"updated"`
}

const (
	PIN_VALUE_PREFIX = "pv:" // pv:{nodeID}:{pinID}
	PIN_WRITE_PREFIX = "pw:" // pw:{nodeID}:{pinID}
)

// GetPinValue 获取点位值
func (s *Storage) GetPinValue(nodeID, pinID string) (*pb.NsonValue, time.Time, error) {
	var value *pb.NsonValue
	var updated time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PIN_VALUE_PREFIX + nodeID + ":" + pinID))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			buf := bytes.NewBuffer(val)
			m, err := nson.DecodeMap(buf)
			if err != nil {
				return err
			}

			var entry PinValueEntry
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
			}

			// 使用 dt.DecodeNsonValue 反序列化
			if len(entry.Value) > 0 {
				value, err = dt.DecodeNsonValue(entry.Value)
				if err != nil {
					return err
				}
			}
			updated = entry.Updated
			return nil
		})
	})

	if err != nil {
		return nil, time.Time{}, err
	}

	return value, updated, nil
}

// SetPinValue 设置点位值
func (s *Storage) SetPinValue(ctx context.Context, nodeID, pinID string, value *pb.NsonValue, updated time.Time) error {
	// 使用 dt.EncodeNsonValue 序列化
	valueBytes, err := dt.EncodeNsonValue(value)
	if err != nil {
		return err
	}

	entry := PinValueEntry{
		ID:      pinID,
		Value:   valueBytes,
		Updated: updated,
	}

	m, err := nson.Marshal(entry)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(PIN_VALUE_PREFIX+nodeID+":"+pinID), buf.Bytes())
	})
}

// ListPinValues 列出节点的点位值（使用前缀迭代器）
func (s *Storage) ListPinValues(nodeID string, after time.Time, limit int) ([]PinValueEntry, error) {
	var result []PinValueEntry

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// 使用 pv:{nodeID}: 前缀扫描
		prefix := []byte(PIN_VALUE_PREFIX + nodeID + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				buf := bytes.NewBuffer(val)
				m, err := nson.DecodeMap(buf)
				if err != nil {
					return err
				}

				var entry PinValueEntry
				if err := nson.Unmarshal(m, &entry); err != nil {
					return err
				}

				if entry.Updated.After(after) {
					result = append(result, entry)
				}
				return nil
			})
			if err != nil {
				return err
			}

			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// --- PinWrite 操作 ---

// GetPinWrite 获取点位写入值
func (s *Storage) GetPinWrite(nodeID, pinID string) (*pb.NsonValue, time.Time, error) {
	var value *pb.NsonValue
	var updated time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PIN_WRITE_PREFIX + nodeID + ":" + pinID))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			buf := bytes.NewBuffer(val)
			m, err := nson.DecodeMap(buf)
			if err != nil {
				return err
			}

			var entry PinValueEntry
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
			}

			// 使用 dt.DecodeNsonValue 反序列化
			if len(entry.Value) > 0 {
				value, err = dt.DecodeNsonValue(entry.Value)
				if err != nil {
					return err
				}
			}
			updated = entry.Updated
			return nil
		})
	})

	if err != nil {
		return nil, time.Time{}, err
	}

	return value, updated, nil
}

// SetPinWrite 设置点位写入值
func (s *Storage) SetPinWrite(ctx context.Context, nodeID, pinID string, value *pb.NsonValue, updated time.Time) error {
	// 使用 dt.EncodeNsonValue 序列化
	valueBytes, err := dt.EncodeNsonValue(value)
	if err != nil {
		return err
	}

	entry := PinValueEntry{
		ID:      pinID,
		Value:   valueBytes,
		Updated: updated,
	}

	m, err := nson.Marshal(entry)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(PIN_WRITE_PREFIX+nodeID+":"+pinID), buf.Bytes())
	})
}

// DeletePinWrite 删除点位写入值
func (s *Storage) DeletePinWrite(ctx context.Context, nodeID, pinID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(PIN_WRITE_PREFIX + nodeID + ":" + pinID))
	})
}

// ListPinWrites 列出节点的写入值（使用前缀迭代器，避免扫描所有 Pin）
func (s *Storage) ListPinWrites(nodeID string, after time.Time, limit int) ([]PinValueEntry, error) {
	var result []PinValueEntry

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// 使用 pw:{nodeID}: 前缀扫描
		prefix := []byte(PIN_WRITE_PREFIX + nodeID + ":")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				buf := bytes.NewBuffer(val)
				m, err := nson.DecodeMap(buf)
				if err != nil {
					return err
				}

				var entry PinValueEntry
				if err := nson.Unmarshal(m, &entry); err != nil {
					return err
				}

				if entry.Updated.After(after) {
					result = append(result, entry)
				}
				return nil
			})
			if err != nil {
				return err
			}

			if limit > 0 && len(result) >= limit {
				break
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// --- 内部方法 ---

// clearNodeIndexUnsafe 清除节点相关的所有索引（无锁）
func (s *Storage) clearNodeIndexUnsafe(node *Node) {
	delete(s.index.nodeByName, node.Name)
	delete(s.index.wireByName, node.ID)
	delete(s.index.pinByName, node.ID)

	for i := range node.Wires {
		wire := &node.Wires[i]
		delete(s.index.wireByID, wire.ID)
		for j := range wire.Pins {
			delete(s.index.pinByID, wire.Pins[j].ID)
		}
	}
}

func (s *Storage) saveNodeUnsafe(node *Node) error {
	data, err := encodeNode(node)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("node:"+node.ID), data)
	})
}

// --- 编解码 ---

func encodeNode(node *Node) ([]byte, error) {
	m, err := nson.Marshal(node)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decodeNode(data []byte) (*Node, error) {
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return nil, err
	}

	var node Node
	if err := nson.Unmarshal(m, &node); err != nil {
		return nil, err
	}

	return &node, nil
}

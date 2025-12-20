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
	"sort"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/dt"
)

// Storage Core 端存储
type Storage struct {
	mu sync.RWMutex

	// 主存储
	nodes   map[string]*dt.Node // nodeID -> Node
	secrets map[string]string   // nodeID -> secret

	// 索引/缓存
	index *index

	// Badger 持久化
	db *badger.DB
}

// index 查询索引（缓存）
type index struct {
	// 全局索引
	wireByID map[string]*dt.Wire // wireID -> Wire（全局唯一）
	pinByID  map[string]*dt.Pin  // pinID -> Pin（全局唯一）
}

func newIndex() *index {
	return &index{
		wireByID: make(map[string]*dt.Wire),
		pinByID:  make(map[string]*dt.Pin),
	}
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
		nodes:   make(map[string]*dt.Node),
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
				node, err := dt.DecodeNode(val)
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
//   - *dt.Node: 节点对象
//   - error: 节点不存在时返回错误
func (s *Storage) GetNode(nodeID string) (*dt.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}
	return node, nil
}

// ListNodes 获取所有节点
func (s *Storage) ListNodes() []dt.Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]dt.Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, dt.DeepCopyNode(node))
	}
	return nodes
}

// --- Wire 操作 ---

// GetWireByID 按 ID 获取 Wire（全局唯一）
func (s *Storage) GetWireByID(wireID string) (*dt.Wire, error) {
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

// ListWires 获取节点的所有 Wire
func (s *Storage) ListWires(nodeID string) ([]dt.Wire, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	wires := make([]dt.Wire, len(node.Wires))
	for i := range node.Wires {
		wires[i] = dt.DeepCopyWire(&node.Wires[i])
	}
	return wires, nil
}

// --- Pin 操作 ---

// GetPinByID 按 ID 获取 Pin（全局唯一）
func (s *Storage) GetPinByID(pinID string) (*dt.Pin, error) {
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

// ListPins 获取 Wire 的所有 Pin
func (s *Storage) ListPins(wireID string) ([]dt.Pin, error) {
	wire, err := s.GetWireByID(wireID)
	if err != nil {
		return nil, err
	}

	pins := make([]dt.Pin, len(wire.Pins))
	for i := range wire.Pins {
		pins[i] = dt.DeepCopyPin(&wire.Pins[i])
	}
	return pins, nil
}

// ListPinsByNode 获取节点的所有 Pin
func (s *Storage) ListPinsByNode(nodeID string) ([]dt.Pin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("node not found: %s", nodeID)
	}

	var pins []dt.Pin
	for i := range node.Wires {
		for j := range node.Wires[i].Pins {
			// 深拷贝 Pin
			pins = append(pins, dt.DeepCopyPin(&node.Wires[i].Pins[j]))
		}
	}
	return pins, nil
}

// --- 同步操作 ---

// Push 接收 Edge 推送的节点配置（NSON 格式）
func (s *Storage) Push(node *dt.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node == nil {
		return fmt.Errorf("node is nil")
	}

	// 清除旧索引
	if old, ok := s.nodes[node.ID]; ok {
		s.clearNodeIndexUnsafe(old)
	}

	// 更新内存
	s.nodes[node.ID] = node

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

	// 使用 NewTransactionAt + CommitAt 删除持久化数据
	commitTs := uint64(time.Now().UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Delete([]byte("node:" + nodeID)); err != nil {
		return err
	}
	if err := txn.Delete([]byte("secret:" + nodeID)); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
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

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(time.Now().UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte("secret:"+nodeID), []byte(secret)); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// --- PinValue 操作 ---

const (
	PIN_VALUE_PREFIX = "pv:" // pv:{nodeID}:{pinID}
	PIN_WRITE_PREFIX = "pw:" // pw:{nodeID}:{pinID}
)

// GetPinValue 获取点位值
func (s *Storage) GetPinValue(nodeID, pinID string) (nson.Value, time.Time, error) {
	var value nson.Value = nson.Null{}
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

			var entry dt.PinValue
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
			}

			value = entry.Value
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
func (s *Storage) SetPinValue(ctx context.Context, nodeID string, value dt.PinValue) error {
	m, err := nson.Marshal(value)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(value.Updated.UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte(PIN_VALUE_PREFIX+nodeID+":"+value.ID), buf.Bytes()); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// ListPinValues 列出节点的点位值（使用 SinceTs 优化查询）
func (s *Storage) ListPinValues(nodeID string, after time.Time, limit int) ([]dt.PinValue, error) {
	var result []dt.PinValue

	txn := s.db.NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.SinceTs = uint64(after.UnixMicro()) // 只读取 version > sinceTs 的数据
	it := txn.NewIterator(opts)
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

			var entry dt.PinValue
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
			}

			result = append(result, entry)
			return nil
		})
		if err != nil {
			return nil, err
		}

	}

	// 按 Updated 时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Updated.Before(result[j].Updated)
	})

	// 应用 limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// --- PinWrite 操作 ---

// GetPinWrite 获取点位写入值
func (s *Storage) GetPinWrite(nodeID, pinID string) (nson.Value, time.Time, error) {
	var value nson.Value = nson.Null{}
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

			var entry dt.PinValue
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
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
func (s *Storage) SetPinWrite(ctx context.Context, nodeID string, value dt.PinValue) error {
	m, err := nson.Marshal(value)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(value.Updated.UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte(PIN_WRITE_PREFIX+nodeID+":"+value.ID), buf.Bytes()); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// DeletePinWrite 删除点位写入值
func (s *Storage) DeletePinWrite(ctx context.Context, nodeID, pinID string) error {
	// 使用 NewTransactionAt + CommitAt 删除
	commitTs := uint64(time.Now().UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Delete([]byte(PIN_WRITE_PREFIX + nodeID + ":" + pinID)); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// ListPinWrites 列出节点的写入值（使用 SinceTs 优化查询）
func (s *Storage) ListPinWrites(nodeID string, after time.Time, limit int) ([]dt.PinValue, error) {
	var result []dt.PinValue

	txn := s.db.NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.SinceTs = uint64(after.UnixMicro()) // 只读取 version > sinceTs 的数据
	it := txn.NewIterator(opts)
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

			var entry dt.PinValue
			if err := nson.Unmarshal(m, &entry); err != nil {
				return err
			}

			result = append(result, entry)
			return nil
		})
		if err != nil {
			return nil, err
		}

	}

	// 按 Updated 时间排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Updated.Before(result[j].Updated)
	})

	// 应用 limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// --- 内部方法 ---

// clearNodeIndexUnsafe 清除节点相关的所有索引（无锁）
func (s *Storage) clearNodeIndexUnsafe(node *dt.Node) {
	for i := range node.Wires {
		wire := &node.Wires[i]
		delete(s.index.wireByID, wire.ID)
		for j := range wire.Pins {
			delete(s.index.pinByID, wire.Pins[j].ID)
		}
	}
}

func (s *Storage) saveNodeUnsafe(node *dt.Node) error {
	data, err := dt.EncodeNode(node)
	if err != nil {
		return err
	}

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(node.Updated.UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte("node:"+node.ID), data); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

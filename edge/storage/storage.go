package storage

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

// Node 节点配置（Edge 只有一个 Node）
type Node struct {
	ID      string    `nson:"id"`
	Name    string    `nson:"name"`
	Tags    []string  `nson:"tags,omitempty"`   // 节点标签列表
	Device  string    `nson:"device,omitempty"` // 设备模板 ID（指向 device.Device.ID）
	Status  int32     `nson:"status"`
	Updated time.Time `nson:"updated"`
	Wires   []Wire    `nson:"wires"`
}

// Wire 通道配置
type Wire struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Type string   `nson:"type"`
	Pins []Pin    `nson:"pins"`
}

// Pin 点位配置
type Pin struct {
	ID   string   `nson:"id"`
	Name string   `nson:"name"`
	Tags []string `nson:"tags,omitempty"`
	Addr string   `nson:"addr"`
	Type uint32   `nson:"type"` // nson.DataType
	Rw   int32    `nson:"rw"`
}

// Storage Edge 端存储（单 Node）
type Storage struct {
	mu sync.RWMutex

	// 主存储（只有一个 Node）
	node   *Node
	secret string

	// 索引/缓存
	index *index

	// Badger 持久化
	db *badger.DB
}

// index 查询索引（缓存）
type index struct {
	// 按 ID 的索引
	wireByID map[string]*Wire // wireID -> Wire
	pinByID  map[string]*Pin  // pinID -> Pin

	// 按名称的索引
	wireByName map[string]*Wire // wireName -> Wire
	pinByName  map[string]*Pin  // "wire.pin" -> Pin
}

func newIndex() *index {
	return &index{
		wireByID:   make(map[string]*Wire),
		pinByID:    make(map[string]*Pin),
		wireByName: make(map[string]*Wire),
		pinByName:  make(map[string]*Pin),
	}
}

// New 创建存储
func New(db *badger.DB) *Storage {
	return &Storage{
		index: newIndex(),
		db:    db,
	}
}

// Load 启动时加载数据
func (s *Storage) Load(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 加载节点配置
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("node"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // 没有节点数据是正常的
			}
			return err
		}

		return item.Value(func(val []byte) error {
			node, err := decodeNode(val)
			if err != nil {
				return err
			}
			s.node = node
			return nil
		})
	})
	if err != nil {
		return err
	}

	// 加载 secret
	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("secret"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			s.secret = string(val)
			return nil
		})
	})
	if err != nil {
		return err
	}

	// 重建索引
	if s.node != nil {
		s.rebuildIndexUnsafe()
	}

	return nil
}

// --- Node 操作 ---

// GetNode 获取节点
func (s *Storage) GetNode() (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return nil, fmt.Errorf("node not initialized")
	}
	return s.node, nil
}

// GetNodeID 获取节点 ID
func (s *Storage) GetNodeID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return ""
	}
	return s.node.ID
}

// SetNode 设置/更新节点配置
func (s *Storage) SetNode(ctx context.Context, node *Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新内存
	s.node = node

	// 重建索引
	s.clearIndexUnsafe()
	s.rebuildIndexUnsafe()

	// 持久化
	return s.saveNodeUnsafe()
}

// UpdateNodeName 更新节点名称
func (s *Storage) UpdateNodeName(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.node == nil {
		return fmt.Errorf("node not initialized")
	}

	s.node.Name = name
	s.node.Updated = time.Now()

	return s.saveNodeUnsafe()
}

// UpdateNodeStatus 更新节点状态
func (s *Storage) UpdateNodeStatus(ctx context.Context, status int32) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.node == nil {
		return fmt.Errorf("node not initialized")
	}

	s.node.Status = status
	s.node.Updated = time.Now()

	return s.saveNodeUnsafe()
}

// --- Secret 操作 ---

// GetSecret 获取节点 Secret
func (s *Storage) GetSecret() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.secret == "" {
		return "", fmt.Errorf("secret not set")
	}
	return s.secret, nil
}

// SetSecret 设置节点 Secret
func (s *Storage) SetSecret(ctx context.Context, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.secret = secret

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("secret"), []byte(secret))
	})
}

// --- Wire 操作 ---

// GetWireByID 按 ID 获取 Wire
func (s *Storage) GetWireByID(wireID string) (*Wire, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wire, ok := s.index.wireByID[wireID]; ok {
		return wire, nil
	}

	return nil, fmt.Errorf("wire not found: %s", wireID)
}

// GetWireByName 按名称获取 Wire
func (s *Storage) GetWireByName(wireName string) (*Wire, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wire, ok := s.index.wireByName[wireName]; ok {
		return wire, nil
	}

	return nil, fmt.Errorf("wire not found by name: %s", wireName)
}

// ListWires 获取所有 Wire
func (s *Storage) ListWires() []*Wire {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return nil
	}

	wires := make([]*Wire, len(s.node.Wires))
	for i := range s.node.Wires {
		wires[i] = &s.node.Wires[i]
	}
	return wires
}

// --- Pin 操作 ---

// GetPinByID 按 ID 获取 Pin
func (s *Storage) GetPinByID(pinID string) (*Pin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pin, ok := s.index.pinByID[pinID]; ok {
		return pin, nil
	}

	return nil, fmt.Errorf("pin not found: %s", pinID)
}

// GetPinByName 按名称获取 Pin（支持 "wire.pin" 格式）
func (s *Storage) GetPinByName(pinName string) (*Pin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if pin, ok := s.index.pinByName[pinName]; ok {
		return pin, nil
	}

	return nil, fmt.Errorf("pin not found by name: %s", pinName)
}

// GetPinWireID 获取 Pin 所属的 Wire ID
func (s *Storage) GetPinWireID(pinID string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return "", fmt.Errorf("node not initialized")
	}

	for i := range s.node.Wires {
		wire := &s.node.Wires[i]
		for j := range wire.Pins {
			if wire.Pins[j].ID == pinID {
				return wire.ID, nil
			}
		}
	}

	return "", fmt.Errorf("pin not found: %s", pinID)
}

// ListPins 获取所有 Pin
func (s *Storage) ListPins() []*Pin {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return nil
	}

	var pins []*Pin
	for i := range s.node.Wires {
		for j := range s.node.Wires[i].Pins {
			pins = append(pins, &s.node.Wires[i].Pins[j])
		}
	}
	return pins
}

// ListPinsByWire 获取 Wire 的所有 Pin
func (s *Storage) ListPinsByWire(wireID string) ([]*Pin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return nil, fmt.Errorf("node not initialized")
	}

	for i := range s.node.Wires {
		if s.node.Wires[i].ID == wireID {
			wire := &s.node.Wires[i]
			pins := make([]*Pin, len(wire.Pins))
			for j := range wire.Pins {
				pins[j] = &wire.Pins[j]
			}
			return pins, nil
		}
	}

	return nil, fmt.Errorf("wire not found: %s", wireID)
}

// --- PinValue 操作 ---

// PinValueEntry 点位值条目
type PinValueEntry struct {
	ID      string    `nson:"id"`
	Value   []byte    `nson:"value"` // 序列化的 nson.Value（使用 nson.EncodeValue）
	Updated time.Time `nson:"updated"`
}

const (
	PIN_VALUE_PREFIX = "pv:"
	PIN_WRITE_PREFIX = "pw:"
)

// GetPinValue 获取点位值
func (s *Storage) GetPinValue(pinID string) (nson.Value, time.Time, error) {
	var value nson.Value
	var updated time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PIN_VALUE_PREFIX + pinID))
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

			if len(entry.Value) > 0 {
				vv, err := nson.DecodeValue(bytes.NewBuffer(entry.Value))
				if err != nil {
					return err
				}
				value = vv
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
func (s *Storage) SetPinValue(ctx context.Context, pinID string, value nson.Value, updated time.Time) error {
	var valueBytes []byte
	if value != nil {
		bufVal := new(bytes.Buffer)
		if err := nson.EncodeValue(bufVal, value); err != nil {
			return err
		}
		valueBytes = bufVal.Bytes()
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

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(updated.UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte(PIN_VALUE_PREFIX+pinID), buf.Bytes()); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// DeletePinValue 删除点位值
func (s *Storage) DeletePinValue(ctx context.Context, pinID string) error {
	// 使用 NewTransactionAt + CommitAt 删除
	commitTs := uint64(time.Now().UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Delete([]byte(PIN_VALUE_PREFIX + pinID)); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// ListPinValues 列出点位值（使用 SinceTs 优化查询）
func (s *Storage) ListPinValues(after time.Time, limit int) ([]PinValueEntry, error) {
	var result []PinValueEntry

	// 将 after 转换为微秒时间戳作为 SinceTs
	sinceTs := uint64(after.UnixMicro())
	// 使用足够大的 readTs 读取所有当前数据
	readTs := uint64(time.Now().Add(time.Hour).UnixMicro())

	txn := s.db.NewTransactionAt(readTs, false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.SinceTs = sinceTs // 只读取 version > sinceTs 的数据
	it := txn.NewIterator(opts)
	defer it.Close()

	prefix := []byte(PIN_VALUE_PREFIX)
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
func (s *Storage) GetPinWrite(pinID string) (nson.Value, time.Time, error) {
	var value nson.Value
	var updated time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(PIN_WRITE_PREFIX + pinID))
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

			if len(entry.Value) > 0 {
				vv, err := nson.DecodeValue(bytes.NewBuffer(entry.Value))
				if err != nil {
					return err
				}
				value = vv
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
func (s *Storage) SetPinWrite(ctx context.Context, pinID string, value nson.Value, updated time.Time) error {
	var valueBytes []byte
	if value != nil {
		bufVal := new(bytes.Buffer)
		if err := nson.EncodeValue(bufVal, value); err != nil {
			return err
		}
		valueBytes = bufVal.Bytes()
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

	// 使用 NewTransactionAt + CommitAt 写入
	commitTs := uint64(updated.UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Set([]byte(PIN_WRITE_PREFIX+pinID), buf.Bytes()); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// DeletePinWrite 删除点位写入值
func (s *Storage) DeletePinWrite(ctx context.Context, pinID string) error {
	// 使用 NewTransactionAt + CommitAt 删除
	commitTs := uint64(time.Now().UnixMicro())
	txn := s.db.NewTransactionAt(commitTs, true)
	defer txn.Discard()

	if err := txn.Delete([]byte(PIN_WRITE_PREFIX + pinID)); err != nil {
		return err
	}

	return txn.CommitAt(commitTs, nil)
}

// ListPinWrites 列出点位写入值（使用 SinceTs 优化查询）
func (s *Storage) ListPinWrites(after time.Time, limit int) ([]PinValueEntry, error) {
	var result []PinValueEntry

	txn := s.db.NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	opts := badger.DefaultIteratorOptions
	opts.SinceTs = uint64(after.UnixMicro()) // 只读取 version > sinceTs 的数据
	it := txn.NewIterator(opts)
	defer it.Close()

	prefix := []byte(PIN_WRITE_PREFIX)
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

// --- Sync 时间戳操作 ---

const (
	SYNC_PREFIX                = "sync:"
	SYNC_NODE                  = "sync:node"      // 本地配置数据最新时间戳
	SYNC_PIN_VALUE             = "sync:pin_value" // 本地 PinValue 最新时间戳
	SYNC_PIN_WRITE             = "sync:pin_write" // 本地 PinWrite 最新时间戳
	SYNC_NODE_TO_REMOTE        = "sync:node_ltr"  // 配置数据已同步到 Core 的时间戳
	SYNC_PIN_VALUE_TO_REMOTE   = "sync:pv_ltr"    // PinValue 已同步到 Core 的时间戳
	SYNC_PIN_WRITE_FROM_REMOTE = "sync:pw_rtl"    // PinWrite 已从 Core 拉取的时间戳
)

// GetSyncTime 获取同步时间戳
func (s *Storage) GetSyncTime(key string) (time.Time, error) {
	var t time.Time

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			if len(val) != 8 {
				return fmt.Errorf("invalid sync time format")
			}
			usec := int64(val[0]) | int64(val[1])<<8 | int64(val[2])<<16 | int64(val[3])<<24 |
				int64(val[4])<<32 | int64(val[5])<<40 | int64(val[6])<<48 | int64(val[7])<<56
			t = time.UnixMicro(usec)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	return t, nil
}

// SetSyncTime 设置同步时间戳
func (s *Storage) SetSyncTime(key string, t time.Time) error {
	usec := t.UnixMicro()
	val := []byte{
		byte(usec), byte(usec >> 8), byte(usec >> 16), byte(usec >> 24),
		byte(usec >> 32), byte(usec >> 40), byte(usec >> 48), byte(usec >> 56),
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), val)
	})
}

// --- 内部方法 ---

// clearIndexUnsafe 清除索引（无锁）
func (s *Storage) clearIndexUnsafe() {
	s.index = newIndex()
}

// rebuildIndexUnsafe 重建索引（无锁）
func (s *Storage) rebuildIndexUnsafe() {
	if s.node == nil {
		return
	}

	for i := range s.node.Wires {
		wire := &s.node.Wires[i]
		s.index.wireByID[wire.ID] = wire
		s.index.wireByName[wire.Name] = wire

		for j := range wire.Pins {
			pin := &wire.Pins[j]
			s.index.pinByID[pin.ID] = pin
			// 使用 "wire.pin" 作为名称索引
			fullName := wire.Name + "." + pin.Name
			s.index.pinByName[fullName] = pin
		}
	}
}

func (s *Storage) saveNodeUnsafe() error {
	if s.node == nil {
		return nil
	}

	data, err := encodeNode(s.node)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte("node"), data)
	})
}

// --- 配置导入/导出 ---

// ExportConfig 导出节点配置为 NSON 字节
func (s *Storage) ExportConfig() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.node == nil {
		return nil, fmt.Errorf("node not initialized")
	}

	return encodeNode(s.node)
}

// ImportConfig 从 NSON 字节导入节点配置
func (s *Storage) ImportConfig(ctx context.Context, data []byte) error {
	node, err := decodeNode(data)
	if err != nil {
		return fmt.Errorf("decode node: %w", err)
	}

	return s.SetNode(ctx, node)
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

// --- 辅助方法 ---

// ParsePinName 解析 Pin 名称，支持 "wire.pin" 格式
func ParsePinName(name string) (wireName, pinName string, ok bool) {
	parts := strings.Split(name, ".")
	if len(parts) == 2 {
		return parts[0], parts[1], true
	}
	return "", name, false
}

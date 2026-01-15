// Package client 提供消息持久化存储
package client

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/snple/beacon/packet"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// StoreConfig 存储配置
type StoreConfig struct {
	// 存储路径（空字符串表示使用内存模式）
	DataDir string

	// 是否启用持久化
	Enabled bool

	// 同步写入 (更安全但更慢)
	SyncWrites bool

	// 值日志文件大小 (MB)
	ValueLogFileSize int64

	// GC 间隔
	GCInterval time.Duration

	// 日志
	Logger *zap.Logger
}

// DefaultStoreConfig 返回默认存储配置
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		DataDir:          "",
		Enabled:          true,
		SyncWrites:       false, // 默认异步写入，性能更好
		ValueLogFileSize: 256,   // 256MB
		GCInterval:       5 * time.Minute,
		Logger:           nil,
	}
}

// Validate 验证存储配置的有效性
func (c *StoreConfig) Validate() error {
	if !c.Enabled {
		return nil // 未启用存储，无需验证
	}

	// DataDir 为空时使用 InMemory 模式，不需要验证

	if c.ValueLogFileSize <= 0 {
		return fmt.Errorf("valueLogFileSize must be > 0, got %d", c.ValueLogFileSize)
	}

	if c.GCInterval < 0 {
		return fmt.Errorf("gcInterval must be >= 0, got %v", c.GCInterval)
	}

	return nil
}

// StoredMessage 持久化存储的消息格式
type StoredMessage struct {
	// 消息标识
	PacketID uint16 `nson:"packet_id"`

	// 基础消息属性
	Topic    string          `nson:"topic"`
	Payload  []byte          `nson:"payload,omitempty"`
	QoS      packet.QoS      `nson:"qos"`
	Retain   bool            `nson:"retain"`
	Priority packet.Priority `nson:"priority"`

	// 消息元数据
	TraceID        string            `nson:"trace_id,omitempty"`
	ContentType    string            `nson:"content_type,omitempty"`
	UserProperties map[string]string `nson:"user_properties,omitempty"`

	// 时间相关
	ExpiryTime    int64 `nson:"expiry_time"`    // 消息过期时间戳（Unix 秒），0 表示不过期
	EnqueueTime   int64 `nson:"enqueue_time"`   // 入队时间戳（Unix 秒）
	LastSentTime  int64 `nson:"last_sent_time"` // 最后发送时间戳（Unix 秒）
	DeliveryCount int   `nson:"delivery_count"` // 投递次数

	// 请求-响应模式属性
	TargetClientID  string `nson:"target_client_id,omitempty"`
	ResponseTopic   string `nson:"response_topic,omitempty"`
	CorrelationData []byte `nson:"correlation_data,omitempty"`
}

// MessageStore 消息持久化存储
// 用于存储 QoS=1 的发送消息，确保客户端重连后消息不丢失
type MessageStore struct {
	db     *badger.DB
	config StoreConfig
	logger *zap.Logger

	// 生命周期
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// NewMessageStore 创建消息存储
func NewMessageStore(config StoreConfig) (*MessageStore, error) {
	if !config.Enabled {
		return nil, nil
	}

	if config.Logger == nil {
		config.Logger, _ = zap.NewDevelopment()
	}

	// 配置 badger options
	var opts badger.Options
	if config.DataDir == "" {
		// InMemory 模式
		opts = badger.DefaultOptions("").WithInMemory(true)
		config.Logger.Info("Client message store using InMemory mode")
	} else {
		// 持久化模式
		opts = badger.DefaultOptions(config.DataDir)
	}

	opts.Logger = nil // 禁用 badger 内置日志
	opts.SyncWrites = config.SyncWrites
	if config.ValueLogFileSize > 0 {
		opts.ValueLogFileSize = config.ValueLogFileSize << 20 // 转换为字节
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	ms := &MessageStore{
		db:      db,
		config:  config,
		logger:  config.Logger,
		closeCh: make(chan struct{}),
	}

	// 启动 GC 协程
	if config.GCInterval > 0 {
		ms.wg.Add(1)
		go ms.gcLoop()
	}

	if config.DataDir == "" {
		config.Logger.Info("Client message store initialized (InMemory mode)",
			zap.Bool("syncWrites", config.SyncWrites))
	} else {
		config.Logger.Info("Client message store initialized",
			zap.String("dataDir", config.DataDir),
			zap.Bool("syncWrites", config.SyncWrites))
	}

	return ms, nil
}

// Close 关闭存储
func (ms *MessageStore) Close() error {
	close(ms.closeCh)
	ms.wg.Wait()

	if ms.db != nil {
		return ms.db.Close()
	}
	return nil
}

// gcLoop 定期运行 GC
func (ms *MessageStore) gcLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ms.closeCh:
			return
		case <-ticker.C:
			ms.runGC()
		}
	}
}

// runGC 执行垃圾回收
func (ms *MessageStore) runGC() {
	for {
		err := ms.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}
}

// messageKey 生成消息存储 key
// 格式: outgoing:{packetID}
func messageKey(packetID uint16) []byte {
	return fmt.Appendf(nil, "outgoing:%d", packetID)
}

// outgoingPrefix 返回所有发送消息的前缀
func outgoingPrefix() []byte {
	return []byte("outgoing:")
}

// Save 保存消息到存储
func (ms *MessageStore) Save(msg *StoredMessage) error {
	key := messageKey(msg.PacketID)

	// 序列化消息
	data, err := encodeStoredMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	err = ms.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, data)
		// 如果消息有过期时间，设置 TTL
		if msg.ExpiryTime > 0 {
			ttl := time.Until(time.Unix(msg.ExpiryTime, 0))
			if ttl > 0 {
				entry = entry.WithTTL(ttl)
			}
		}

		return txn.SetEntry(entry)
	})

	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}

	ms.logger.Debug("Message saved",
		zap.Uint16("packetID", msg.PacketID),
		zap.String("topic", msg.Topic))

	return nil
}

// Delete 删除消息 (ACK 后调用)
func (ms *MessageStore) Delete(packetID uint16) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := messageKey(packetID)
		return txn.Delete(key)
	})
}

// Get 获取指定的消息
func (ms *MessageStore) Get(packetID uint16) (*StoredMessage, error) {
	var msg *StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(messageKey(packetID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			msg, err = decodeStoredMessage(val)
			return err
		})
	})

	return msg, err
}

// GetAll 获取所有待发送的消息
func (ms *MessageStore) GetAll() ([]*StoredMessage, error) {
	var messages []*StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := outgoingPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				msg, err := decodeStoredMessage(val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.ExpiryTime > 0 && time.Now().Unix() > msg.ExpiryTime {
					return nil // 跳过过期消息
				}

				messages = append(messages, msg)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get all messages: %w", err)
	}

	return messages, nil
}

// GetBatch 获取一批待发送的消息
// limit: 最大获取数量
// excludePacketIDs: 需要排除的 PacketID（已在 pendingAck 中）
func (ms *MessageStore) GetBatch(limit int, excludePacketIDs map[uint16]bool) ([]*StoredMessage, error) {
	var messages []*StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := outgoingPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if len(messages) >= limit {
				break // 达到限制
			}

			item := it.Item()

			err := item.Value(func(val []byte) error {
				msg, err := decodeStoredMessage(val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.ExpiryTime > 0 && time.Now().Unix() > msg.ExpiryTime {
					return nil // 跳过过期消息
				}

				// 排除已在 pendingAck 中的消息
				if excludePacketIDs != nil && excludePacketIDs[msg.PacketID] {
					return nil
				}

				messages = append(messages, msg)
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get batch messages: %w", err)
	}

	return messages, nil
}

// UpdateLastSentTime 更新消息的最后发送时间和投递次数
func (ms *MessageStore) UpdateLastSentTime(packetID uint16) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := messageKey(packetID)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		var msg *StoredMessage
		err = item.Value(func(val []byte) error {
			msg, err = decodeStoredMessage(val)
			return err
		})
		if err != nil {
			return err
		}

		// 更新发送时间和投递次数
		msg.LastSentTime = time.Now().Unix()
		msg.DeliveryCount++

		// 重新编码
		data, err := encodeStoredMessage(msg)
		if err != nil {
			return err
		}

		entry := badger.NewEntry(key, data)
		if msg.ExpiryTime > 0 {
			ttl := time.Until(time.Unix(msg.ExpiryTime, 0))
			if ttl > 0 {
				entry = entry.WithTTL(ttl)
			}
		}

		return txn.SetEntry(entry)
	})
}

// Count 统计消息数量
func (ms *MessageStore) Count() (int, error) {
	count := 0

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := outgoingPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // 只计数，不需要值

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			count++
		}

		return nil
	})

	return count, err
}

// Clear 清空所有消息 (CleanSession=true 时调用)
func (ms *MessageStore) Clear() error {
	prefix := outgoingPrefix()
	var keysToDelete [][]byte

	// 第一步：收集所有需要删除的 key
	err := ms.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // 只需要 key

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)
			keysToDelete = append(keysToDelete, key)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	if len(keysToDelete) == 0 {
		return nil
	}

	// 第二步：批量删除
	err = ms.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete keys: %w", err)
	}

	ms.logger.Info("Cleared all persisted messages",
		zap.Int("count", len(keysToDelete)))

	return nil
}

// CleanupExpired 清理过期消息
func (ms *MessageStore) CleanupExpired() (int, error) {
	count := 0
	keysToDelete := make([][]byte, 0)

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := outgoingPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		now := time.Now().Unix()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				msg, err := decodeStoredMessage(val)
				if err != nil {
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.ExpiryTime > 0 && now > msg.ExpiryTime {
					keyCopy := make([]byte, len(item.Key()))
					copy(keyCopy, item.Key())
					keysToDelete = append(keysToDelete, keyCopy)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// 批量删除过期消息
	if len(keysToDelete) > 0 {
		err = ms.db.Update(func(txn *badger.Txn) error {
			for _, key := range keysToDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
				count++
			}
			return nil
		})
	}

	return count, err
}

// Stats 存储统计信息
type StoreStats struct {
	TotalMessages int64
	StorageSize   int64
}

// GetStats 获取存储统计信息
func (ms *MessageStore) GetStats() (*StoreStats, error) {
	stats := &StoreStats{}

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := outgoingPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.PrefetchValues = false // 只计数，不需要值

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			stats.TotalMessages++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// 获取 LSM 树大小
	lsmSize, vlogSize := ms.db.Size()
	stats.StorageSize = lsmSize + vlogSize

	return stats, nil
}

// encodeStoredMessage 编码存储的消息（使用 nson）
func encodeStoredMessage(msg *StoredMessage) ([]byte, error) {
	// 使用 nson.Marshal 将结构体转为 Map
	m, err := nson.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("nson marshal failed: %w", err)
	}

	// 将 Map 编码为字节
	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return nil, fmt.Errorf("nson encode failed: %w", err)
	}

	return buf.Bytes(), nil
}

// decodeStoredMessage 解码存储的消息（使用 nson）
func decodeStoredMessage(data []byte) (*StoredMessage, error) {
	// 将字节解码为 Map
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return nil, fmt.Errorf("nson decode failed: %w", err)
	}

	// 将 Map 反序列化为结构体
	var msg StoredMessage
	if err := nson.Unmarshal(m, &msg); err != nil {
		return nil, fmt.Errorf("nson unmarshal failed: %w", err)
	}

	return &msg, nil
}

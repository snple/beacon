// Package client 提供消息持久化存储
package client

import (
	"bytes"
	"fmt"
	"sync"
	"time"

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
	PacketID nson.Id `nson:"pid"` // 数据包 ID

	// 原始 Message 包（视为只读）
	Message *Message `nson:"m"`
}

// messageStore 消息持久化存储
// 用于存储 QoS=1 的发送消息，确保客户端重连后消息不丢失
type messageStore struct {
	db     *badger.DB
	config StoreConfig
	logger *zap.Logger

	// 生命周期
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// newMessageStore 创建消息存储
func newMessageStore(config StoreConfig) (*messageStore, error) {
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

	ms := &messageStore{
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
func (ms *messageStore) Close() error {
	close(ms.closeCh)
	ms.wg.Wait()

	if ms.db != nil {
		return ms.db.Close()
	}
	return nil
}

// gcLoop 定期运行 GC
func (ms *messageStore) gcLoop() {
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
func (ms *messageStore) runGC() {
	for {
		err := ms.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}
}

// messageKey 生成消息存储 key
// 格式: msg:{packetID_hex}
func messageKey(packetID nson.Id) []byte {
	return fmt.Appendf(nil, "msg:%s", packetID.Hex())
}

// messagePrefix 返回所有发送消息的前缀
func messagePrefix() []byte {
	return []byte("msg:")
}

// Save 保存消息到存储
func (ms *messageStore) save(msg *Message) error {
	key := messageKey(msg.Packet.PacketID)

	stored := &StoredMessage{
		PacketID: msg.Packet.PacketID,
		Message:  msg,
	}

	// 序列化消息
	data, err := encodeStoredMessage(stored)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	err = ms.db.Update(func(txn *badger.Txn) error {
		entry := badger.NewEntry(key, data)
		// 如果消息有过期时间，设置 TTL
		if stored.Message.Packet.Properties != nil && stored.Message.Packet.Properties.ExpiryTime > 0 {
			ttl := time.Until(time.Unix(stored.Message.Packet.Properties.ExpiryTime, 0))
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
		zap.String("packetID", msg.Packet.PacketID.Hex()),
		zap.String("topic", msg.Packet.Topic))
	return nil
}

// Delete 删除消息 (ACK 后调用)
func (ms *messageStore) Delete(packetID nson.Id) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := messageKey(packetID)
		return txn.Delete(key)
	})
}

// Get 获取指定的消息
func (ms *messageStore) Get(packetID nson.Id) (*StoredMessage, error) {
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

// GetBatch 获取一批待发送的消息
// limit: 最大获取数量
// excludePacketIDs: 需要排除的 PacketID（已在 pendingAck 中）
func (ms *messageStore) GetBatch(limit int, excludePacketIDs map[nson.Id]bool) ([]*StoredMessage, error) {
	var messages []*StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := messagePrefix()
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
				if msg.Message.IsExpired() {
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

// Count 统计消息数量
func (ms *messageStore) Count() (int, error) {
	count := 0

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := messagePrefix()
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

// clear 清空所有消息 (CleanSession=true 时调用)
func (ms *messageStore) clear() error {
	prefix := messagePrefix()
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

// cleanupExpired 清理过期消息
func (ms *messageStore) cleanupExpired() (int, error) {
	count := 0
	keysToDelete := make([][]byte, 0)

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := messagePrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				msg, err := decodeStoredMessage(val)
				if err != nil {
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.Message.IsExpired() {
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
func (ms *messageStore) GetStats() (*StoreStats, error) {
	stats := &StoreStats{}

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := messagePrefix()
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

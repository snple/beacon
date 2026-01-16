// Package core 实现消息持久化存储
package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// StoredMessage 持久化存储的消息格式
type StoredMessage struct {
	// 消息标识
	ID       string `nson:"id"`  // 消息唯一标识
	ClientID string `nson:"cid"` // 客户端 ID
	PacketID uint16 `nson:"pid"` // 数据包 ID（冗余字段，便于索引/过滤）

	// 原始 Message 包（视为只读）
	Message *Message `nson:"m"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	// 存储路径
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

// DefaultStorageConfig 返回默认存储配置
func DefaultStorageConfig() StorageConfig {
	logger, _ := zap.NewDevelopment()
	return StorageConfig{
		DataDir:          "",
		Enabled:          true,
		SyncWrites:       false, // 默认异步写入，性能更好
		ValueLogFileSize: 1024,  // 1024MB
		GCInterval:       5 * time.Minute,
		Logger:           logger,
	}
}

// Validate 验证存储配置的有效性
func (c *StorageConfig) Validate() error {
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

// messageStore 消息持久化存储
// 用于存储 QoS=1 的消息，确保 core 重启后消息不丢失
type messageStore struct {
	db     *badger.DB
	config StorageConfig
	logger *zap.Logger

	// 生命周期
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// NewMessageStore 创建消息存储
func newMessageStore(config StorageConfig) (*messageStore, error) {
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
		config.Logger.Info("Message store using InMemory mode")
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
		config.Logger.Info("Message store initialized (InMemory mode)",
			zap.Bool("syncWrites", config.SyncWrites))
	} else {
		config.Logger.Info("Message store initialized",
			zap.String("dataDir", config.DataDir),
			zap.Bool("syncWrites", config.SyncWrites))
	}

	return ms, nil
}

// Close 关闭存储
func (ms *messageStore) close() error {
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
// 格式: msg:{clientID}:{packetID}
func messageKey(clientID string, packetID uint16) []byte {
	return fmt.Appendf(nil, "msg:%s:%d", clientID, packetID)
}

// messagePrefix 返回所有发送消息的前缀
func messagePrefix() []byte {
	return []byte("msg:")
}

// packetIDSeedKey 生成 PacketID 种子存储 key
func packetIDSeedKey(clientID string) []byte {
	return fmt.Appendf(nil, "meta:pid_seed:%s", clientID)
}

// retainKey 生成保留消息存储 key
func retainKey(topic string) []byte {
	return fmt.Appendf(nil, "retain:%s", topic)
}

// retainKeyPrefix 返回保留消息前缀
func retainKeyPrefix() []byte {
	return []byte("retain:")
}

func encodePacketIDSeed(seed uint16) []byte {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], seed)
	return b[:]
}

func decodePacketIDSeed(b []byte) (uint16, bool) {
	if len(b) != 2 {
		return 0, false
	}
	return binary.BigEndian.Uint16(b), true
}

// PacketIDSeed returns the stored PacketID seed for a client.
// It is used to resume outbound PacketID allocation after restart without scanning the whole store.
func (ms *messageStore) packetIDSeed(clientID string) (uint16, error) {
	if ms == nil || ms.db == nil {
		return uint16(minPacketID), nil
	}

	var seed = uint16(minPacketID)
	err := ms.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(packetIDSeedKey(clientID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}
		return item.Value(func(val []byte) error {
			if s, ok := decodePacketIDSeed(val); ok {
				seed = s
			}
			return nil
		})
	})
	return seed, err
}

// SetPacketIDSeed stores the last allocated PacketID for a client (best-effort).
func (ms *messageStore) setPacketIDSeed(clientID string, seed uint16) error {
	if ms == nil || ms.db == nil {
		return nil
	}
	return ms.db.Update(func(txn *badger.Txn) error {
		return txn.Set(packetIDSeedKey(clientID), encodePacketIDSeed(seed))
	})
}

// clientPrefix 生成客户端消息前缀
func clientPrefix(clientID string) []byte {
	return fmt.Appendf(nil, "msg:%s:", clientID)
}

// Save 保存一条“面向某个 client 的出站投递消息”（QoS1）。
//
// 注意：
// - baseMsg 视为只读共享内容
// - packetID/qos 是投递层字段，会写入存储的 packet 副本
func (ms *messageStore) save(clientID string, msg *Message) error {
	if msg == nil || msg.PacketID == 0 || msg.Packet == nil {
		return fmt.Errorf("nil message")
	}

	key := messageKey(clientID, msg.PacketID)

	stored := &StoredMessage{
		ID:       fmt.Sprintf("msg:%s:%d", clientID, msg.PacketID),
		ClientID: clientID,
		PacketID: msg.PacketID,

		Message: msg,
	}

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
		zap.String("clientID", clientID),
		zap.String("msgID", stored.ID),
		zap.String("topic", msg.Packet.Topic))

	return nil
}

// CountMessages 统计客户端消息数量
func (ms *messageStore) countMessages(clientID string) (int, error) {
	count := 0

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := clientPrefix(clientID)
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

// Delete 删除消息 (客户端 ACK 后调用)
func (ms *messageStore) delete(clientID string, packetID uint16) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := []byte(messageKey(clientID, packetID))
		return txn.Delete(key)
	})
}

// DeleteAllForClient 删除客户端所有消息 (CleanSession=true 时调用)
func (ms *messageStore) deleteAllForClient(clientID string) error {
	prefix := clientPrefix(clientID)
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

	ms.logger.Info("Cleared persisted messages for client (CleanSession)",
		zap.String("clientID", clientID),
		zap.Int("count", len(keysToDelete)))

	return nil
}

// getPendingMessagesBatch 获取客户端待投递消息（限制数量）
// limit: 最大获取数量
// excludeIDs: 需要排除的消息 ID（已在 pendingAck 中）
func (ms *messageStore) getPendingMessagesBatch(clientID string, limit int, excludeIDs map[uint16]bool) ([]StoredMessage, error) {
	var messages []StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := clientPrefix(clientID)
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
				var msg StoredMessage

				err := decodeStoredMessage(&msg, val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.Message.IsExpired() {
					return nil
				}

				// 排除已在 pendingAck 中的消息
				if excludeIDs != nil && excludeIDs[msg.PacketID] {
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
		return nil, fmt.Errorf("failed to get pending messages batch: %w", err)
	}

	return messages, nil
}

// Stats 存储统计信息
type StorageStats struct {
	TotalMessages   int64
	PendingMessages int64
	StorageSize     int64
}

// getStats 获取存储统计信息
func (ms *messageStore) getStats() (*StorageStats, error) {
	stats := &StorageStats{}

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

// saveRetainMessage 保存保留消息
func (ms *messageStore) saveRetainMessage(topic string, msg *Message) error {
	if msg == nil || msg.Packet == nil || len(msg.Packet.Payload) == 0 {
		return ErrInvalidRetainMessage
	}

	stored := &StoredMessage{
		ID:       topic,
		ClientID: "",
		PacketID: 0,

		Message: msg,
	}

	data, err := encodeStoredMessage(stored)
	if err != nil {
		return err
	}

	return ms.db.Update(func(txn *badger.Txn) error {
		key := retainKey(topic)
		entry := badger.NewEntry(key, data)

		if stored.Message.Packet != nil && stored.Message.Packet.Properties != nil && stored.Message.Packet.Properties.ExpiryTime > 0 {
			ttl := time.Until(time.Unix(stored.Message.Packet.Properties.ExpiryTime, 0))
			if ttl > 0 {
				entry = entry.WithTTL(ttl)
			}
		}

		return txn.SetEntry(entry)
	})
}

// deleteRetainMessage 删除保留消息
func (ms *messageStore) deleteRetainMessage(topic string) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := retainKey(topic)
		return txn.Delete(key)
	})
}

// getRetainMessages 获取匹配主题的保留消息
func (ms *messageStore) getRetainMessages(topicFilter string) ([]StoredMessage, error) {
	var messages []StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := retainKeyPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var msg StoredMessage

				err := decodeStoredMessage(&msg, val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
				}

				// 检查主题是否匹配 (简化的通配符匹配)
				if msg.Message.Packet != nil && topicMatches(msg.Message.Packet.Topic, topicFilter) {
					messages = append(messages, msg)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return messages, err
}

// getAllRetainMessages 获取所有保留消息
func (ms *messageStore) getAllRetainMessages() ([]StoredMessage, error) {
	var messages []StoredMessage

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := retainKeyPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var msg StoredMessage
				err := decodeStoredMessage(&msg, val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
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

	return messages, err
}

// topicMatches 检查主题是否匹配过滤器
// 支持通配符: + (单层通配符) 和 # (多层通配符)
func topicMatches(topic, filter string) bool {
	return matchTopic(filter, topic)
}

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

func decodeStoredMessage(msg *StoredMessage, data []byte) error {
	// 将字节解码为 Map
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return fmt.Errorf("nson decode failed: %w", err)
	}

	// 将 Map 反序列化为结构体
	if err := nson.Unmarshal(m, &msg); err != nil {
		return fmt.Errorf("nson unmarshal failed: %w", err)
	}

	return nil
}

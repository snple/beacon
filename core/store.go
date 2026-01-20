// Package core 实现消息持久化存储
package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"go.uber.org/zap"
)

// StoreOptions 存储配置
type StoreOptions struct {
	// 存储路径
	DataDir string

	// 同步写入 (更安全但更慢)
	SyncWrites bool

	// 值日志文件大小 (MB)
	ValueLogFileSize int64

	// GC 间隔
	GCInterval time.Duration

	// 日志
	Logger *zap.Logger
}

// DefaultStoreOptions 返回默认存储配置
func DefaultStoreOptions() StoreOptions {
	logger, _ := zap.NewDevelopment()
	return StoreOptions{
		DataDir:          "",
		SyncWrites:       false, // 默认异步写入，性能更好
		ValueLogFileSize: 1024,  // 1024MB
		GCInterval:       5 * time.Minute,
		Logger:           logger,
	}
}

// Validate 验证存储配置的有效性
func (c *StoreOptions) Validate() error {
	// DataDir 为空时使用 InMemory 模式，不需要验证

	if c.ValueLogFileSize <= 0 {
		return fmt.Errorf("valueLogFileSize must be > 0, got %d", c.ValueLogFileSize)
	}

	if c.GCInterval < 0 {
		return fmt.Errorf("gcInterval must be >= 0, got %v", c.GCInterval)
	}

	return nil
}

// store 消息持久化存储
// 用于存储 QoS=1 的消息，确保 core 重启后消息不丢失
type store struct {
	options StoreOptions

	db     *badger.DB
	logger *zap.Logger

	// 生命周期
	closeCh chan struct{}
	wg      sync.WaitGroup
}

// newStore 创建消息存储
func newStore(options StoreOptions) (*store, error) {
	if options.Logger == nil {
		options.Logger, _ = zap.NewDevelopment()
	}

	// 配置 badger options
	var opts badger.Options
	if options.DataDir == "" {
		// InMemory 模式
		opts = badger.DefaultOptions("").WithInMemory(true)
		options.Logger.Info("Message store using InMemory mode")
	} else {
		// 持久化模式
		opts = badger.DefaultOptions(options.DataDir)
	}

	opts.Logger = nil // 禁用 badger 内置日志
	opts.SyncWrites = options.SyncWrites
	if options.ValueLogFileSize > 0 {
		opts.ValueLogFileSize = options.ValueLogFileSize << 20 // 转换为字节
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	ms := &store{
		options: options,

		db:      db,
		logger:  options.Logger,
		closeCh: make(chan struct{}),
	}

	// 启动 GC 协程
	if options.GCInterval > 0 {
		ms.wg.Add(1)
		go ms.gcLoop()
	}

	if options.DataDir == "" {
		options.Logger.Info("Message store initialized (InMemory mode)",
			zap.Bool("syncWrites", options.SyncWrites))
	} else {
		options.Logger.Info("Message store initialized",
			zap.String("dataDir", options.DataDir),
			zap.Bool("syncWrites", options.SyncWrites))
	}

	return ms, nil
}

// Close 关闭存储
func (ms *store) close() error {
	close(ms.closeCh)
	ms.wg.Wait()

	if ms.db != nil {
		return ms.db.Close()
	}
	return nil
}

// gcLoop 定期运行 GC
func (ms *store) gcLoop() {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.options.GCInterval)
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
func (ms *store) runGC() {
	for {
		err := ms.db.RunValueLogGC(0.5)
		if err != nil {
			break
		}
	}
}

// retainKey 生成保留消息存储 key
func retainKey(topic string) []byte {
	return fmt.Appendf(nil, "retain:%s", topic)
}

// retainKeyPrefix 返回保留消息前缀
func retainKeyPrefix() []byte {
	return []byte("retain:")
}

// setRetain 保存保留消息
func (ms *store) setRetain(topic string, msg *Message) error {
	if msg == nil || msg.Packet == nil || len(msg.Packet.Payload) == 0 {
		return ErrInvalidRetainMessage
	}

	data, err := encodeMessage(msg)
	if err != nil {
		return err
	}

	return ms.db.Update(func(txn *badger.Txn) error {
		key := retainKey(topic)
		entry := badger.NewEntry(key, data)

		if msg.Packet != nil && msg.Packet.Properties != nil && msg.Packet.Properties.ExpiryTime > 0 {
			ttl := time.Until(time.Unix(msg.Packet.Properties.ExpiryTime, 0))
			if ttl > 0 {
				entry = entry.WithTTL(ttl)
			}
		}

		return txn.SetEntry(entry)
	})
}

// deleteRetain 删除保留消息
func (ms *store) deleteRetain(topic string) error {
	return ms.db.Update(func(txn *badger.Txn) error {
		key := retainKey(topic)
		return txn.Delete(key)
	})
}

// getRetain 获取单个保留消息
func (ms *store) getRetain(topic string) (*Message, error) {
	var msg *Message

	err := ms.db.View(func(txn *badger.Txn) error {
		key := retainKey(topic)
		item, err := txn.Get(key)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			var msgStored Message
			if err := decodeMessage(&msgStored, val); err != nil {
				return err
			}
			// 检查是否过期
			if msgStored.IsExpired() {
				return nil
			}
			msg = &msgStored
			return nil
		})
	})

	return msg, err
}

// iterateRetain 流式遍历保留消息索引，避免一次性加载所有消息到内存
// callback 返回 false 时停止遍历
func (ms *store) iterateRetain(callback func(msg *Message) bool) error {
	return ms.db.View(func(txn *badger.Txn) error {
		prefix := retainKeyPrefix()
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			var shouldContinue bool
			err := item.Value(func(val []byte) error {
				var msgStored Message
				err := decodeMessage(&msgStored, val)
				if err != nil {
					ms.logger.Warn("Failed to decode message during iteration",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					shouldContinue = true // 跳过损坏的消息，继续遍历
					return nil
				}

				if msgStored.Packet == nil {
					shouldContinue = true
					return nil
				}

				if msgStored.IsExpired() {
					shouldContinue = true
					return nil
				}

				shouldContinue = callback(&msgStored)
				return nil
			})
			if err != nil {
				return err
			}
			if !shouldContinue {
				break
			}
		}

		return nil
	})
}

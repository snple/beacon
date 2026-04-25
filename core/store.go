// Package core 实现消息持久化存储
package core

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	istore "github.com/snple/beacon/internal/store"
	"go.uber.org/zap"
)

// StoreOptions 存储配置（类型别名，保持向后兼容）
type StoreOptions = istore.Options

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

// store 消息持久化存储
// 内嵌共享的 Store，添加 core 特有的 retain 方法
type store struct {
	*istore.Store
	logger *zap.Logger
}

// newStore 创建消息存储
func newStore(options StoreOptions) (*store, error) {
	s, err := istore.New(options)
	if err != nil {
		return nil, err
	}

	return &store{
		Store:  s,
		logger: options.Logger,
	}, nil
}

// close 关闭存储
func (ms *store) close() error {
	return ms.Store.Close()
}

// db 返回底层数据库（兼容现有代码）
func (ms *store) db() *badger.DB {
	return ms.Store.DB()
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

	return ms.db().Update(func(txn *badger.Txn) error {
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
	return ms.db().Update(func(txn *badger.Txn) error {
		key := retainKey(topic)
		return txn.Delete(key)
	})
}

// getRetain 获取单个保留消息
func (ms *store) getRetain(topic string) (*Message, error) {
	var msg *Message

	err := ms.db().View(func(txn *badger.Txn) error {
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
	return ms.db().View(func(txn *badger.Txn) error {
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

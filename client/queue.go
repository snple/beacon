package client

import (
	"bytes"
	"fmt"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

// Queue 是基于 PacketID 的消息队列实现
// 使用 PacketID (nson.Id) 作为 key，Message 作为 value
// 支持按 PacketID 删除和 FIFO 遍历
type Queue struct {
	db     *badger.DB
	prefix []byte // 队列的键前缀，支持多队列
}

// NewQueue 创建一个新的基于 PacketID 的队列实例
// prefix: 队列的命名空间前缀，允许在同一个 DB 中创建多个队列
func NewQueue(db *badger.DB, prefix string) *Queue {
	return &Queue{
		db:     db,
		prefix: []byte(prefix),
	}
}

// Enqueue 将消息添加到队列
// 使用消息的 PacketID 作为 key
func (q *Queue) Enqueue(msg *Message) error {
	if msg == nil || msg.Packet.PacketID.IsZero() {
		return fmt.Errorf("invalid message or PacketID")
	}

	return q.db.Update(func(txn *badger.Txn) error {
		key := q.makeKey(msg.Packet.PacketID)

		// 序列化消息
		data, err := encodeMessage(msg)
		if err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}

		return txn.Set(key, data)
	})
}

// Dequeue 从队列头部取出消息并删除
// 使用迭代器获取最早的元素（PacketID 最小的）
func (q *Queue) Dequeue() (*Message, error) {
	msg := &Message{}

	err := q.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		// 定位到队列前缀的第一个元素
		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return ErrQueueEmpty
		}

		item := it.Item()
		key := item.KeyCopy(nil)

		// 读取数据
		var data []byte
		data, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// 反序列化消息
		err = decodeMessage(msg, data)
		if err != nil {
			return fmt.Errorf("failed to decode message: %w", err)
		}
		// 删除已读取的数据
		return txn.Delete(key)
	})

	return msg, err
}

// Peek 查看队列头部消息但不删除
func (q *Queue) Peek() (*Message, error) {
	msg := &Message{}

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return fmt.Errorf("queue is empty")
		}

		item := it.Item()

		var data []byte
		data, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		err = decodeMessage(msg, data)
		return err
	})

	return msg, err
}

// Delete 根据 PacketID 删除消息
func (q *Queue) Delete(packetID nson.Id) error {
	if packetID.IsZero() {
		return fmt.Errorf("invalid PacketID")
	}

	return q.db.Update(func(txn *badger.Txn) error {
		key := q.makeKey(packetID)
		return txn.Delete(key)
	})
}

// Get 根据 PacketID 获取消息（不删除）
func (q *Queue) Get(packetID nson.Id) (*Message, error) {
	if packetID.IsZero() {
		return nil, fmt.Errorf("invalid PacketID")
	}

	msg := &Message{}

	err := q.db.View(func(txn *badger.Txn) error {
		key := q.makeKey(packetID)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		var data []byte
		data, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		err = decodeMessage(msg, data)
		return err
	})

	return msg, err
}

// Size 返回队列中的元素数量
// 注意：这个操作需要遍历所有元素，性能 O(n)
func (q *Queue) Size() (uint64, error) {
	var size uint64

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 只需要 key，不需要 value
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(q.prefix); it.ValidForPrefix(q.prefix); it.Next() {
			size++
		}
		return nil
	})

	return size, err
}

// IsEmpty 检查队列是否为空
func (q *Queue) IsEmpty() (bool, error) {
	var empty = true

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)
		empty = !it.ValidForPrefix(q.prefix)

		return nil
	})

	return empty, err
}

// Clear 清空队列
func (q *Queue) Clear() error {
	return q.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		// 收集所有需要删除的 key
		var keys [][]byte
		for it.Seek(q.prefix); it.ValidForPrefix(q.prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keys = append(keys, key)
		}

		// 删除所有 key
		for _, key := range keys {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetFirstId 获取队列中第一个元素的 PacketID（用于测试和调试）
func (q *Queue) GetFirstId() (nson.Id, error) {
	var id nson.Id

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return fmt.Errorf("queue is empty")
		}

		key := it.Item().KeyCopy(nil)
		id = q.extractId(key)

		return nil
	})

	return id, err
}

// GetLastId 获取队列中最后一个元素的 PacketID（用于测试和调试）
func (q *Queue) GetLastId() (nson.Id, error) {
	var id nson.Id

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		opts.Reverse = true // 反向遍历
		it := txn.NewIterator(opts)
		defer it.Close()

		// 构造反向查找的起始 key（prefix + 最大可能值）
		seekKey := make([]byte, len(q.prefix)+1)
		copy(seekKey, q.prefix)
		seekKey[len(seekKey)-1] = 0xFF

		it.Seek(seekKey)

		// 检查是否在 prefix 范围内
		if !it.ValidForPrefix(q.prefix) {
			return fmt.Errorf("queue is empty")
		}

		key := it.Item().KeyCopy(nil)
		id = q.extractId(key)

		return nil
	})

	return id, err
}

// 辅助方法：生成队列元素的键
func (q *Queue) makeKey(id nson.Id) []byte {
	key := make([]byte, len(q.prefix)+len(id))
	copy(key, q.prefix)
	copy(key[len(q.prefix):], id[:])
	return key
}

// 辅助方法：从 key 中提取 PacketID
func (q *Queue) extractId(key []byte) nson.Id {
	if len(key) <= len(q.prefix) {
		return nson.Id{} // 返回空 Id
	}
	return nson.Id(key[len(q.prefix):])
}

// encodeMessage 序列化消息
func encodeMessage(msg *Message) ([]byte, error) {
	// 使用 nson.Marshal 将结构体转为 Map
	m, err := nson.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// 将 Map 编码为字节
	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// decodeMessage 反序列化消息
func decodeMessage(msg *Message, data []byte) error {
	// 将字节解码为 Map
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return err
	}

	// 将 Map 反序列化为结构体
	return nson.Unmarshal(m, msg)
}

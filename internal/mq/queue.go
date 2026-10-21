package mq

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

// 队列相关错误
var (
	// ErrQueueEmpty 队列为空
	ErrQueueEmpty = errors.New("queue is empty")

	// ErrCorruptedMessage 队列中的消息无法解码（数据损坏），已从队列中清除
	ErrCorruptedMessage = errors.New("queued message is corrupted")
)

// Queue 是基于 PacketID 的持久化消息队列（泛型）
// 使用 PacketID (nson.Id) 作为 key，T 作为 value，支持按 PacketID 删除和 FIFO 遍历。
//
// T 为消息的值类型（core.Message / client.Message），
// getID 用于提取消息的 PacketID：
//   - core：Message.PacketID
//   - client：Message.Packet.PacketID
//
// Enqueue 会拒绝零值 PacketID 的消息。
type Queue[T any] struct {
	db     *badger.DB
	prefix []byte          // 队列的键前缀，支持多队列
	getID  func(T) nson.Id // 提取消息 PacketID
}

// NewQueue 创建一个新的基于 PacketID 的队列实例
// db: badger 数据库实例
// prefix: 队列的命名空间前缀，允许在同一个 DB 中创建多个队列
// getID: 提取消息 PacketID（需 nil 安全）
func NewQueue[T any](db *badger.DB, prefix string, getID func(T) nson.Id) *Queue[T] {
	return &Queue[T]{
		db:     db,
		prefix: []byte(prefix),
		getID:  getID,
	}
}

// Enqueue 将消息添加到队列
// 使用消息的 PacketID 作为 key
func (q *Queue[T]) Enqueue(msg T) error {
	_, err := q.Put(msg)
	return err
}

// Put 将消息写入队列，并报告该 PacketID 是否是新增条目。
// 相同 PacketID 重复写入会覆盖旧值（此时 created=false），
// 供上层的队列计数/配额逻辑保持准确。
func (q *Queue[T]) Put(msg T) (created bool, err error) {
	id := q.getID(msg)
	if id.IsZero() {
		return false, fmt.Errorf("invalid message or PacketID")
	}

	err = q.db.Update(func(txn *badger.Txn) error {
		key := q.makeKey(id)

		// 序列化消息
		data, err := EncodeMessage(msg)
		if err != nil {
			return fmt.Errorf("failed to encode message: %w", err)
		}

		// 事务可能因冲突重试，因此每次尝试都显式重置结果
		_, getErr := txn.Get(key)
		switch {
		case getErr == nil:
			created = false // 已存在：覆盖
		case errors.Is(getErr, badger.ErrKeyNotFound):
			created = true
		default:
			return getErr
		}

		return txn.Set(key, data)
	})
	if err != nil {
		return false, err
	}
	return created, nil
}

// Dequeue 从队列头部取出消息并删除
// 使用迭代器获取最早的元素（PacketID 最小的）
// 若头部消息无法解码（数据损坏），会将其删除并返回 ErrCorruptedMessage，
// 以保证队列可以继续推进，避免单条坏消息永久阻塞后续消息。
func (q *Queue[T]) Dequeue() (T, error) {
	var zero T
	var result T

	// 记录损坏消息的 key，用于在事务外单独提交删除
	var corruptedKey []byte

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
		data, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// 反序列化消息
		var msg T
		if err := DecodeMessage(&msg, data); err != nil {
			// 注意：badger 在闭包返回错误时会回滚整个事务（包括上面的 txn.Delete），
			// 因此损坏消息的删除必须在事务外单独提交。
			corruptedKey = key
			return fmt.Errorf("%w: %v", ErrCorruptedMessage, err)
		}
		result = msg

		// 删除已读取的数据
		return txn.Delete(key)
	})

	if err != nil {
		if corruptedKey != nil {
			// 单独提交删除损坏的头部消息，让队列继续推进
			if delErr := q.db.Update(func(txn *badger.Txn) error {
				return txn.Delete(corruptedKey)
			}); delErr != nil {
				return zero, fmt.Errorf("failed to delete corrupted message: %w", delErr)
			}
		}
		return zero, err
	}

	return result, nil
}

// DeleteHead 删除队列头部消息
// 用于清除无法解码（损坏）的头部消息，防止其阻塞队列推进。
func (q *Queue[T]) DeleteHead() error {
	return q.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)
		if !it.ValidForPrefix(q.prefix) {
			return ErrQueueEmpty
		}

		return txn.Delete(it.Item().KeyCopy(nil))
	})
}

// PeekID 返回队列头部消息的 PacketID（不读取消息内容）。
// 用于低成本的"队头能否补货"判断，避免为每次 ACK 解码整条消息。
func (q *Queue[T]) PeekID() (nson.Id, bool, error) {
	var id nson.Id
	found := false

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)
		if !it.ValidForPrefix(q.prefix) {
			return nil
		}

		id = q.extractId(it.Item().KeyCopy(nil))
		found = true
		return nil
	})

	return id, found && err == nil, err
}

// Peek 查看队列头部消息但不删除
func (q *Queue[T]) Peek() (T, error) {
	var zero T
	var result T

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		opts.Prefix = q.prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return ErrQueueEmpty
		}

		item := it.Item()

		data, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// 反序列化消息；失败表示数据损坏，交由调用方决定是否清除
		var msg T
		if err := DecodeMessage(&msg, data); err != nil {
			return fmt.Errorf("%w: %v", ErrCorruptedMessage, err)
		}
		result = msg
		return nil
	})

	if err != nil {
		return zero, err
	}
	return result, nil
}

// Delete 根据 PacketID 删除消息
func (q *Queue[T]) Delete(packetID nson.Id) error {
	_, err := q.DeleteIfExists(packetID)
	return err
}

// DeleteIfExists 根据 PacketID 删除消息，并报告该条目是否真实存在。
// 供上层的队列计数/配额逻辑保持准确（重复删除不会误减计数）。
func (q *Queue[T]) DeleteIfExists(packetID nson.Id) (bool, error) {
	if packetID.IsZero() {
		return false, fmt.Errorf("invalid PacketID")
	}

	existed := false
	err := q.db.Update(func(txn *badger.Txn) error {
		key := q.makeKey(packetID)

		// 事务可能因冲突重试，因此每次尝试都显式重置结果
		existed = false
		_, getErr := txn.Get(key)
		switch {
		case getErr == nil:
			existed = true
		case errors.Is(getErr, badger.ErrKeyNotFound):
			return nil
		default:
			return getErr
		}

		return txn.Delete(key)
	})
	if err != nil {
		return false, err
	}
	return existed, nil
}

// Get 根据 PacketID 获取消息（不删除）
func (q *Queue[T]) Get(packetID nson.Id) (T, error) {
	var zero T
	var result T

	if packetID.IsZero() {
		return zero, fmt.Errorf("invalid PacketID")
	}

	err := q.db.View(func(txn *badger.Txn) error {
		key := q.makeKey(packetID)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		data, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		var msg T
		if err := DecodeMessage(&msg, data); err != nil {
			return err
		}
		result = msg
		return nil
	})

	if err != nil {
		return zero, err
	}
	return result, nil
}

// Size 返回队列中的元素数量
// 注意：这个操作需要遍历所有元素，性能 O(n)
func (q *Queue[T]) Size() (uint64, error) {
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
func (q *Queue[T]) IsEmpty() (bool, error) {
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

// Clear 清空队列（分批删除，避免大量消息时 OOM）
func (q *Queue[T]) Clear() error {
	const batchSize = 1000

	for {
		deleted := 0
		err := q.db.Update(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.PrefetchValues = false
			opts.Prefix = q.prefix
			it := txn.NewIterator(opts)
			defer it.Close()

			for it.Seek(q.prefix); it.ValidForPrefix(q.prefix) && deleted < batchSize; it.Next() {
				key := it.Item().KeyCopy(nil)
				if err := txn.Delete(key); err != nil {
					return err
				}
				deleted++
			}

			return nil
		})
		if err != nil {
			return err
		}
		if deleted < batchSize {
			return nil
		}
	}
}

// GetFirstId 获取队列中第一个元素的 PacketID（用于测试和调试）
func (q *Queue[T]) GetFirstId() (nson.Id, error) {
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
func (q *Queue[T]) GetLastId() (nson.Id, error) {
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
func (q *Queue[T]) makeKey(id nson.Id) []byte {
	key := make([]byte, len(q.prefix)+len(id))
	copy(key, q.prefix)
	copy(key[len(q.prefix):], id[:])
	return key
}

// 辅助方法：从 key 中提取 PacketID
func (q *Queue[T]) extractId(key []byte) nson.Id {
	if len(key) <= len(q.prefix) {
		return nson.Id{} // 返回空 Id
	}
	return nson.Id(key[len(q.prefix):])
}

// EncodeMessage 序列化消息
func EncodeMessage[T any](msg T) ([]byte, error) {
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

// DecodeMessage 反序列化消息到 msg 指向的对象
// msg 为指向消息值（struct）的指针（如 *core.Message / *client.Message）。
func DecodeMessage[T any](msg *T, data []byte) error {
	// 将字节解码为 Map
	buf := bytes.NewBuffer(data)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return err
	}

	// 将 Map 反序列化为结构体
	return nson.Unmarshal(m, msg)
}

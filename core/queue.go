package core

import (
	"fmt"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
)

// Queue 是基于 nson Id 的 FIFO 队列实现
// 使用 nson Id 作为 key，天然保证时间顺序和唯一性
// Id 结构：timestamp(6字节) + counter(2字节) + random(4字节)
type Queue struct {
	db     *badger.DB
	prefix []byte // 队列的键前缀，支持多队列
}

// NewQueue 创建一个新的基于 Id 的队列实例
// prefix: 队列的命名空间前缀，允许在同一个 DB 中创建多个队列
func NewQueue(db *badger.DB, prefix string) *Queue {
	return &Queue{
		db:     db,
		prefix: []byte(prefix),
	}
}

// Enqueue 将数据添加到队列尾部
// 使用 nson.NewId() 生成唯一且有序的 key
func (q *Queue) Enqueue(data []byte) error {
	return q.db.Update(func(txn *badger.Txn) error {
		// 生成唯一且递增的 Id 作为 key
		id := nson.NewId()
		key := q.makeKey(id)

		return txn.Set(key, data)
	})
}

// Dequeue 从队列头部取出数据并删除
// 使用迭代器获取最早的元素（Id 最小的）
func (q *Queue) Dequeue() ([]byte, error) {
	var data []byte

	err := q.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		// 定位到队列前缀的第一个元素
		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return fmt.Errorf("queue is empty")
		}

		item := it.Item()
		key := item.KeyCopy(nil)

		// 读取数据
		var err error
		data, err = item.ValueCopy(nil)
		if err != nil {
			return err
		}

		// 删除已读取的数据
		return txn.Delete(key)
	})

	return data, err
}

// Peek 查看队列头部数据但不删除
func (q *Queue) Peek() ([]byte, error) {
	var data []byte

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		it.Seek(q.prefix)

		if !it.ValidForPrefix(q.prefix) {
			return fmt.Errorf("queue is empty")
		}

		item := it.Item()

		var err error
		data, err = item.ValueCopy(nil)
		return err
	})

	return data, err
}

// Size 返回队列中的元素数量
// 注意：这个操作需要遍历所有元素，性能 O(n)
func (q *Queue) Size() (uint64, error) {
	var size uint64

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // 只需要 key，不需要 value
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

// GetFirstId 获取队列中第一个元素的 Id（用于测试和调试）
func (q *Queue) GetFirstId() (nson.Id, error) {
	var id nson.Id

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
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

// GetLastId 获取队列中最后一个元素的 Id（用于测试和调试）
func (q *Queue) GetLastId() (nson.Id, error) {
	var id nson.Id

	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
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

// 辅助方法：从 key 中提取 Id
func (q *Queue) extractId(key []byte) nson.Id {
	if len(key) <= len(q.prefix) {
		return nson.Id{} // 返回空 Id
	}
	return nson.Id(key[len(q.prefix):])
}

package core

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/internal/mq"
)

// Queue 是基于 PacketID 的消息队列实现
// 复用共享的 mq.Queue，并注入 core.Message 的 PacketID 提取器（coreMessagePacketIDValue）。
// 通过嵌入保持原有 API，避免改动大量调用点。
//
// 额外维护一个实时计数（Count），用于客户端队列配额判断，避免每次配额检查都做全量扫描。
type Queue struct {
	*mq.Queue[Message]
	count  atomic.Int64
	db     *badger.DB
	prefix []byte
}

// NewQueue 创建一个新的基于 PacketID 的队列实例
// prefix: 队列的命名空间前缀，允许在同一个 DB 中创建多个队列
func NewQueue(db *badger.DB, prefix string) *Queue {
	q := &Queue{
		Queue:  mq.NewQueue(db, prefix, coreMessagePacketIDValue),
		db:     db,
		prefix: []byte(prefix),
	}

	// 初始化计数（用于配额判断）。队列通常很小，键扫描的代价可接受。
	if n, err := q.Queue.Size(); err == nil {
		q.count.Store(int64(n))
	}

	return q
}

// coreMessagePacketIDValue 提取 core.Message 的 PacketID（值类型版本，供 mq.Queue 使用）
func coreMessagePacketIDValue(m Message) nson.Id {
	return m.PacketID
}

// Count 返回队列中的消息条数（随本包装器的写入/删除操作实时维护）。
func (q *Queue) Count() int64 {
	return max(q.count.Load(), 0)
}

// Enqueue 将消息添加到队列
// 使用消息的 PacketID 作为 key
func (q *Queue) Enqueue(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("invalid message or PacketID")
	}

	created, err := q.Queue.Put(*msg)
	if err != nil {
		return err
	}
	if created {
		q.count.Add(1)
	}
	return nil
}

// Dequeue 从队列头部取出消息并删除
func (q *Queue) Dequeue() (*Message, error) {
	msg, err := q.Queue.Dequeue()
	if err != nil {
		// 损坏的头部消息已被删除，计数需要同步
		if errors.Is(err, mq.ErrCorruptedMessage) {
			q.count.Add(-1)
		}
		return nil, err
	}
	q.count.Add(-1)
	return &msg, nil
}

// Peek 查看队列头部消息但不删除
func (q *Queue) Peek() (*Message, error) {
	msg, err := q.Queue.Peek()
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// Get 根据 PacketID 获取消息（不删除）
func (q *Queue) Get(packetID nson.Id) (*Message, error) {
	msg, err := q.Queue.Get(packetID)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// Delete 根据 PacketID 删除消息（条目不存在时不会影响计数）
func (q *Queue) Delete(packetID nson.Id) error {
	existed, err := q.Queue.DeleteIfExists(packetID)
	if err != nil {
		return err
	}
	if existed {
		q.count.Add(-1)
	}
	return nil
}

// DeleteHead 删除队列头部消息
func (q *Queue) DeleteHead() error {
	if err := q.Queue.DeleteHead(); err != nil {
		return err
	}
	q.count.Add(-1)
	return nil
}

// Clear 清空队列
func (q *Queue) Clear() error {
	if err := q.Queue.Clear(); err != nil {
		return err
	}
	q.count.Store(0)
	return nil
}

// Bytes 返回队列中消息编码后的总字节数（近似占用量）。
//
// 通过 badger 迭代读取 value 大小（不解码消息体），队列为空时零开销。
// 供管理面展示积压占用，不参与配额判断。
func (q *Queue) Bytes() int64 {
	if q.Count() <= 0 {
		return 0
	}

	var total int64
	err := q.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = q.prefix
		opts.PrefetchValues = false

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(q.prefix); it.ValidForPrefix(q.prefix); it.Next() {
			total += int64(it.Item().ValueSize())
		}
		return nil
	})
	if err != nil {
		return 0
	}
	return total
}

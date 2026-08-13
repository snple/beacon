package client

import (
	"fmt"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/internal/mq"
)

// Queue 是基于 PacketID 的消息队列实现
// 复用共享的 mq.Queue，并注入 client.Message 的 PacketID 提取器（clientMessagePacketIDValue）。
// 通过嵌入保持原有 API，避免改动大量调用点。
type Queue struct {
	*mq.Queue[Message]
}

// NewQueue 创建一个新的基于 PacketID 的队列实例
// prefix: 队列的命名空间前缀，允许在同一个 DB 中创建多个队列
func NewQueue(db *badger.DB, prefix string) *Queue {
	return &Queue{mq.NewQueue(db, prefix, clientMessagePacketIDValue)}
}

// clientMessagePacketIDValue 提取 client.Message 的 PacketID（值类型版本，供 mq.Queue 使用）
func clientMessagePacketIDValue(m Message) nson.Id {
	if m.Packet == nil {
		return nson.Id{}
	}
	return m.Packet.PacketID
}

// Enqueue 将消息添加到队列
// 使用消息的 PacketID 作为 key
func (q *Queue) Enqueue(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("invalid message or PacketID")
	}
	return q.Queue.Enqueue(*msg)
}

// Dequeue 从队列头部取出消息并删除
func (q *Queue) Dequeue() (*Message, error) {
	msg, err := q.Queue.Dequeue()
	if err != nil {
		return nil, err
	}
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

package client

import (
	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
)

// sendQueue 发送队列（用于流量控制 + PacketID 去重）
// 封装共享的 mq.SendQueue，并注入 client.Message 的 PacketID 提取器（clientMessagePacketID）。
// 通过薄门面保持原有小写 API，避免改动大量调用点。
type sendQueue struct {
	*mq.SendQueue[*Message]
}

// newSendQueue 创建发送队列
func newSendQueue(capacity int) *sendQueue {
	return &sendQueue{mq.NewSendQueue(capacity, clientMessagePacketID)}
}

func (q *sendQueue) tryEnqueue(msg *Message) error {
	return q.TryEnqueue(msg)
}

func (q *sendQueue) tryDequeue() (*Message, bool) {
	return q.TryDequeue()
}

func (q *sendQueue) finish(packetID packet.PacketID) {
	q.Finish(packetID)
}

func (q *sendQueue) available() int {
	return q.Available()
}

func (q *sendQueue) used() int32 {
	return q.Used()
}

// clientMessagePacketID 提取 client.Message 的 PacketID（nil 安全，供 mq.SendQueue 去重用）
func clientMessagePacketID(m *Message) packet.PacketID {
	if m == nil || m.Packet == nil {
		return packet.PacketID{}
	}
	return m.Packet.PacketID
}

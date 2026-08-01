package client

import (
	"context"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// setupRetransmitConn 构造用于测试重传的 Client+Conn（client.conn 已挂接）
func setupRetransmitConn(t *testing.T) (*Client, *Conn) {
	t.Helper()

	client := &Client{
		options: NewClientOptions(),
		logger:  zap.NewNop(),
	}
	conn := &Conn{
		client:     client,
		sendQueue:  newSendQueue(4),
		pendingAck: make(map[nson.Id]pendingMessage),
		ctx:        context.Background(),
		logger:     zap.NewNop(),
	}
	client.conn = conn
	return client, conn
}

// TestRetransmitUnackedMessages_ResendsOverdue 验证超过重传间隔未确认的 QoS1 消息被重发，
// 且重发时置 Dup 标志（与 core 侧对称）。
func TestRetransmitUnackedMessages_ResendsOverdue(t *testing.T) {
	client, conn := setupRetransmitConn(t)
	// 阻止 triggerSend 启动异步发送 goroutine，让消息保留在 sendQueue 供断言
	conn.processing.Store(true)

	msg := clientQueueTestMessage("test/topic", []byte("hello"), packet.QoS1)
	id := msg.Packet.PacketID
	conn.pendingAck[id] = pendingMessage{
		msg:        msg,
		lastSentAt: time.Now().Unix() - 100, // 远超 RetransmitInterval(5s)
	}

	sent := client.retransmitUnackedMessages()
	require.Equal(t, 1, sent)
	require.True(t, msg.Dup, "overdue message must be marked Dup for retransmit")

	// 消息应被重新放入发送队列
	got, ok := conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.Equal(t, id, got.Packet.PacketID)
	require.True(t, got.Dup)
}

// TestRetransmitUnackedMessages_SkipsRecent 验证未超过重传间隔的消息不被重发
func TestRetransmitUnackedMessages_SkipsRecent(t *testing.T) {
	client, conn := setupRetransmitConn(t)

	msg := clientQueueTestMessage("test/topic", []byte("hello"), packet.QoS1)
	conn.pendingAck[msg.Packet.PacketID] = pendingMessage{
		msg:        msg,
		lastSentAt: time.Now().Unix(), // 刚发送
	}

	sent := client.retransmitUnackedMessages()
	require.Equal(t, 0, sent)
	require.False(t, msg.Dup)
}

// TestRetransmitUnackedMessages_IgnoresNonPublishPending 验证 subscribe/unsubscribe
// 的 pending 条目（msg==nil）不会被当作发布消息重传
func TestRetransmitUnackedMessages_IgnoresNonPublishPending(t *testing.T) {
	client, conn := setupRetransmitConn(t)

	id := nson.NewId()
	conn.pendingAck[id] = pendingMessage{ch: make(chan error, 1)} // msg == nil

	sent := client.retransmitUnackedMessages()
	require.Equal(t, 0, sent)

	conn.pendingAckMu.Lock()
	_, exists := conn.pendingAck[id]
	conn.pendingAckMu.Unlock()
	require.True(t, exists, "non-publish pending entry must be preserved")
}

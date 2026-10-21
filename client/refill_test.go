package client

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestProcessSendQueue_RefillsFromPersistentQueue 验证客户端发送协程排空内存队列后会立即
// 从持久化队列补货，而不是等待 RetransmitInterval 定时器。
func TestProcessSendQueue_RefillsFromPersistentQueue(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	c := &Client{
		queue:   NewQueue(db, "client:refill:"),
		logger:  zap.NewNop(),
		options: NewClientOptions().WithRetransmitInterval(time.Hour),
		ctx:     context.Background(),
	}

	capture := &captureConn{}
	conn := &Conn{
		client:     c,
		conn:       capture,
		sendQueue:  newSendQueue(10),
		pendingAck: make(map[nson.Id]pendingMessage),
		ctx:        context.Background(),
		logger:     zap.NewNop(),
	}
	c.conn = conn

	const n = 5
	for i := 0; i < n; i++ {
		msg := clientQueueTestMessage("refill/topic", []byte{byte('0' + i)}, packet.QoS0)
		require.NoError(t, c.queue.Enqueue(msg))
	}
	require.Equal(t, int64(n), c.queue.Count())

	conn.processSendQueue()

	require.Equal(t, int64(0), c.queue.Count(),
		"writer should drain the persistent queue without waiting for the retransmit tick")

	reader := bytes.NewReader(capture.buf)
	count := 0
	for {
		if _, err := packet.ReadPacket(reader, 0); err != nil {
			break
		}
		count++
	}
	require.Equal(t, n, count, "all queued messages should be written")
}

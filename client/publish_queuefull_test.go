package client

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPublishMessage_QoS1QueueFullReturnsNilAfterPersist(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "client:test:")
	client := &Client{
		queue:   queue,
		logger:  zap.NewNop(),
		options: NewClientOptions(),
	}
	conn := &Conn{
		client:    client,
		sendQueue: newSendQueue(1),
		logger:    zap.NewNop(),
	}
	client.conn = conn

	filler := createClientTestMessage("test/filler", []byte("busy"), packet.QoS1)
	require.NoError(t, conn.sendQueue.tryEnqueue(filler))

	msg := Message{
		Packet:    packet.NewPublishPacket("test/topic", []byte("payload")),
		Timestamp: time.Now().Unix(),
	}
	msg.Packet.QoS = packet.QoS1

	err := client.publishMessage(msg, 50*time.Millisecond)
	require.NoError(t, err, "QoS1 publish should succeed once persisted even if send queue is temporarily full")

	size, err := queue.Size()
	require.NoError(t, err)
	require.Equal(t, uint64(1), size)
}

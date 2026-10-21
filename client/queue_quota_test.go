package client

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestPublish_OutboundQueueFullReturnsErrQueueFull 验证本地出站队列配额：
// 队列满时 Publish 明确返回 ErrQueueFull，而不是无限增长。
func TestPublish_OutboundQueueFullReturnsErrQueueFull(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	c := &Client{
		queue:   NewQueue(db, "client:quota:"),
		logger:  zap.NewNop(),
		options: NewClientOptions().WithMaxQueueSize(1),
	}

	require.NoError(t, c.Publish("test/topic", []byte("1"), NewPublishOptions().WithQoS(packet.QoS0)))

	err := c.Publish("test/topic", []byte("2"), NewPublishOptions().WithQoS(packet.QoS0))
	require.ErrorIs(t, err, ErrQueueFull)

	require.Equal(t, int64(1), c.queue.Count())
}

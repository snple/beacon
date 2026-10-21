package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// newQuotaTestClient 构造一个仅用于 deliver 配额测试的离线客户端。
func newQuotaTestClient(t *testing.T, c *Core, clientID string) *Client {
	t.Helper()

	connect := packet.NewConnectPacket()
	connect.ClientID = clientID
	connect.Properties.ReceiveWindow = 10

	client := &Client{
		ID:      clientID,
		core:    c,
		session: newSession(clientID, connect, c),
	}
	require.NoError(t, client.initQueue())
	return client
}

// TestDeliver_ClientQueueQuotaDropNewest 验证队列满时默认丢弃新消息并计数。
func TestDeliver_ClientQueueQuotaDropNewest(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStoreDir(t.TempDir()).
		WithMaxClientQueueSize(2, QueueOverflowDropNewest))
	require.NoError(t, err)
	defer c.store.close()

	client := newQuotaTestClient(t, c, "quota-drop-newest")

	require.NoError(t, client.deliver(createTestMessage("q/topic", []byte("0"), packet.QoS0)))
	require.NoError(t, client.deliver(createTestMessage("q/topic", []byte("1"), packet.QoS0)))

	err = client.deliver(createTestMessage("q/topic", []byte("2"), packet.QoS0))
	require.ErrorIs(t, err, ErrClientQueueFull)

	require.Equal(t, int64(2), client.queue.Count())
	require.Equal(t, int64(1), c.stats.MessagesDropped.Load())

	first, err := client.queue.Peek()
	require.NoError(t, err)
	require.Equal(t, "0", string(first.Packet.Payload), "drop-newest 应保留最早的两条消息")
}

// TestDeliver_ClientQueueQuotaDropOldest 验证 drop-oldest 策略保留最新消息。
func TestDeliver_ClientQueueQuotaDropOldest(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStoreDir(t.TempDir()).
		WithMaxClientQueueSize(2, QueueOverflowDropOldest))
	require.NoError(t, err)
	defer c.store.close()

	client := newQuotaTestClient(t, c, "quota-drop-oldest")

	for i, payload := range []string{"0", "1", "2"} {
		require.NoError(t, client.deliver(createTestMessage("q/topic", []byte(payload), packet.QoS0)), "deliver %d", i)
	}

	require.Equal(t, int64(2), client.queue.Count())
	require.Equal(t, int64(1), c.stats.MessagesDropped.Load())

	first, err := client.queue.Dequeue()
	require.NoError(t, err)
	require.Equal(t, "1", string(first.Packet.Payload))

	second, err := client.queue.Dequeue()
	require.NoError(t, err)
	require.Equal(t, "2", string(second.Packet.Payload))
}

// TestPublish_RejectsWhenPendingQueueQuotaExceeded 验证全局分发队列配额：
// 超限后 publish 返回 false（QoS1 发布端会收到 QuotaExceeded）。
func TestPublish_RejectsWhenPendingQueueQuotaExceeded(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStoreDir(t.TempDir()).
		WithMaxPendingMessages(1))
	require.NoError(t, err)
	defer c.store.close()

	first := c.newPublishMessage(packet.NewPublishPacket("quota/1", []byte("1")))
	require.True(t, c.publish(first))

	second := c.newPublishMessage(packet.NewPublishPacket("quota/2", []byte("2")))
	require.False(t, c.publish(second), "pending queue quota should reject the second message")

	require.Equal(t, int64(1), c.stats.MessagesRejected.Load())
}

// TestPublishRateLimit_PerClient 验证每客户端发布速率超限时 QoS1 返回 MessageRateTooHigh。
func TestPublishRateLimit_PerClient(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions().WithMaxPublishRate(1, 1))
	defer core.Stop()

	addr := testServe(t, core)

	publisher := testSetupClient(t, addr, "rate-limit-pub", nil)
	defer publisher.Close()

	opts := client.NewPublishOptions().WithQoS(packet.QoS1).WithTimeout(2 * time.Second)

	require.NoError(t, publisher.Publish("rate/topic", []byte("1"), opts))
	require.Error(t, publisher.Publish("rate/topic", []byte("2"), opts), "second publish should be rate limited")

	require.Equal(t, int64(1), core.GetStats().MessagesRejected)

	// 令牌按 rate 补充后应恢复
	time.Sleep(1100 * time.Millisecond)
	require.NoError(t, publisher.Publish("rate/topic", []byte("3"), opts))
}

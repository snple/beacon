package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// newQuotaTestClient 构造一个仅用于投递配额测试的离线客户端。
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
	client.initQueue()

	return client
}

// mustRegisterMessage 构造并注册一条共享消息（模拟 publish 的注册步骤）。
// 调用方持有 put 产生的那一个引用，需自行 release。
func mustRegisterMessage(t *testing.T, c *Core, topic string, payload []byte) *Message {
	t.Helper()

	msg, err := newMessage(packet.NewPublishPacket(topic, payload))
	require.NoError(t, err)
	c.msgs.put(msg)

	return msg
}

// TestEnqueueDelivery_ClientQueueQuotaDropNewest 验证队列满时默认丢弃新消息并计数。
func TestEnqueueDelivery_ClientQueueQuotaDropNewest(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithMaxClientQueueSize(2, QueueOverflowDropNewest))
	require.NoError(t, err)

	client := newQuotaTestClient(t, c, "quota-drop-newest")

	enqueue := func(payload string) error {
		msg := mustRegisterMessage(t, c, "q/topic", []byte(payload))
		defer c.msgs.release(msg)
		return client.enqueueDelivery(msg, msg.QoS, msg.Retain)
	}

	require.NoError(t, enqueue("0"))
	require.NoError(t, enqueue("1"))
	require.ErrorIs(t, enqueue("2"), ErrClientQueueFull)

	require.Equal(t, 2, client.queue.len())
	require.Equal(t, int64(1), c.stats.MessagesDropped.Load())

	first := client.queue.peek()
	require.NotNil(t, first)
	require.Equal(t, "0", string(first.msg.Packet.Payload), "drop-newest 应保留最早的两条消息")
}

// TestEnqueueDelivery_ClientQueueQuotaDropOldest 验证 drop-oldest 策略保留最新消息。
func TestEnqueueDelivery_ClientQueueQuotaDropOldest(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithMaxClientQueueSize(2, QueueOverflowDropOldest))
	require.NoError(t, err)

	client := newQuotaTestClient(t, c, "quota-drop-oldest")

	for _, payload := range []string{"0", "1", "2"} {
		msg := mustRegisterMessage(t, c, "q/topic", []byte(payload))
		require.NoError(t, client.enqueueDelivery(msg, msg.QoS, msg.Retain))
		c.msgs.release(msg)
	}

	require.Equal(t, 2, client.queue.len())
	require.Equal(t, int64(1), c.stats.MessagesDropped.Load())

	first, ok := client.queue.popFront()
	require.True(t, ok)
	require.Equal(t, "1", string(first.msg.Packet.Payload))

	second, ok := client.queue.popFront()
	require.True(t, ok)
	require.Equal(t, "2", string(second.msg.Packet.Payload))
}

// TestPublish_RejectsWhenPendingQueueQuotaExceeded 验证全局分发队列配额：
// 超限后 publish 返回 nil（QoS1 发布端会收到 QuotaExceeded）。
func TestPublish_RejectsWhenPendingQueueQuotaExceeded(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithMaxPendingMessages(1))
	require.NoError(t, err)

	require.NotNil(t, c.publish(packet.NewPublishPacket("quota/1", []byte("1"))))
	require.Nil(t, c.publish(packet.NewPublishPacket("quota/2", []byte("2"))),
		"pending queue quota should reject the second message")

	require.Equal(t, int64(1), c.stats.MessagesRejected.Load())
}

// TestPublish_RejectsWhenPendingBytesQuotaExceeded 验证内存水位配额：
// 水位被占满后新的发布会被拒绝。
func TestPublish_RejectsWhenPendingBytesQuotaExceeded(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithMaxPendingBytes(1))
	require.NoError(t, err)

	require.NotNil(t, c.publish(packet.NewPublishPacket("quota/bytes-1", []byte("1"))))
	require.Nil(t, c.publish(packet.NewPublishPacket("quota/bytes-2", []byte("2"))),
		"memory quota should reject further publishes")

	require.Equal(t, int64(1), c.stats.MessagesRejected.Load())
}

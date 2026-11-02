package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestCleanupExpiredPendingAckRemovesMessage 验证过期的未确认消息会被清理出待确认队列，
// 并计入 MessagesDropped。
func TestCleanupExpiredPendingAckRemovesMessage(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)
	require.NoError(t, c.Start())
	defer c.Stop()

	client := newQuotaTestClient(t, c, "expiry-client")
	c.clientsMu.Lock()
	c.clients[client.ID] = client
	c.clientsMu.Unlock()

	msg := mustRegisterMessage(t, c, "expiry/topic", []byte("v"))
	msg.Packet.Properties.ExpiryTime = time.Now().Add(-time.Minute).Unix()

	client.queue.pushBack(newDelivery(msg, packet.QoS0, false))

	d := newDelivery(msg, packet.QoS1, false)
	client.session.pendingAck.pushBack(d)

	before := c.stats.MessagesDropped.Load()
	c.cleanupExpiredMessages()

	require.Zero(t, client.queue.len(), "expired queued message must be removed")
	require.Zero(t, client.session.pendingAck.len(), "expired pending message must be removed")
	require.Equal(t, before+2, c.stats.MessagesDropped.Load())
}

// TestCleanupExpiredRetainedMessagesRemovesEntry 验证过期的保留消息会从保留存储中删除，
// 并释放其持有的消息引用。
func TestCleanupExpiredRetainedMessagesRemovesEntry(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithRetainEnabled(true))
	require.NoError(t, err)
	require.NoError(t, c.Start())
	defer c.Stop()

	const topic = "retain/expired"

	msg := mustRegisterMessage(t, c, topic, []byte("v"))
	msg.Packet.Properties.ExpiryTime = time.Now().Add(-time.Minute).Unix()
	msg.Retain = true

	c.msgs.retain(msg) // retainStore 持有的引用
	require.NoError(t, c.retainStore.set(topic, msg, msg.Packet.Properties.ExpiryTime))
	c.msgs.release(msg) // 归还发布路径的引用

	require.NotNil(t, c.retainStore.getEntry(topic), "retain entry should exist before cleanup")

	c.cleanupExpiredRetainMessages()

	require.Nil(t, c.retainStore.getEntry(topic), "expired retain entry must be removed")

	count, _ := c.msgs.stats()
	require.Zero(t, count, "expired retained message should release its reference")
}

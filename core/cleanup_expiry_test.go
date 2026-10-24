package core

import (
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestCleanupExpiredPendingAckRemovesMessage 验证过期的未确认消息会被清理出持久化队列，
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

	pub := packet.NewPublishPacket("expiry/topic", []byte("v"))
	pub.Properties.ExpiryTime = time.Now().Add(-time.Minute).Unix()

	msg := &Message{Packet: pub, PacketID: nson.NewId(), Timestamp: time.Now().Unix()}
	require.NoError(t, client.queue.Enqueue(msg))

	client.session.pendingAckMu.Lock()
	client.session.pendingAck[msg.PacketID] = pendingMessage{msg: msg}
	client.session.pendingAckMu.Unlock()

	before := c.stats.MessagesDropped.Load()
	c.cleanupExpiredMessages()

	require.Zero(t, client.queue.Count(), "expired pending message must be removed from the queue")

	client.session.pendingAckMu.Lock()
	require.Empty(t, client.session.pendingAck)
	client.session.pendingAckMu.Unlock()

	require.Equal(t, before+1, c.stats.MessagesDropped.Load())
}

// TestCleanupExpiredRetainedMessagesRemovesStoreEntry 验证过期的保留消息会同时从
// 索引与持久化存储中删除。
func TestCleanupExpiredRetainedMessagesRemovesStoreEntry(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithRetainEnabled(true))
	require.NoError(t, err)
	require.NoError(t, c.Start())
	defer c.Stop()

	const topic = "retain/expired"
	pub := packet.NewPublishPacket(topic, []byte("v"))
	msg := Message{Packet: pub, Timestamp: time.Now().Unix()}
	require.NoError(t, c.store.setRetain(topic, &msg))
	require.NoError(t, c.retainStore.set(topic, time.Now().Add(-time.Minute).Unix()))

	c.cleanupExpiredRetainMessages()

	require.Empty(t, c.retainStore.matchTopics("retain/**"), "expired retain index must be removed")

	stored, err := c.store.getRetain(topic)
	require.NoError(t, err)
	require.Nil(t, stored, "expired retained message must be deleted from the store")
}

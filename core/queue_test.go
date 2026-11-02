package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// createTestMessage 构造一条未注册的共享消息（纯队列单测使用）。
func createTestMessage(topic string, payload []byte, qos packet.QoS) *Message {
	pub := packet.NewPublishPacket(topic, payload)
	pub.QoS = qos

	msg, err := newMessage(pub)
	if err != nil {
		panic(err)
	}
	return msg
}

func TestPendingQueue_FIFO(t *testing.T) {
	q := newPendingQueue()
	require.Equal(t, 0, q.len())

	ids := []packet.PacketID{packet.NewPacketID(), packet.NewPacketID(), packet.NewPacketID()}
	for _, id := range ids {
		q.enqueue(id)
	}
	require.Equal(t, 3, q.len())

	for _, want := range ids {
		got, ok := q.dequeue()
		require.True(t, ok)
		require.Equal(t, want, got)
	}

	_, ok := q.dequeue()
	require.False(t, ok)
	require.Equal(t, 0, q.len())
}

func TestPendingQueue_ReusesCapacityAfterDrain(t *testing.T) {
	q := newPendingQueue()

	for range 100 {
		q.enqueue(packet.NewPacketID())
	}
	for q.len() > 0 {
		_, ok := q.dequeue()
		require.True(t, ok)
	}

	// 排空后再次入队仍然按 FIFO 工作
	id := packet.NewPacketID()
	q.enqueue(id)

	got, ok := q.dequeue()
	require.True(t, ok)
	require.Equal(t, id, got)
}

func TestPendingQueue_CompactsConsumedPrefix(t *testing.T) {
	q := newPendingQueue()
	for range 4 {
		q.enqueue(packet.NewPacketID())
	}

	for range 10_000 {
		_, ok := q.dequeue()
		require.True(t, ok)
		q.enqueue(packet.NewPacketID())
	}

	require.Equal(t, 4, q.len())
	require.Less(t, q.head, pendingQueueCompactThreshold)
	require.LessOrEqual(t, len(q.ids), 4+pendingQueueCompactThreshold)
}

func TestDeliveryQueue_FIFOAndRemove(t *testing.T) {
	store := newMessageStore()
	q := newDeliveryQueue(store)

	d1 := newDelivery(createTestMessage("q/1", []byte("1"), packet.QoS0), packet.QoS0, false)
	d2 := newDelivery(createTestMessage("q/2", []byte("2"), packet.QoS1), packet.QoS1, false)

	q.pushBack(d1)
	q.pushFront(d2)
	require.Equal(t, 2, q.len())
	require.Equal(t, d2, q.peek())

	got, ok := q.popFront()
	require.True(t, ok)
	require.Equal(t, d2, got)

	removed, ok := q.remove(d1.packetID)
	require.True(t, ok)
	require.Equal(t, d1, removed)
	require.Equal(t, 0, q.len())

	_, ok = q.remove(d1.packetID)
	require.False(t, ok)
}

func TestDeliveryQueue_DropExpired(t *testing.T) {
	store := newMessageStore()
	q := newDeliveryQueue(store)

	expired := createTestMessage("q/expired", []byte("x"), packet.QoS0)
	expired.Packet.Properties.ExpiryTime = 1 // 1970 年，必然过期
	live := createTestMessage("q/live", []byte("y"), packet.QoS0)

	q.pushBack(newDelivery(expired, packet.QoS0, false))
	q.pushBack(newDelivery(live, packet.QoS0, false))

	require.Equal(t, 1, q.dropExpired())
	require.Equal(t, 1, q.len())
	require.Equal(t, live, q.peek().msg)
}

func TestMessageStore_ReleasesWhenRefCountReachesZero(t *testing.T) {
	store := newMessageStore()

	msg := createTestMessage("store/topic", []byte("payload"), packet.QoS1)
	store.put(msg)

	count, bytes := store.stats()
	require.Equal(t, 1, count)
	require.Positive(t, bytes)

	// 额外持有者
	store.retain(msg)
	store.release(msg)

	count, _ = store.stats()
	require.Equal(t, 1, count, "still referenced by the publisher's reference")

	// take 转移队列持有的引用，release 归还
	require.Same(t, msg, store.take(msg.ID))
	store.release(msg)

	count, bytes = store.stats()
	require.Equal(t, 0, count)
	require.Equal(t, int64(0), bytes)
}

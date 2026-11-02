package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestDeliveryQueue_CountTracksMutations 验证队列长度随写入/删除/清空保持一致。
func TestDeliveryQueue_CountTracksMutations(t *testing.T) {
	store := newMessageStore()
	q := newDeliveryQueue(store)

	d1 := newDelivery(createTestMessage("q/1", []byte("1"), packet.QoS0), packet.QoS0, false)
	d2 := newDelivery(createTestMessage("q/2", []byte("2"), packet.QoS0), packet.QoS0, false)

	require.Equal(t, 0, q.len())

	q.pushBack(d1)
	q.pushBack(d2)
	require.Equal(t, 2, q.len())

	// 同一条投递（同一 PacketID）重复入队不会被去重：调用方保证每条投递只入队一次，
	// 这里只验证按 PacketID 删除的语义。
	require.True(t, q.drop(d1.packetID))
	require.Equal(t, 1, q.len())
	require.False(t, q.drop(d1.packetID))
	require.Equal(t, 1, q.len())

	require.True(t, q.dropOldest())
	require.Equal(t, 0, q.len())

	q.pushBack(d2)
	require.Equal(t, 1, q.len())
	q.close()
	require.Equal(t, 0, q.len())
}

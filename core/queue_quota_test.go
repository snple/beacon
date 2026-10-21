package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestQueue_CountTracksMutations 验证队列计数随写入/覆盖/删除/清空保持一致。
func TestQueue_CountTracksMutations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	q := NewQueue(db, "quota:")
	require.Equal(t, int64(0), q.Count())

	m1 := createTestMessage("q/1", []byte("1"), packet.QoS0)
	m2 := createTestMessage("q/2", []byte("2"), packet.QoS0)

	require.NoError(t, q.Enqueue(m1))
	require.NoError(t, q.Enqueue(m2))
	require.Equal(t, int64(2), q.Count())

	// 相同 PacketID 覆盖写入不应重复计数
	require.NoError(t, q.Enqueue(m1))
	require.Equal(t, int64(2), q.Count())

	// 删除存在/不存在的条目
	require.NoError(t, q.Delete(m1.PacketID))
	require.Equal(t, int64(1), q.Count())
	require.NoError(t, q.Delete(m1.PacketID))
	require.Equal(t, int64(1), q.Count())

	// DeleteHead
	require.NoError(t, q.DeleteHead())
	require.Equal(t, int64(0), q.Count())

	require.NoError(t, q.Enqueue(m1))
	require.Equal(t, int64(1), q.Count())
	require.NoError(t, q.Clear())
	require.Equal(t, int64(0), q.Count())
}

// TestQueue_CountSurvivesReopen 验证新建包装器时会从存储中恢复计数。
func TestQueue_CountSurvivesReopen(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	q := NewQueue(db, "quota-reopen:")
	require.NoError(t, q.Enqueue(createTestMessage("q/1", []byte("1"), packet.QoS0)))
	require.NoError(t, q.Enqueue(createTestMessage("q/2", []byte("2"), packet.QoS0)))

	reopened := NewQueue(db, "quota-reopen:")
	require.Equal(t, int64(2), reopened.Count())
}

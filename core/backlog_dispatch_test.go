package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestStart_DrainsPersistedDispatchBacklog 验证重启后全局分发队列中的积压
// 会被主动排空，而不是等到下一次发布才处理。
func TestStart_DrainsPersistedDispatchBacklog(t *testing.T) {
	dir := t.TempDir()

	c1, err := NewWithOptions(NewCoreOptions().WithStoreDir(dir))
	require.NoError(t, err)

	require.NoError(t, c1.queue.Enqueue(
		createTestMessage("backlog/topic", []byte("x"), packet.QoS0)))
	require.Equal(t, int64(1), c1.queue.Count())

	// 不调用 Start（没有后台协程），直接关闭存储模拟停机。
	require.NoError(t, c1.store.close())

	c2 := testSetupCore(t, NewCoreOptions().WithStoreDir(dir))
	defer c2.Stop()

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "persisted dispatch backlog drained", func() bool {
		return c2.queue.Count() == 0
	}), "persisted dispatch backlog should be drained on Start")
}

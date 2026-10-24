package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestRecoveredSessionKeepsPublishLimiter 验证进程重启后从磁盘恢复的离线会话
// 仍然带有发布限流器（MaxPublishRate 不能因重启而失效）。
func TestRecoveredSessionKeepsPublishLimiter(t *testing.T) {
	const (
		clientID  = "restore-rate"
		rateLimit = 5.0
	)

	dir := t.TempDir()

	first := testSetupCore(t, NewCoreOptions().
		WithStoreDir(dir).
		WithMaxPublishRate(rateLimit, 0))
	addr := testServe(t, first)

	cli := testSetupClient(t, addr, clientID, client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, cli.Close())

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "session becomes offline", func() bool {
		info, err := first.GetClientInfo(clientID)
		return err == nil && !info.Connected
	}))

	require.NoError(t, first.Stop())

	second, err := NewWithOptions(NewCoreOptions().
		WithStoreDir(dir).
		WithMaxPublishRate(rateLimit, 0).
		WithLogger(zap.NewNop()))
	require.NoError(t, err)
	require.NoError(t, second.Start())
	defer second.Stop()

	second.clientsMu.RLock()
	recovered := second.clients[clientID]
	second.clientsMu.RUnlock()

	require.NotNil(t, recovered, "offline session should be recovered from disk")
	require.NotNil(t, recovered.publishLimiter, "recovered session must keep the publish rate limiter")
}

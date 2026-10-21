package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestPublishToClient_QueuesForOfflineSession 验证定向投递与主题发布一致：
// 离线但保留会话的客户端会先入队，重连后收到消息。
func TestPublishToClient_QueuesForOfflineSession(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	addr := testServe(t, c)

	cl := testSetupClient(t, addr, "direct-offline", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, cl.Disconnect())
	require.NoError(t, cl.Close())

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "client offline session retained", func() bool {
		return c.GetStats().OfflineSessions == 1
	}))

	require.NoError(t, c.PublishToClient("direct-offline", "direct/offline", []byte("queued"), PublishOptions{
		QoS: packet.QoS1,
	}))

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "message queued for offline client", func() bool {
		info, err := c.GetClientInfo("direct-offline")
		return err == nil && info.QueueSize == 1
	}))

	// 重连后应恢复会话并收到积压消息
	cl2 := testSetupClient(t, addr, "direct-offline", client.NewClientOptions().WithSessionTimeout(60))
	defer cl2.Close()
	require.True(t, cl2.SessionPresent(), "session should be resumed")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	msg, err := cl2.PollMessage(ctx, 2*time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("queued"), msg.Payload())
}

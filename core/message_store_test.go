package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestMessageStore_ReleasedAfterAllDeliveriesAcked 验证端到端引用计数平衡：
// 一条消息投递给多个订阅者、全部 ACK 之后，messageStore 必须回到空（无泄漏）。
func TestMessageStore_ReleasedAfterAllDeliveriesAcked(t *testing.T) {
	core := testSetupCore(t, NewCoreOptions().WithRetransmitInterval(time.Second))
	defer core.Stop()

	addr := testServe(t, core)

	const subscribers = 3
	clients := make([]*client.Client, 0, subscribers)
	for i := range subscribers {
		cl := testSetupClient(t, addr, "leak-sub-"+string(rune('a'+i)),
			client.NewClientOptions().WithSessionTimeout(60))
		require.NoError(t, cl.SubscribeWithOptions([]string{"leak/topic"},
			client.NewSubscribeOptions().WithQoS(packet.QoS1)))
		clients = append(clients, cl)
	}
	defer func() {
		for _, cl := range clients {
			_ = cl.Close()
		}
	}()

	// 等订阅全部生效
	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "subscriptions registered", func() bool {
		return core.GetStats().SubscriptionsCount >= subscribers
	}))

	pub := testSetupClient(t, addr, "leak-pub", nil)
	defer pub.Close()

	require.NoError(t, pub.Publish("leak/topic", []byte("payload"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	// 所有订阅者都收到并（自动）确认后，共享消息应被回收
	require.True(t, eventually(t, 5*time.Second, 20*time.Millisecond, "message released after acks", func() bool {
		count, _ := core.msgs.stats()
		return count == 0
	}), "message content should be released once every delivery is acknowledged")

	// 每个订阅者都真的收到了消息
	for i, cl := range clients {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		msg, err := cl.PollMessage(ctx, 500*time.Millisecond)
		cancel()
		require.NoErrorf(t, err, "subscriber %d should receive the message", i)
		require.Equal(t, []byte("payload"), msg.Packet.Payload)
	}
}

// TestMessageStore_ReleasedAfterClientCleanup 验证客户端会话被清理时，
// 其待发送队列中的消息引用会被释放。
func TestMessageStore_ReleasedAfterClientCleanup(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	cl := newQuotaTestClient(t, c, "cleanup-leak")

	msg := mustRegisterMessage(t, c, "cleanup/topic", []byte("payload"))
	require.NoError(t, cl.enqueueDelivery(msg, packet.QoS0, false))
	c.msgs.release(msg) // 归还发布路径的引用，此时只剩队列持有

	count, _ := c.msgs.stats()
	require.Equal(t, 1, count)

	c.cleanupClient(cl.ID, cl)

	count, bytes := c.msgs.stats()
	require.Zero(t, count, "cleanup should release queued message references")
	require.Zero(t, bytes)
}

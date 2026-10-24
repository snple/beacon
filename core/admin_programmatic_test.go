package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestCoreSubscribeUnsubscribeProgrammatic 验证编程接口的订阅/取消订阅
// 会同步会话状态、持久化并参与主题匹配。
func TestCoreSubscribeUnsubscribeProgrammatic(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "api-sub", nil)
	defer cli.Close()

	c.Subscribe("api-sub", packet.Subscription{
		Topic:   "api/topic",
		Options: packet.SubscribeOptions{QoS: packet.QoS1},
	})

	info, err := c.GetClientInfo("api-sub")
	require.NoError(t, err)
	require.Len(t, info.Subscriptions, 1)
	require.Equal(t, "api/topic", info.Subscriptions[0].Topic)
	require.Equal(t, packet.QoS1, info.Subscriptions[0].QoS)

	res, err := c.Publish("api/topic", []byte("v"), PublishOptions{QoS: packet.QoS0})
	require.NoError(t, err)
	require.Equal(t, 1, res.MatchedSubscribers, "programmatic subscription should be matched")

	c.Unsubscribe("api-sub", "api/topic")

	info, err = c.GetClientInfo("api-sub")
	require.NoError(t, err)
	require.Empty(t, info.Subscriptions)

	res, err = c.Publish("api/topic", []byte("v"), PublishOptions{QoS: packet.QoS0})
	require.NoError(t, err)
	require.Zero(t, res.MatchedSubscribers)
}

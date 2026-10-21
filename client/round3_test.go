package client

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestPublishAfterCloseReturnsErrClientClosed 确认 Close 之后对外 API 返回
// ErrClientClosed，而不是底层存储关闭后的 badger 错误。
func TestPublishAfterCloseReturnsErrClientClosed(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	require.NoError(t, c.Close())

	require.ErrorIs(t, c.Publish("t", []byte("x"), nil), ErrClientClosed)
	require.ErrorIs(t, c.PublishToClient("other", "t", []byte("x"), nil), ErrClientClosed)
	require.ErrorIs(t, c.Subscribe("t"), ErrClientClosed)
	require.ErrorIs(t, c.Unsubscribe("t"), ErrClientClosed)
}

// TestPruneRejectedSubscriptionsKeepsGranted 确认 SUBACK 部分失败时只移除
// 被拒绝的订阅，服务端已授予的订阅记录必须保留（否则重连不会恢复它们）。
func TestPruneRejectedSubscriptionsKeepsGranted(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	defer c.Close()

	c.subscribedTopicsMu.Lock()
	c.subscribedTopics["granted"] = SubscribeOptions{QoS: packet.QoS1}
	c.subscribedTopics["denied"] = SubscribeOptions{QoS: packet.QoS1}
	c.subscribedTopicsMu.Unlock()

	subErr := &SubscriptionError{
		Index:  1,
		Reason: packet.ReasonNotAuthorized.String(),
		Codes:  []packet.ReasonCode{packet.ReasonGrantedQoS1, packet.ReasonNotAuthorized},
	}
	c.pruneRejectedSubscriptions([]string{"granted", "denied"}, subErr)

	require.True(t, c.HasSubscription("granted"))
	require.False(t, c.HasSubscription("denied"))

	// 非 SubscriptionError（例如连接层错误）无法判断结果，不应删除任何记录。
	c.pruneRejectedSubscriptions([]string{"granted"}, ErrNotConnected)
	require.True(t, c.HasSubscription("granted"))
}

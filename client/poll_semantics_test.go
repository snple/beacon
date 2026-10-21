package client

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestPollMessage_ReturnsBufferedMessageWhenDisconnected 验证断线后仍可取走已缓冲消息。
func TestPollMessage_ReturnsBufferedMessageWhenDisconnected(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	defer c.Close()

	c.messageQueue <- &Message{Packet: packet.NewPublishPacket("buffered/topic", []byte("x"))}
	c.connected.Store(false)

	msg, err := c.PollMessage(context.Background(), time.Second)
	require.NoError(t, err)
	require.Equal(t, "buffered/topic", msg.Topic())
}

// TestPollMessage_ZeroTimeoutBlocksUntilMessage 验证 timeout<=0 表示无限等待。
func TestPollMessage_ZeroTimeoutBlocksUntilMessage(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	defer c.Close()

	c.connected.Store(true)

	go func() {
		time.Sleep(50 * time.Millisecond)
		c.messageQueue <- &Message{Packet: packet.NewPublishPacket("later/topic", []byte("x"))}
	}()

	msg, err := c.PollMessage(context.Background(), 0)
	require.NoError(t, err)
	require.Equal(t, "later/topic", msg.Topic())
}

// TestPollMessage_ZeroTimeoutHonorsContext 验证无限等待仍可被 ctx 取消。
func TestPollMessage_ZeroTimeoutHonorsContext(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions())
	require.NoError(t, err)
	defer c.Close()

	c.connected.Store(true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = c.PollMessage(ctx, 0)
	require.ErrorIs(t, err, context.Canceled)
}

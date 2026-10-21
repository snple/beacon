package core

import (
	"context"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestStats_BytesCounters 验证收发字节计数不再恒为 0。
func TestStats_BytesCounters(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	addr := testServe(t, c)

	cl := testSetupClient(t, addr, "bytes-client", nil)
	defer cl.Close()

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "bytes counters updated", func() bool {
		s := c.GetStats()
		return s.BytesReceived > 0 && s.BytesSent > 0
	}), "CONNECT/CONNACK should be counted")

	before := c.GetStats()

	require.NoError(t, cl.Subscribe("bytes/topic"))
	require.NoError(t, cl.Publish("bytes/topic", []byte("hello"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := cl.PollMessage(ctx, 2*time.Second)
	require.NoError(t, err)

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "byte counters grow after traffic", func() bool {
		s := c.GetStats()
		return s.BytesReceived > before.BytesReceived && s.BytesSent > before.BytesSent
	}))
}

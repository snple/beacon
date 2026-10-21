package client

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestClientOptions_ValidateRejectsUnsafeValues 验证会导致 panic 或立即超时的配置被拒绝。
func TestClientOptions_ValidateRejectsUnsafeValues(t *testing.T) {
	cases := []struct {
		name string
		opts *ClientOptions
	}{
		{"negative retransmit interval", NewClientOptions().WithRetransmitInterval(-time.Second)},
		{"negative message queue size", NewClientOptions().WithMessageQueueSize(-1)},
		{"negative connect timeout", NewClientOptions().WithConnectTimeout(-time.Second)},
		{"negative publish timeout", NewClientOptions().WithPublishTimeout(-time.Second)},
		{"negative write timeout", NewClientOptions().WithWriteTimeout(-time.Second)},
		{"negative max queue size", NewClientOptions().WithMaxQueueSize(-1)},
		{"oversized max packet", NewClientOptions().WithMaxPacketSize(packet.MaxPacketSize + 1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewWithOptions(tc.opts)
			require.Error(t, err)
		})
	}
}

// TestClientOptions_DefaultsAreValid 验证默认值与零值配置都能通过校验。
func TestClientOptions_DefaultsAreValid(t *testing.T) {
	require.NoError(t, NewClientOptions().Validate())

	c, err := NewWithOptions(&ClientOptions{})
	require.NoError(t, err)
	require.NoError(t, c.Close())
}

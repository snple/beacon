package core

import (
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func TestMessageCopy_ReusesReadOnlyPacket(t *testing.T) {
	original := &Message{
		Packet: &packet.PublishPacket{
			Topic:      "copy/test",
			Payload:    []byte("payload"),
			Properties: packet.NewPublishProperties(),
		},
		Dup:       true,
		QoS:       packet.QoS1,
		Retain:    true,
		PacketID:  nson.NewId(),
		Timestamp: time.Now().Unix(),
	}

	cloned := original.Copy()

	require.Equal(t, original.Dup, cloned.Dup)
	require.Equal(t, original.QoS, cloned.QoS)
	require.Equal(t, original.Retain, cloned.Retain)
	require.Equal(t, original.PacketID, cloned.PacketID)
	require.Same(t, original.Packet, cloned.Packet, "message copy should reuse the read-only packet payload")

	cloned.PacketID = nson.NewId()
	cloned.QoS = packet.QoS0
	cloned.Retain = false
	cloned.Dup = false

	require.NotEqual(t, original.PacketID, cloned.PacketID)
	require.Equal(t, packet.QoS1, original.QoS)
	require.True(t, original.Retain)
	require.True(t, original.Dup)
	if original.Packet != nil {
		require.Equal(t, "copy/test", original.Packet.Topic)
	}
}

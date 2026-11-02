package client

import (
	"testing"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func TestClientMessageBinaryRoundTrip(t *testing.T) {
	props := packet.NewPublishProperties()
	props.TraceID = "trace-1"
	props.ExpiryTime = 1700000123
	props.UserProperties = map[string]string{"k": "v"}

	original := Message{
		Packet: &packet.PublishPacket{
			QoS:        packet.QoS1,
			Retain:     true,
			Topic:      "a/b",
			PacketID:   packet.NewPacketID(),
			Properties: props,
			Payload:    []byte("hello"),
		},
		Dup:       true,
		Timestamp: 1700000000,
	}

	data, err := mq.EncodeMessage(original)
	require.NoError(t, err)

	var got Message
	require.NoError(t, mq.DecodeMessage(&got, data))

	require.Equal(t, original.Dup, got.Dup)
	require.Equal(t, original.Timestamp, got.Timestamp)
	require.NotNil(t, got.Packet)
	require.Equal(t, original.Packet.Topic, got.Packet.Topic)
	require.Equal(t, original.Packet.PacketID, got.Packet.PacketID)
	require.Equal(t, original.Packet.QoS, got.Packet.QoS)
	require.Equal(t, original.Packet.Retain, got.Packet.Retain)
	require.Equal(t, original.Packet.Payload, got.Packet.Payload)
	require.Equal(t, original.Packet.Properties, got.Packet.Properties)
}

func TestClientMessageUnmarshalBinaryRejectsInvalid(t *testing.T) {
	valid, err := mq.EncodeMessage(Message{
		Packet: packet.NewPublishPacket("t", nil),
	})
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{"too short", valid[:clientMessageHeaderSize-1]},
		{"bad version", append([]byte{0xFF}, valid[1:]...)},
		{"missing packet", valid[:clientMessageHeaderSize]},
		{"trailing bytes", append(append([]byte(nil), valid...), 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Message
			require.ErrorIs(t, got.UnmarshalBinary(tt.data), ErrInvalidMessage)
		})
	}
}

func TestClientMessageMarshalBinaryRejectsNilPacket(t *testing.T) {
	_, err := Message{}.MarshalBinary()
	require.ErrorIs(t, err, ErrInvalidMessage)
}

package core

import (
	"encoding/binary"
	"testing"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func testPublishProperties() *packet.PublishProperties {
	props := packet.NewPublishProperties()
	props.ExpiryTime = 1700000123
	props.ContentType = "application/json"
	props.TraceID = "trace-1"
	props.TargetClientID = "target-client"
	props.SourceClientID = "source-client"
	props.ResponseTopic = "reply/topic"
	props.CorrelationData = []byte{1, 2, 3}
	props.UserProperties = map[string]string{"k": "v"}
	return props
}

func TestMessageBinaryRoundTrip(t *testing.T) {
	original := Message{
		Packet: &packet.PublishPacket{
			QoS:        packet.QoS1,
			Retain:     true,
			Topic:      "a/b",
			PacketID:   packet.NewPacketID(),
			Properties: testPublishProperties(),
			Payload:    []byte("hello"),
		},
		Dup:       true,
		QoS:       packet.QoS1,
		Retain:    true,
		PacketID:  packet.NewPacketID(),
		Timestamp: 1700000000,
	}

	data, err := mq.EncodeMessage(original)
	require.NoError(t, err)

	var got Message
	require.NoError(t, mq.DecodeMessage(&got, data))

	require.Equal(t, original.Dup, got.Dup)
	require.Equal(t, original.QoS, got.QoS)
	require.Equal(t, original.Retain, got.Retain)
	require.Equal(t, original.PacketID, got.PacketID)
	require.Equal(t, original.Timestamp, got.Timestamp)

	require.NotNil(t, got.Packet)
	require.Equal(t, original.Packet.Topic, got.Packet.Topic)
	require.Equal(t, original.Packet.PacketID, got.Packet.PacketID)
	require.Equal(t, original.Packet.QoS, got.Packet.QoS)
	require.Equal(t, original.Packet.Retain, got.Packet.Retain)
	require.Equal(t, original.Packet.Payload, got.Packet.Payload)
	require.Equal(t, original.Packet.Properties, got.Packet.Properties)
}

func TestMessageUnmarshalBinaryRejectsInvalid(t *testing.T) {
	valid, err := mq.EncodeMessage(Message{
		Packet:   packet.NewPublishPacket("t", nil),
		PacketID: packet.NewPacketID(),
	})
	require.NoError(t, err)

	badVersion := append([]byte{0xFF}, valid[1:]...)
	trailing := append(append([]byte(nil), valid...), 0)

	tests := []struct {
		name string
		data []byte
	}{
		{"too short", valid[:messageHeaderSize-1]},
		{"bad version", badVersion},
		{"missing packet", valid[:messageHeaderSize]},
		{"trailing bytes", trailing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Message
			require.ErrorIs(t, got.UnmarshalBinary(tt.data), ErrInvalidMessage)
		})
	}
}

func TestMessageUnmarshalBinaryRejectsNonPublishPacket(t *testing.T) {
	frame, err := packet.EncodePacket(packet.NewConnackPacket(packet.ReasonSuccess), 0)
	require.NoError(t, err)

	data := make([]byte, messageHeaderSize)
	data[0] = messageFormatVersion
	data = append(data, frame...)

	var got Message
	require.ErrorIs(t, got.UnmarshalBinary(data), ErrInvalidMessage)
}

func TestMessageUnmarshalBinaryRejectsOversizedFrame(t *testing.T) {
	// 固定头声明 1 GiB 的报文长度，但实际只有 5 字节：
	// 解码必须在分配缓冲前拒绝，而不是按声明长度分配。
	data := make([]byte, messageHeaderSize+5)
	data[0] = messageFormatVersion
	data[messageHeaderSize] = byte(packet.PUBLISH)
	binary.BigEndian.PutUint32(data[messageHeaderSize+1:], 1<<30)

	var got Message
	require.ErrorIs(t, got.UnmarshalBinary(data), ErrInvalidMessage)
}

func TestMessageMarshalBinaryRejectsNilPacket(t *testing.T) {
	_, err := Message{PacketID: packet.NewPacketID()}.MarshalBinary()
	require.ErrorIs(t, err, ErrInvalidMessage)
}

func TestSessionRecordBinaryRoundTrip(t *testing.T) {
	original := &sessionRecord{
		ClientID:  "client-1",
		Timeout:   60,
		ExpiresAt: 1700000000,
		Subscriptions: []sessionSubscription{
			{
				Topic:             "a/*",
				QoS:               1,
				NoLocal:           true,
				RetainAsPublished: true,
				RetainHandling:    2,
			},
			{Topic: "b", QoS: 0},
		},
	}

	data, err := mq.EncodeMessage(original)
	require.NoError(t, err)

	var got sessionRecord
	require.NoError(t, mq.DecodeMessage(&got, data))
	require.Equal(t, *original, got)
}

func TestSessionRecordUnmarshalBinaryRejectsInvalid(t *testing.T) {
	valid, err := mq.EncodeMessage(&sessionRecord{ClientID: "c", Timeout: 1})
	require.NoError(t, err)

	tests := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"bad version", append([]byte{0xFF}, valid[1:]...)},
		{"truncated", valid[:len(valid)-1]},
		{"trailing bytes", append(append([]byte(nil), valid...), 0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sessionRecord
			require.Error(t, got.UnmarshalBinary(tt.data))
		})
	}
}

func TestSessionRecordUnmarshalBinaryRejectsBadSubscriptionCount(t *testing.T) {
	// version + 空 ClientID + Timeout + ExpiresAt + 超大订阅计数（无对应数据）
	data := []byte{sessionRecordFormatVersion, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF}

	var got sessionRecord
	require.Error(t, got.UnmarshalBinary(data))
}

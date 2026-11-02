package core

import (
	"bytes"
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestMessageFrame_MatchesEncodePacket 验证分段写出的 PUBLISH 帧与
// packet.EncodePacket 完全一致：零拷贝路径不能改变线格式。
func TestMessageFrame_MatchesEncodePacket(t *testing.T) {
	props := packet.NewPublishProperties()
	props.ContentType = "application/json"
	props.TraceID = "trace-1"
	props.SourceClientID = "source-client"
	props.UserProperties = map[string]string{"k": "v"}

	pub := &packet.PublishPacket{
		Topic:      "a/b/c",
		Payload:    []byte("hello"),
		QoS:        packet.QoS1,
		Retain:     true,
		Properties: props,
	}

	msg, err := newMessage(pub)
	require.NoError(t, err)

	d := &delivery{
		msg:      msg,
		packetID: packet.NewPacketID(),
		qos:      packet.QoS1,
		retain:   true,
	}
	d.dup.Store(true)

	frame := msg.frame(d)

	var got bytes.Buffer
	_, err = frame.WriteTo(&got)
	require.NoError(t, err)

	want, err := packet.EncodePacket(&packet.PublishPacket{
		Topic:      pub.Topic,
		Payload:    pub.Payload,
		QoS:        packet.QoS1,
		Retain:     true,
		Dup:        true,
		PacketID:   d.packetID,
		Properties: pub.Properties,
	}, 0)
	require.NoError(t, err)

	require.Equal(t, want, got.Bytes())
	require.Equal(t, uint32(len(want)), frame.Size())
}

// TestNewMessage_ReusesPayload 验证消息内容层直接引用原始载荷，不做拷贝。
func TestNewMessage_ReusesPayload(t *testing.T) {
	payload := []byte("payload")
	pub := packet.NewPublishPacket("reuse/topic", payload)

	msg, err := newMessage(pub)
	require.NoError(t, err)

	require.Same(t, pub, msg.Packet)
	require.Equal(t, len(payload), msg.PayloadLen())
	require.Equal(t, "reuse/topic", msg.Topic())
}

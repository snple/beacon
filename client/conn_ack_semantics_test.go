package client

import (
	"testing"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestHandlePublish_QueueFullQoS1LeavesUnacknowledged 验证接收队列满时不再回
// PUBACK(QuotaExceeded)（那会让服务端直接删除消息），而是保持未确认等待重传。
func TestHandlePublish_QueueFullQoS1LeavesUnacknowledged(t *testing.T) {
	client, conn, capture := setupManualAckConn(t, true)

	for range cap(client.messageQueue) {
		client.messageQueue <- &Message{Packet: packet.NewPublishPacket("filler", nil)}
	}

	pub := packet.NewPublishPacket("test/topic", []byte("x"))
	pub.QoS = packet.QoS1
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))
	require.Empty(t, capture.buf, "queue full must not send any PUBACK")
}

// TestHandlePublish_HookRejectSendsNotAuthorized 验证钩子拒绝时返回 NotAuthorized，
// 而不是伪装成已成功处理。
func TestHandlePublish_HookRejectSendsNotAuthorized(t *testing.T) {
	client, conn, capture := setupManualAckConn(t, true)
	client.options.WithMessageHandler(MessageHandlerFunc(func(ctx *PublishContext) bool {
		return false
	}))

	pub := packet.NewPublishPacket("test/topic", []byte("x"))
	pub.QoS = packet.QoS1
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))

	pkt := readSinglePacket(t, capture)
	ack, ok := pkt.(*packet.PubackPacket)
	require.True(t, ok, "expected PUBACK, got %T", pkt)
	require.Equal(t, packet.ReasonNotAuthorized, ack.ReasonCode)
}

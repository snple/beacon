package client

import (
	"bytes"
	"context"
	"testing"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// captureConn 捕获写入数据的模拟连接（拼接所有 Write）
type captureConn struct {
	mockConn
	buf []byte
}

func (c *captureConn) Write(b []byte) (int, error) {
	c.buf = append(c.buf, b...)
	return len(b), nil
}

// setupManualAckConn 构造用于测试 handlePublish 的 Client+Conn
func setupManualAckConn(t *testing.T, autoAck bool) (*Client, *Conn, *captureConn) {
	t.Helper()

	opts := NewClientOptions().
		WithAutoAck(autoAck).
		WithMessageQueueSize(10)

	client := &Client{
		options:      opts,
		messageQueue: make(chan *Message, 10),
		ctx:          context.Background(),
		logger:       zap.NewNop(),
	}

	capture := &captureConn{}
	conn := &Conn{
		client:     client,
		conn:       capture,
		pendingAck: make(map[nson.Id]pendingMessage),
		ctx:        context.Background(),
		logger:     zap.NewNop(),
	}

	return client, conn, capture
}

// readSinglePacket 从捕获缓冲区中解析出一个完整数据包
func readSinglePacket(t *testing.T, capture *captureConn) packet.Packet {
	t.Helper()
	require.NotEmpty(t, capture.buf, "expected captured writes")
	pkt, err := packet.ReadPacket(bytes.NewReader(capture.buf), 0)
	require.NoError(t, err)
	return pkt
}

// TestHandlePublish_AutoAckEnabledAcksOnEnqueue 验证默认（AutoAck=true）时入队即自动 PUBACK
func TestHandlePublish_AutoAckEnabledAcksOnEnqueue(t *testing.T) {
	_, conn, capture := setupManualAckConn(t, true)

	pub := packet.NewPublishPacket("test/topic", []byte("hello"))
	pub.QoS = packet.QoS1
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))

	pkt := readSinglePacket(t, capture)
	ack, ok := pkt.(*packet.PubackPacket)
	require.True(t, ok, "expected PUBACK on enqueue, got %T", pkt)
	require.Equal(t, pub.PacketID, ack.PacketID)
}

// TestHandlePublish_AutoAckDisabledRequiresManualAck 验证 AutoAck=false 时需手动确认
func TestHandlePublish_AutoAckDisabledRequiresManualAck(t *testing.T) {
	client, conn, capture := setupManualAckConn(t, false)

	pub := packet.NewPublishPacket("test/topic", []byte("hello"))
	pub.QoS = packet.QoS1
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))

	// 入队后不应自动发送 PUBACK
	require.Empty(t, capture.buf, "expected no PUBACK before manual ack")

	// 从 messageQueue 取出消息并手动确认
	msg := <-client.messageQueue
	require.NotNil(t, msg)
	require.NoError(t, msg.Ack())

	// 手动确认后应发送 PUBACK
	pkt := readSinglePacket(t, capture)
	ack, ok := pkt.(*packet.PubackPacket)
	require.True(t, ok, "expected PUBACK after manual ack, got %T", pkt)
	require.Equal(t, pub.PacketID, ack.PacketID)
}

// TestHandlePublish_AutoAckDisabledRedeliversRetransmit 验证手动模式下重传不会被吞掉：
// 应用处理中断电/崩溃未确认时，服务端重传应重新投递给应用，保证 QoS1“消息被处理”的语义
// （因此手动模式下为 at-least-once，应用如需不重复处理，按 PacketID 自行去重）。
func TestHandlePublish_AutoAckDisabledRedeliversRetransmit(t *testing.T) {
	client, conn, _ := setupManualAckConn(t, false)

	pub := packet.NewPublishPacket("test/topic", []byte("hello"))
	pub.QoS = packet.QoS1
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))

	// 服务端重传同一 PacketID（Dup=true），尚未确认 → 应重新投递，不能丢弃
	dup := packet.NewPublishPacket("test/topic", []byte("hello"))
	dup.QoS = packet.QoS1
	dup.PacketID = pub.PacketID
	dup.Dup = true

	require.NoError(t, conn.handlePublish(dup))

	require.Len(t, client.messageQueue, 2, "unacked retransmit must be re-delivered to the app")
}

// TestHandlePublish_QoS0NoAck 验证 QoS0 消息在手动模式下也不发送 ACK，且 Ack() 返回 ErrNoPendingAck
func TestHandlePublish_QoS0NoAck(t *testing.T) {
	client, conn, capture := setupManualAckConn(t, false)

	pub := packet.NewPublishPacket("test/topic", []byte("hello"))
	pub.QoS = packet.QoS0
	pub.PacketID = nson.NewId()

	require.NoError(t, conn.handlePublish(pub))
	require.Empty(t, capture.buf, "QoS0 must not send PUBACK")

	msg := <-client.messageQueue
	require.ErrorIs(t, msg.Ack(), ErrNoPendingAck)
}

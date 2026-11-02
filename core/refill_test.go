package core

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// captureWriteConn 记录写入的字节，用于验证发送循环实际发出的数据包。
type captureWriteConn struct {
	buf []byte
}

func (c *captureWriteConn) Read(_ []byte) (int, error)         { return 0, io.EOF }
func (c *captureWriteConn) Close() error                       { return nil }
func (c *captureWriteConn) LocalAddr() net.Addr                { return testAddr("local") }
func (c *captureWriteConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (c *captureWriteConn) SetDeadline(_ time.Time) error      { return nil }
func (c *captureWriteConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *captureWriteConn) SetWriteDeadline(_ time.Time) error { return nil }

func (c *captureWriteConn) Write(b []byte) (int, error) {
	c.buf = append(c.buf, b...)
	return len(b), nil
}

// TestProcessSendQueue_DrainsMemoryQueue 验证发送协程会一次排空内存待发送队列。
func TestProcessSendQueue_DrainsMemoryQueue(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().WithRetransmitInterval(time.Hour))
	require.NoError(t, err)

	client := newQuotaTestClient(t, c, "refill-client")

	const n = 5
	for i := range n {
		msg := mustRegisterMessage(t, c, "refill/topic", []byte{byte('0' + i)})
		require.NoError(t, client.enqueueDelivery(msg, msg.QoS, msg.Retain))
		c.msgs.release(msg)
	}
	require.Equal(t, n, client.queue.len())

	// 全部入队后再挂上连接：否则 enqueueDelivery 会立即启动发送协程并发排空队列。
	capture := &captureWriteConn{}
	conn := &conn{
		client:        client,
		conn:          capture,
		maxPacketSize: 0,
		sendWindow:    10,
		resendSet:     make(map[packet.PacketID]struct{}),
	}
	client.conn = conn

	conn.processSendQueue()

	require.Equal(t, 0, client.queue.len(), "writer should drain the memory queue in one pass")

	// 解析捕获到的数据包数量
	reader := bytes.NewReader(capture.buf)
	count := 0
	for {
		_, err := packet.ReadPacket(reader, 0)
		if err != nil {
			break
		}
		count++
	}
	require.Equal(t, n, count, "all queued messages should be written")
}

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

// TestProcessSendQueue_RefillsFromPersistentQueue 验证发送协程排空内存队列后会立即从
// 持久化队列补货，而不是等待 RetransmitInterval 定时器。
func TestProcessSendQueue_RefillsFromPersistentQueue(t *testing.T) {
	c, err := NewWithOptions(NewCoreOptions().
		WithStoreDir(t.TempDir()).
		WithRetransmitInterval(time.Hour))
	require.NoError(t, err)
	defer c.store.close()

	client := newQuotaTestClient(t, c, "refill-client")

	capture := &captureWriteConn{}
	conn := &conn{
		client:        client,
		conn:          capture,
		sendQueue:     newSendQueue(10),
		maxPacketSize: 0,
	}
	client.conn = conn

	const n = 5
	for i := 0; i < n; i++ {
		msg := createTestMessage("refill/topic", []byte{byte('0' + i)}, packet.QoS0)
		require.NoError(t, client.queue.Enqueue(msg))
	}
	require.Equal(t, int64(n), client.queue.Count())

	conn.processSendQueue()

	require.Equal(t, int64(0), client.queue.Count(),
		"writer should drain the persistent queue without waiting for the retransmit tick")

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

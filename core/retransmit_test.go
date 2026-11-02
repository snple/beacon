package core

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

type failingWriteConn struct{}

func (failingWriteConn) Read(_ []byte) (int, error)         { return 0, errors.New("read not supported") }
func (failingWriteConn) Write(_ []byte) (int, error)        { return 0, errors.New("write failed") }
func (failingWriteConn) Close() error                       { return nil }
func (failingWriteConn) LocalAddr() net.Addr                { return testAddr("local") }
func (failingWriteConn) RemoteAddr() net.Addr               { return testAddr("remote") }
func (failingWriteConn) SetDeadline(_ time.Time) error      { return nil }
func (failingWriteConn) SetReadDeadline(_ time.Time) error  { return nil }
func (failingWriteConn) SetWriteDeadline(_ time.Time) error { return nil }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

// newSendTestClient 构造一个用于发送路径单测的离线客户端。
func newSendTestClient(t *testing.T, c *Core, clientID string) *Client {
	t.Helper()

	connect := packet.NewConnectPacket()
	connect.ClientID = clientID
	connect.Properties.ReceiveWindow = 10

	client := &Client{
		ID:      clientID,
		core:    c,
		session: newSession(clientID, connect, c),
	}
	client.initQueue()

	return client
}

// TestProcessSendQueue_RequeuesWithDupAfterSendFailure 验证写失败后投递回到队头并标记 Dup，
// 以便重连后重发时接收端可以识别重复投递。
func TestProcessSendQueue_RequeuesWithDupAfterSendFailure(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)

	client := newSendTestClient(t, core, "dup-persist-client")

	conn := &conn{
		client:        client,
		conn:          failingWriteConn{},
		sendWindow:    10,
		resendSet:     make(map[packet.PacketID]struct{}),
		maxPacketSize: 0,
	}
	client.conn = conn

	msg := mustRegisterMessage(t, core, "test/topic", []byte("payload"))
	require.NoError(t, client.enqueueDelivery(msg, packet.QoS1, false))
	core.msgs.release(msg)

	conn.processSendQueue()

	head := client.queue.peek()
	require.NotNil(t, head, "failed delivery should stay queued for the next connection")
	require.True(t, head.dup.Load(), "failed sends should be marked Dup for the next attempt")
}

// TestProcessSendQueue_DropsDeliveryRejectedMessages 验证被 OnDeliver 钩子拒绝的投递
// 会被直接丢弃，而不是永远重试。
func TestProcessSendQueue_DropsDeliveryRejectedMessages(t *testing.T) {
	core, err := NewWithOptions(
		NewCoreOptions().WithMessageHandler(&MessageHandlerFunc{
			DeliverFunc: func(ctx *DeliverContext) bool {
				return false
			},
		}),
	)
	require.NoError(t, err)

	client := newSendTestClient(t, core, "delivery-rejected-client")

	conn := &conn{
		client:     client,
		conn:       &captureWriteConn{},
		sendWindow: 10,
		resendSet:  make(map[packet.PacketID]struct{}),
	}
	client.conn = conn

	msg := mustRegisterMessage(t, core, "test/topic", []byte("payload"))
	require.NoError(t, client.enqueueDelivery(msg, packet.QoS1, false))
	core.msgs.release(msg)

	conn.processSendQueue()

	require.Equal(t, 0, client.queue.len(),
		"delivery rejected by hook should be dropped instead of retried forever")
	require.Equal(t, int64(1), core.stats.MessagesDropped.Load())
}

// TestTriggerRetransmit_ConcurrentWithStop 验证重传与停机并发时不会 panic。
func TestTriggerRetransmit_ConcurrentWithStop(t *testing.T) {
	for range 3 {
		core := testSetupCore(t, NewCoreOptions())
		addr := testServe(t, core)

		cl := testSetupClient(t, addr, "retransmit-stop", nil)

		done := make(chan struct{})
		go func() {
			defer close(done)
			for range 200 {
				core.triggerRetransmit()
			}
		}()

		time.Sleep(5 * time.Millisecond)
		cl.Close()
		require.NoError(t, core.Stop())
		<-done
	}
}

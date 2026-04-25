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

func TestProcessSendQueue_PersistsDupAfterSendFailure(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "dup-persist-client"
	connect.Properties.ReceiveWindow = 10

	client := &Client{
		ID:      connect.ClientID,
		core:    core,
		session: newSession(connect.ClientID, connect, core),
	}
	require.NoError(t, client.initQueue())

	conn := &conn{
		client:    client,
		conn:      failingWriteConn{},
		sendQueue: newSendQueue(10),
	}
	client.conn = conn

	msg := createTestMessage("test/topic", []byte("payload"), packet.QoS1)
	require.NoError(t, client.queue.Enqueue(msg))
	require.NoError(t, conn.sendQueue.tryEnqueue(msg))

	conn.processSendQueue()

	persisted, err := client.queue.Peek()
	require.NoError(t, err)
	require.True(t, persisted.Dup, "failed sends should persist Dup=true for the next real retransmit")
}

func TestProcessSendQueue_DropsDeliveryRejectedMessages(t *testing.T) {
	core, err := NewWithOptions(
		NewCoreOptions().
			WithStoreDir(t.TempDir()).
			WithMessageHandler(&MessageHandlerFunc{
				DeliverFunc: func(ctx *DeliverContext) bool {
					return false
				},
			}),
	)
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "delivery-rejected-client"
	connect.Properties.ReceiveWindow = 10

	client := &Client{
		ID:      connect.ClientID,
		core:    core,
		session: newSession(connect.ClientID, connect, core),
	}
	require.NoError(t, client.initQueue())

	conn := &conn{
		client:    client,
		sendQueue: newSendQueue(10),
	}
	client.conn = conn

	msg := createTestMessage("test/topic", []byte("payload"), packet.QoS1)
	require.NoError(t, client.queue.Enqueue(msg))
	require.NoError(t, conn.sendQueue.tryEnqueue(msg))

	conn.processSendQueue()

	isEmpty, err := client.queue.IsEmpty()
	require.NoError(t, err)
	require.True(t, isEmpty, "delivery rejected by hook should be dropped instead of retried forever")
}

package client

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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

func setupClientTestDB(t *testing.T) (*badger.DB, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "client_queue_test_*")
	require.NoError(t, err)

	db, err := badger.Open(badger.DefaultOptions(dir).WithLoggingLevel(badger.ERROR))
	require.NoError(t, err)

	cleanup := func() {
		_ = db.Close()
		_ = os.RemoveAll(dir)
	}

	return db, cleanup
}

func createClientTestMessage(topic string, payload []byte, qos packet.QoS) *Message {
	pkt := packet.NewPublishPacket(topic, payload)
	pkt.QoS = qos
	pkt.PacketID = nson.NewId()

	return &Message{
		Packet:    pkt,
		Timestamp: time.Now().Unix(),
	}
}

func TestSendQueueRetainsDedupUntilFinish(t *testing.T) {
	queue := newSendQueue(4)
	msg := createClientTestMessage("test/topic", []byte("payload"), packet.QoS1)

	require.NoError(t, queue.tryEnqueue(msg))
	dequeued, ok := queue.tryDequeue()
	require.True(t, ok)
	require.Same(t, msg, dequeued)

	err := queue.tryEnqueue(msg)
	require.ErrorIs(t, err, ErrMessageAlreadyInQueue)

	queue.finish(msg.Packet.PacketID)
	require.NoError(t, queue.tryEnqueue(msg))
}

func TestRetransmitQueuedMessages_FirstDeliveryKeepsDupFalse(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	msg := createClientTestMessage("test/topic", []byte("payload"), packet.QoS1)
	queue := NewQueue(db, "client:test:")
	require.NoError(t, queue.Enqueue(msg))

	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:    client,
		sendQueue: newSendQueue(4),
		logger:    zap.NewNop(),
	}
	conn.processing.Store(true)
	client.conn = conn

	sent := client.retransmitQueuedMessages()
	require.Equal(t, 1, sent)

	dequeued, ok := conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.False(t, dequeued.Dup, "first queued delivery should not be marked as duplicate")
}

func TestRetransmitQueuedMessages_SkipsHeadMessageAwaitingPuback(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	msg := createClientTestMessage("test/topic", []byte("payload"), packet.QoS1)
	queue := NewQueue(db, "client:test:")
	require.NoError(t, queue.Enqueue(msg))

	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:     client,
		sendQueue:  newSendQueue(4),
		pendingAck: map[nson.Id]chan error{msg.Packet.PacketID: make(chan error, 1)},
		logger:     zap.NewNop(),
	}
	conn.processing.Store(true)
	client.conn = conn

	sent := client.retransmitQueuedMessages()
	require.Equal(t, 0, sent)

	_, ok := conn.sendQueue.tryDequeue()
	require.False(t, ok, "message awaiting PUBACK should not be re-enqueued for retransmit")
}

func TestProcessSendQueue_PersistsDupAfterSendFailure(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "client:test:")
	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:    client,
		conn:      failingWriteConn{},
		sendQueue: newSendQueue(4),
		logger:    zap.NewNop(),
	}
	client.conn = conn

	msg := createClientTestMessage("test/topic", []byte("payload"), packet.QoS1)
	require.NoError(t, queue.Enqueue(msg))
	require.NoError(t, conn.sendQueue.tryEnqueue(msg))

	conn.processSendQueue()

	persisted, err := queue.Peek()
	require.NoError(t, err)
	require.True(t, persisted.Dup, "failed sends should persist Dup=true for the next real retransmit")
}

func TestSendMessage_UsesMessageDupFlag(t *testing.T) {
	writer, reader := net.Pipe()
	defer writer.Close()
	defer reader.Close()

	client := &Client{logger: zap.NewNop()}
	conn := &Conn{
		client: client,
		conn:   writer,
		logger: zap.NewNop(),
	}

	msg := createClientTestMessage("test/topic", []byte("payload"), packet.QoS1)
	msg.Dup = true

	errCh := make(chan error, 1)
	go func() {
		errCh <- conn.sendMessage(msg)
	}()

	pkt, err := packet.ReadPacket(reader, 0)
	require.NoError(t, err)

	pub, ok := pkt.(*packet.PublishPacket)
	require.True(t, ok)
	require.True(t, pub.Dup, "wire packet should reflect Message.Dup")
	require.NoError(t, <-errCh)
}

func TestPublish_ClonesPayloadInput(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "client:test:")
	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:    client,
		sendQueue: newSendQueue(4),
		logger:    zap.NewNop(),
	}
	conn.processing.Store(true)
	client.conn = conn

	payload := []byte("payload")
	require.NoError(t, client.Publish("test/topic", payload, NewPublishOptions().WithQoS(packet.QoS0)))

	copy(payload, []byte("mutated"))

	msg, ok := conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.Equal(t, []byte("payload"), msg.Packet.Payload, "publish should not retain caller-owned payload slice")
}

func TestPublish_ClonesMutablePropertiesInput(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "client:test:")
	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:    client,
		sendQueue: newSendQueue(4),
		logger:    zap.NewNop(),
	}
	conn.processing.Store(true)
	client.conn = conn

	correlationData := []byte("corr")
	userProperties := map[string]string{"x-origin": "client"}
	require.NoError(t, client.Publish(
		"test/topic",
		[]byte("payload"),
		NewPublishOptions().
			WithCorrelationData(correlationData).
			WithUserProperties(userProperties),
	))

	copy(correlationData, []byte("xxxx"))
	userProperties["x-origin"] = "mutated"

	msg, ok := conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.NotNil(t, msg.Packet.Properties)
	require.Equal(t, []byte("corr"), msg.Packet.Properties.CorrelationData, "publish should clone caller-owned correlation data")
	require.Equal(t, "client", msg.Packet.Properties.UserProperties["x-origin"], "publish should clone caller-owned user properties")
}

func TestPublishToClient_DoesNotMutateCallerOptions(t *testing.T) {
	db, cleanup := setupClientTestDB(t)
	defer cleanup()

	queue := NewQueue(db, "client:test:")
	client := &Client{
		queue:  queue,
		logger: zap.NewNop(),
	}
	conn := &Conn{
		client:    client,
		sendQueue: newSendQueue(4),
		logger:    zap.NewNop(),
	}
	conn.processing.Store(true)
	client.conn = conn

	opts := NewPublishOptions().WithUserProperty("x-origin", "client")
	require.NoError(t, client.PublishToClient("target-client", "test/topic", []byte("payload"), opts))

	require.Empty(t, opts.TargetClientID, "PublishToClient should not mutate caller-owned options")
}

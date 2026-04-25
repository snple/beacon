package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

func TestBroadcast_CopiesMessagePerClient(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.Properties.ReceiveWindow = 10

	client1 := &Client{ID: "broadcast-client-1", core: core, session: newSession("broadcast-client-1", connect, core)}
	require.NoError(t, client1.initQueue())
	client1.conn = &conn{client: client1, sendQueue: newSendQueue(10)}
	client1.conn.processing.Store(true)

	client2 := &Client{ID: "broadcast-client-2", core: core, session: newSession("broadcast-client-2", connect, core)}
	require.NoError(t, client2.initQueue())
	client2.conn = &conn{client: client2, sendQueue: newSendQueue(10)}
	client2.conn.processing.Store(true)

	core.clients[client1.ID] = client1
	core.clients[client2.ID] = client2

	sent := core.Broadcast("broadcast/test", []byte("payload"), PublishOptions{QoS: packet.QoS1})
	require.Equal(t, 2, sent)

	msg1, ok := client1.conn.sendQueue.tryDequeue()
	require.True(t, ok)
	msg2, ok := client2.conn.sendQueue.tryDequeue()
	require.True(t, ok)

	require.NotSame(t, msg1, msg2, "broadcast should enqueue independent message instances per client")
	require.NotEqual(t, msg1.PacketID.Hex(), msg2.PacketID.Hex(), "broadcast deliveries should have distinct packet IDs per client")
	require.Equal(t, packet.QoS1, msg1.QoS, "broadcast should preserve requested QoS in delivery layer")
	require.Equal(t, packet.QoS1, msg2.QoS, "broadcast should preserve requested QoS in delivery layer")
}

func TestBroadcast_UsesDefaultMessageExpiry(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()).WithDefaultMessageExpiry(5 * time.Second))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.Properties.ReceiveWindow = 10

	client1 := &Client{ID: "broadcast-expiry-client", core: core, session: newSession("broadcast-expiry-client", connect, core)}
	require.NoError(t, client1.initQueue())
	client1.conn = &conn{client: client1, sendQueue: newSendQueue(10)}
	client1.conn.processing.Store(true)

	core.clients[client1.ID] = client1

	before := time.Now().Unix()
	sent := core.Broadcast("broadcast/expiry", []byte("payload"), PublishOptions{})
	require.Equal(t, 1, sent)

	msg, ok := client1.conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.NotNil(t, msg.Packet.Properties)
	require.Greater(t, msg.Packet.Properties.ExpiryTime, before, "broadcast should apply default message expiry")
	require.LessOrEqual(t, msg.Packet.Properties.ExpiryTime, before+6, "broadcast expiry should stay close to configured default")
}

func TestBroadcast_PreservesUserProperties(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.Properties.ReceiveWindow = 10

	client1 := &Client{ID: "broadcast-userprops-client", core: core, session: newSession("broadcast-userprops-client", connect, core)}
	require.NoError(t, client1.initQueue())
	client1.conn = &conn{client: client1, sendQueue: newSendQueue(10)}
	client1.conn.processing.Store(true)

	core.clients[client1.ID] = client1

	sent := core.Broadcast("broadcast/userprops", []byte("payload"), PublishOptions{UserProperties: map[string]string{"x-origin": "core"}})
	require.Equal(t, 1, sent)

	msg, ok := client1.conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.NotNil(t, msg.Packet.Properties)
	require.Equal(t, "core", msg.Packet.Properties.UserProperties["x-origin"])
}

func TestBroadcast_ClonesPayloadInput(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.Properties.ReceiveWindow = 10

	client1 := &Client{ID: "broadcast-payload-client", core: core, session: newSession("broadcast-payload-client", connect, core)}
	require.NoError(t, client1.initQueue())
	client1.conn = &conn{client: client1, sendQueue: newSendQueue(10)}
	client1.conn.processing.Store(true)

	core.clients[client1.ID] = client1

	payload := []byte("payload")
	sent := core.Broadcast("broadcast/payload", payload, PublishOptions{})
	require.Equal(t, 1, sent)

	copy(payload, []byte("mutated"))

	msg, ok := client1.conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.Equal(t, []byte("payload"), msg.Packet.Payload, "broadcast should not retain caller-owned payload slice")
}

func TestPublishToClient_ClonesPayloadInput(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithStoreDir(t.TempDir()))
	require.NoError(t, err)
	defer core.store.close()

	connect := packet.NewConnectPacket()
	connect.Properties.ReceiveWindow = 10

	client1 := &Client{ID: "publish-payload-client", core: core, session: newSession("publish-payload-client", connect, core)}
	require.NoError(t, client1.initQueue())
	client1.conn = &conn{client: client1, sendQueue: newSendQueue(10)}
	client1.conn.processing.Store(true)

	core.clients[client1.ID] = client1

	payload := []byte("payload")
	require.NoError(t, core.PublishToClient(client1.ID, "direct/payload", payload, PublishOptions{}))

	copy(payload, []byte("mutated"))

	msg, ok := client1.conn.sendQueue.tryDequeue()
	require.True(t, ok)
	require.NotNil(t, msg.Packet)
	require.Equal(t, []byte("payload"), msg.Packet.Payload, "publish-to-client should not retain caller-owned payload slice")
}

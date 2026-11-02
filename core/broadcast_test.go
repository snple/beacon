package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// newBroadcastTestClient 构造一个只有内存队列的离线客户端。
func newBroadcastTestClient(core *Core, clientID string) *Client {
	connect := packet.NewConnectPacket()
	connect.ClientID = clientID
	connect.Properties.ReceiveWindow = 10

	client := &Client{
		ID:      clientID,
		core:    core,
		session: newSession(clientID, connect, core),
	}
	client.initQueue()

	// 占位的连接：Broadcast 只投递给在线客户端；processing=true 时 triggerSend
	// 只会打标记而不会启动发送协程，投递留在队列里供断言。
	client.conn = &conn{client: client, conn: &captureWriteConn{}, sendWindow: 10, resendSet: make(map[packet.PacketID]struct{})}
	client.conn.processing.Store(true)

	return client
}

// TestBroadcast_SharesMessagePerClient 验证广播只保存一份消息内容：
// 各客户端队列中的投递共享同一个 *Message，但拥有各自独立的出站 PacketID。
func TestBroadcast_SharesMessagePerClient(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)

	client1 := newBroadcastTestClient(core, "broadcast-client-1")
	client2 := newBroadcastTestClient(core, "broadcast-client-2")

	core.clients[client1.ID] = client1
	core.clients[client2.ID] = client2

	sent := core.Broadcast("broadcast/test", []byte("payload"), PublishOptions{QoS: packet.QoS1})
	require.Equal(t, 2, sent)

	d1 := client1.queue.peek()
	d2 := client2.queue.peek()
	require.NotNil(t, d1)
	require.NotNil(t, d2)

	require.Same(t, d1.msg, d2.msg, "broadcast must share one message content across subscribers")
	require.NotEqual(t, d1.packetID.Hex(), d2.packetID.Hex(),
		"each delivery should still get its own packet ID for ACK tracking")
	require.Equal(t, packet.QoS1, d1.qos)
	require.Equal(t, packet.QoS1, d2.qos)
}

func TestBroadcast_UsesDefaultMessageExpiry(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions().WithDefaultMessageExpiry(5 * time.Second))
	require.NoError(t, err)

	client1 := newBroadcastTestClient(core, "broadcast-expiry-client")
	core.clients[client1.ID] = client1

	before := time.Now().Unix()
	sent := core.Broadcast("broadcast/expiry", []byte("payload"), PublishOptions{})
	require.Equal(t, 1, sent)

	d := client1.queue.peek()
	require.NotNil(t, d)
	require.NotNil(t, d.msg.Packet.Properties)
	require.Greater(t, d.msg.Packet.Properties.ExpiryTime, before, "broadcast should apply default message expiry")
	require.LessOrEqual(t, d.msg.Packet.Properties.ExpiryTime, before+6,
		"broadcast expiry should stay close to configured default")
}

func TestBroadcast_PreservesUserProperties(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)

	client1 := newBroadcastTestClient(core, "broadcast-userprops-client")
	core.clients[client1.ID] = client1

	sent := core.Broadcast("broadcast/userprops", []byte("payload"),
		PublishOptions{UserProperties: map[string]string{"x-origin": "core"}})
	require.Equal(t, 1, sent)

	d := client1.queue.peek()
	require.NotNil(t, d)
	require.Equal(t, "core", d.msg.Packet.Properties.UserProperties["x-origin"])
}

func TestBroadcast_ClonesPayloadInput(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)

	client1 := newBroadcastTestClient(core, "broadcast-payload-client")
	core.clients[client1.ID] = client1

	payload := []byte("payload")
	sent := core.Broadcast("broadcast/payload", payload, PublishOptions{})
	require.Equal(t, 1, sent)

	copy(payload, []byte("mutated"))

	d := client1.queue.peek()
	require.NotNil(t, d)
	require.Equal(t, []byte("payload"), d.msg.Packet.Payload,
		"broadcast should not retain caller-owned payload slice")
}

func TestPublishToClient_ClonesPayloadInput(t *testing.T) {
	core, err := NewWithOptions(NewCoreOptions())
	require.NoError(t, err)

	client1 := newBroadcastTestClient(core, "publish-payload-client")
	core.clients[client1.ID] = client1

	payload := []byte("payload")
	require.NoError(t, core.PublishToClient(client1.ID, "direct/payload", payload, PublishOptions{}))

	copy(payload, []byte("mutated"))

	d := client1.queue.peek()
	require.NotNil(t, d)
	require.Equal(t, []byte("payload"), d.msg.Packet.Payload,
		"publish-to-client should not retain caller-owned payload slice")
}

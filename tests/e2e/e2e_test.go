// Package e2e 提供基于公开 API 的端到端黑盒测试，
// 验证 core 与 client 作为外部库时的完整交互契约。
package e2e

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// startCore 启动一个 core（内存存储）并返回监听地址
func startCore(t *testing.T) (*core.Core, string) {
	t.Helper()

	c, err := core.NewWithOptions(core.NewCoreOptions())
	require.NoError(t, err)
	require.NoError(t, c.Start())

	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	go c.Serve(listener)

	t.Cleanup(func() {
		listener.Close()
		_ = c.Stop()
	})
	return c, addr
}

// connect 创建并连接一个客户端（自动清理）
func connect(t *testing.T, addr, id string) *client.Client {
	t.Helper()

	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore(addr).
			WithClientID(id),
	)
	require.NoError(t, err)
	require.NoError(t, c.ConnectWithDialer(&client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}))

	t.Cleanup(func() { c.Close() })
	return c
}

// TestE2E_PubSubQoS0 端到端：QoS0 发布订阅
func TestE2E_PubSubQoS0(t *testing.T) {
	_, addr := startCore(t)

	sub := connect(t, addr, "e2e-qos0-sub")
	pub := connect(t, addr, "e2e-qos0-pub")

	require.NoError(t, sub.Subscribe("e2e/qos0"))

	require.NoError(t, pub.Publish("e2e/qos0", []byte("hello"),
		client.NewPublishOptions().WithQoS(packet.QoS0)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, "e2e/qos0", msg.Packet.Topic)
	require.Equal(t, []byte("hello"), msg.Packet.Payload)
}

// TestE2E_PubSubQoS1 端到端：QoS1 发布订阅（含确认与投递）
func TestE2E_PubSubQoS1(t *testing.T) {
	_, addr := startCore(t)

	sub := connect(t, addr, "e2e-qos1-sub")
	pub := connect(t, addr, "e2e-qos1-pub")

	require.NoError(t, sub.Subscribe("e2e/qos1"))

	require.NoError(t, pub.Publish("e2e/qos1", []byte("qos1 msg"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("qos1 msg"), msg.Packet.Payload)
}

// TestE2E_PublishToClient 端到端：定向投递（不依赖订阅树）
func TestE2E_PublishToClient(t *testing.T) {
	_, addr := startCore(t)

	target := connect(t, addr, "e2e-target")
	pub := connect(t, addr, "e2e-dir-pub")

	require.NoError(t, pub.PublishToClient("e2e-target", "direct/topic", []byte("direct msg"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := target.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, "direct/topic", msg.Packet.Topic)
	require.Equal(t, []byte("direct msg"), msg.Packet.Payload)
}

// TestE2E_Retain 端到端：保留消息（新订阅者立即收到最后一条保留消息）
func TestE2E_Retain(t *testing.T) {
	_, addr := startCore(t)

	pub := connect(t, addr, "e2e-retain-pub")

	// Publish 在收到 PUBACK 后返回，此时保留消息已写入存储。
	require.NoError(t, pub.Publish("e2e/retain", []byte("last value"),
		client.NewPublishOptions().WithQoS(packet.QoS1).WithRetain(true)))

	// 新订阅者订阅保留主题，应立即收到保留消息
	sub := connect(t, addr, "e2e-retain-sub")
	require.NoError(t, sub.Subscribe("e2e/retain"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("last value"), msg.Packet.Payload)
}

// TestE2E_OfflinePublishDeliveredAfterReconnect 验证未连接时 Publish 的消息在连接后补发，
// 而不是在建立连接时被清空。
func TestE2E_OfflinePublishDeliveredAfterReconnect(t *testing.T) {
	_, addr := startCore(t)

	sub := connect(t, addr, "e2e-offline-sub")
	require.NoError(t, sub.Subscribe("e2e/offline"))

	pub, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("e2e-offline-pub"))
	require.NoError(t, err)
	defer pub.Close()

	// 未连接时发布：消息进入本地出站队列，等待连接后补发。
	require.NoError(t, pub.Publish("e2e/offline", []byte("offline"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	require.NoError(t, pub.ConnectWithDialer(&client.TCPDialer{Address: addr, DialTimeout: 5 * time.Second}))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 12*time.Second)
	require.NoError(t, err, "message published while offline should be delivered after reconnect")
	require.Equal(t, []byte("offline"), msg.Packet.Payload)
}

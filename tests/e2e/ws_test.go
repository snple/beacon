package e2e

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// startCoreWS 启动一个 core（内存存储），并开启 WebSocket 监听
func startCoreWS(t *testing.T) (*core.Core, string) {
	t.Helper()

	c, err := core.NewWithOptions(core.NewCoreOptions())
	require.NoError(t, err)
	require.NoError(t, c.Start())

	// 同步创建 listener，避免"先探测端口再重开"导致客户端抢先连接失败。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	wsAddr := ln.Addr().String()

	go c.ServeWSListener(ln, "/ws")

	t.Cleanup(func() {
		_ = c.Stop()
	})
	return c, wsAddr
}

// connectWS 通过 WebSocket 创建一个客户端（自动清理）
func connectWS(t *testing.T, wsAddr, id string) *client.Client {
	t.Helper()

	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore("ws://" + wsAddr + "/ws").
			WithClientID(id),
	)
	require.NoError(t, err)
	require.NoError(t, c.ConnectWithDialer(&client.WSDialer{
		Address:     "ws://" + wsAddr + "/ws",
		DialTimeout: 10 * time.Second,
	}))

	t.Cleanup(func() { c.Close() })
	return c
}

// TestE2E_WebSocket_PubSubQoS0 端到端：WebSocket 传输的 QoS0 发布订阅
func TestE2E_WebSocket_PubSubQoS0(t *testing.T) {
	_, wsAddr := startCoreWS(t)

	sub := connectWS(t, wsAddr, "e2e-ws-qos0-sub")
	pub := connectWS(t, wsAddr, "e2e-ws-qos0-pub")

	require.NoError(t, sub.Subscribe("e2e/ws/qos0"))

	require.NoError(t, pub.Publish("e2e/ws/qos0", []byte("hello-ws"),
		client.NewPublishOptions().WithQoS(packet.QoS0)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, "e2e/ws/qos0", msg.Packet.Topic)
	require.Equal(t, []byte("hello-ws"), msg.Packet.Payload)
}

// TestE2E_WebSocket_PubSubQoS1 端到端：WebSocket 传输的 QoS1 发布订阅（含确认）
func TestE2E_WebSocket_PubSubQoS1(t *testing.T) {
	_, wsAddr := startCoreWS(t)

	sub := connectWS(t, wsAddr, "e2e-ws-qos1-sub")
	pub := connectWS(t, wsAddr, "e2e-ws-qos1-pub")

	require.NoError(t, sub.Subscribe("e2e/ws/qos1"))

	require.NoError(t, pub.Publish("e2e/ws/qos1", []byte("hello-ws-qos1"),
		client.NewPublishOptions().WithQoS(packet.QoS1)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := sub.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, "e2e/ws/qos1", msg.Packet.Topic)
	require.Equal(t, []byte("hello-ws-qos1"), msg.Packet.Payload)
}

// TestE2E_WebSocket_Reconnect 端到端：WebSocket 客户端断线重连（自动重连需配合 WSDialer）
func TestE2E_WebSocket_Reconnect(t *testing.T) {
	_, wsAddr := startCoreWS(t)

	// 使用自动重连的客户端：ConnectWithDialer 支持重连
	c, err := client.NewWithOptions(
		client.NewClientOptions().
			WithCore("ws://" + wsAddr + "/ws").
			WithClientID("e2e-ws-reconnect"),
	)
	require.NoError(t, err)
	require.NoError(t, c.ConnectWithDialer(&client.WSDialer{
		Address:     "ws://" + wsAddr + "/ws",
		DialTimeout: 10 * time.Second,
	}))
	t.Cleanup(func() { c.Close() })

	// 订阅
	require.NoError(t, c.Subscribe("e2e/ws/reconnect"))

	// 发布一条消息并接收
	require.NoError(t, c.Publish("e2e/ws/reconnect", []byte("hello-reconnect"),
		client.NewPublishOptions().WithQoS(packet.QoS0)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msg, err := c.PollMessage(ctx, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, "e2e/ws/reconnect", msg.Packet.Topic)
	require.Equal(t, []byte("hello-reconnect"), msg.Packet.Payload)

	// 强制断开底层连接，验证自动重连后仍可收发
	// （重连循环有 1s 初始延迟，轮询等待）
	require.NoError(t, c.ForceClose())
	require.Eventually(t, func() bool { return c.IsConnected() },
		8*time.Second, 100*time.Millisecond, "client should reconnect automatically")

	require.NoError(t, c.Publish("e2e/ws/reconnect", []byte("after-reconnect"),
		client.NewPublishOptions().WithQoS(packet.QoS0)))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	msg2, err := c.PollMessage(ctx2, 4*time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("after-reconnect"), msg2.Packet.Payload)
}

var _ = fmt.Sprintf // 保留 fmt 导入（供后续扩展调试用）

package core

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestInfoAndReady 验证管理面服务器信息与就绪状态。
func TestInfoAndReady(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithBuildInfo("1.2.3", "abc123", "2026-01-01T00:00:00Z"))
	defer c.Stop()

	require.True(t, c.Ready())

	// listener 由 ServeTCP 注册，端口是随机的，所以要等它出现。
	go func() { _ = c.ServeTCP("127.0.0.1:0") }()

	var listeners []ListenerInfo
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "listener registered", func() bool {
		listeners = c.Listeners()
		return len(listeners) > 0
	}))
	require.Equal(t, "tcp", listeners[0].Protocol)
	require.NotEmpty(t, listeners[0].Address)

	info := c.Info()
	require.Equal(t, "1.2.3", info.Version)
	require.Equal(t, "abc123", info.Commit)
	require.Equal(t, "MX/2", info.Protocol)
	require.False(t, info.StartedAt.IsZero())
	require.Greater(t, info.Uptime, time.Duration(0))
	require.Equal(t, uint32(10000), info.Limits.MaxClients)
	require.True(t, info.Store.Ready)
	require.Len(t, info.Listeners, 1)

	require.NoError(t, c.Stop())
	require.False(t, c.Ready())
}

// TestPublishAPI 验证 broker 侧主题发布：命中订阅者、保留消息、非法主题拒绝。
func TestPublishAPI(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	online := testSetupClient(t, addr, "pub-online", nil)
	defer online.Close()
	require.NoError(t, online.Subscribe("api/topic"))

	// 离线会话：先订阅再断开，会话保留
	offline := testSetupClient(t, addr, "pub-offline", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, offline.Subscribe("api/topic"))
	require.NoError(t, offline.Disconnect())
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline session recorded", func() bool {
		return c.isOfflineSession("pub-offline")
	}))
	defer offline.Close()

	res, err := c.Publish("api/topic", []byte("hello"), PublishOptions{})
	require.NoError(t, err)
	require.True(t, res.Enqueued)
	require.Equal(t, 2, res.MatchedSubscribers, "在线 + 离线会话都应命中")
	require.False(t, res.Retained)

	msg, err := online.PollMessage(context.Background(), 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "api/topic", msg.Topic())
	require.Equal(t, []byte("hello"), msg.Payload())

	// 保留消息：新订阅者应能收到
	res, err = c.Publish("api/retained", []byte("keep"), PublishOptions{Retain: true})
	require.NoError(t, err)
	require.True(t, res.Retained)

	rsub := testSetupClient(t, addr, "pub-retained-sub", nil)
	defer rsub.Close()
	require.NoError(t, rsub.Subscribe("api/retained"))
	msg, err = rsub.PollMessage(context.Background(), 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, []byte("keep"), msg.Payload())

	// 非法主题
	for _, topic := range []string{"", "a/*", "a/**", "a+b"} {
		_, err := c.Publish(topic, []byte("x"), PublishOptions{})
		require.ErrorIs(t, err, ErrInvalidTopic, "topic %q", topic)
	}

	// QoS 非法
	_, err = c.Publish("api/topic", []byte("x"), PublishOptions{QoS: packet.QoS(9)})
	require.ErrorIs(t, err, ErrInvalidQoS)
}

// TestPublishAPI_NotStarted 验证未启动时发布返回 ErrCoreNotRunning。
func TestPublishAPI_NotStarted(t *testing.T) {
	c := round3Core(t, nil)
	defer c.Stop()

	_, err := c.Publish("t", []byte("x"), PublishOptions{})
	require.ErrorIs(t, err, ErrCoreNotRunning)
}

// TestListClients 验证轻量客户端列表：概要字段、在线/离线区分、字典序。
func TestListClients(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	for i := 0; i < 5; i++ {
		cli := testSetupClient(t, addr, fmt.Sprintf("list-%02d", i), nil)
		defer cli.Close()
	}
	// 一个离线会话
	off := testSetupClient(t, addr, "list-offline", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, off.Disconnect())
	defer off.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline recorded", func() bool {
		return c.isOfflineSession("list-offline")
	}))

	clients := c.ListClients()
	require.Len(t, clients, 6)
	for i, summary := range clients {
		if i > 0 {
			require.Less(t, clients[i-1].ClientID, summary.ClientID, "must be sorted by ClientID")
		}
		require.False(t, summary.ConnectedAt.IsZero())
		require.False(t, summary.LastSeenAt.IsZero())
		if summary.Connected {
			require.NotEmpty(t, summary.RemoteAddr)
			require.True(t, summary.ExpiresAt.IsZero())
		} else {
			require.Equal(t, "list-offline", summary.ClientID)
			require.Empty(t, summary.RemoteAddr)
			require.False(t, summary.ExpiresAt.IsZero())
		}
	}
}

// TestClientInfoFields 验证新增字段（队列水位、身份、传输方式、计数器）。
func TestClientInfoFields(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithAuthHandler(&AuthHandlerFunc{
		ConnectFunc: func(ctx *AuthConnectContext) error {
			ctx.Identity = &Identity{Subject: "device-42", Roles: []string{"sensor"}}
			return nil
		},
	}))
	defer c.Stop()

	addr := testServe(t, c)

	sub := testSetupClient(t, addr, "info-sub", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, sub.SubscribeWithOptions([]string{"info/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)))
	require.NoError(t, sub.Disconnect())
	defer sub.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline recorded", func() bool {
		return c.isOfflineSession("info-sub")
	}))

	// 离线入队两条消息
	for i := 0; i < 2; i++ {
		_, err := c.Publish("info/topic", []byte("payload"), PublishOptions{})
		require.NoError(t, err)
	}
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "messages enqueued", func() bool {
		info, err := c.GetClientInfo("info-sub")
		return err == nil && info.QueueSize >= 2
	}))

	info, err := c.GetClientInfo("info-sub")
	require.NoError(t, err)
	require.False(t, info.Connected)
	require.GreaterOrEqual(t, info.QueueSize, int64(2))
	require.Greater(t, info.QueueBytes, int64(0))
	require.Greater(t, info.OldestQueuedAge, time.Duration(0))
	require.False(t, info.ExpiresAt.IsZero())
	require.NotNil(t, info.Identity)
	require.Equal(t, "device-42", info.Identity.Subject)
	require.Len(t, info.Subscriptions, 1)

	// 在线客户端：传输方式与协议版本、字节计数、身份
	online := testSetupClient(t, addr, "info-online", nil)
	defer online.Close()
	require.NoError(t, online.Subscribe("info/online"))
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "online info available", func() bool {
		oi, err := c.GetClientInfo("info-online")
		return err == nil && oi.Connected && oi.Transport == "tcp"
	}))
	oi, err := c.GetClientInfo("info-online")
	require.NoError(t, err)
	require.Equal(t, "MX/2", oi.ProtocolVersion)
	require.Greater(t, oi.BytesReceived, int64(0))
	require.True(t, oi.ExpiresAt.IsZero())
	require.Equal(t, "device-42", oi.Identity.Subject)
}

// TestRetainedPublishAndClear 验证保留消息的写入，以及用"空负载 + Retain"清除。
func TestRetainedPublishAndClear(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	_, err := c.Publish("r/a", []byte("aaa"), PublishOptions{Retain: true})
	require.NoError(t, err)
	_, err = c.Publish("r/a", []byte("bbbb"), PublishOptions{Retain: true, QoS: packet.QoS1})
	require.NoError(t, err)

	// 新订阅者应收到最后一次写入的保留消息
	sub := testSetupClient(t, addr, "retained-sub", nil)
	defer sub.Close()
	require.NoError(t, sub.Subscribe("r/a"))
	msg, err := sub.PollMessage(context.Background(), 3*time.Second)
	require.NoError(t, err)
	require.Equal(t, "r/a", msg.Topic())
	require.Equal(t, []byte("bbbb"), msg.Payload())

	// 清除：空负载 + Retain（协议语义）
	_, err = c.Publish("r/a", nil, PublishOptions{Retain: true})
	require.NoError(t, err)

	sub2 := testSetupClient(t, addr, "retained-sub2", nil)
	defer sub2.Close()
	require.NoError(t, sub2.Subscribe("r/a"))
	_, err = sub2.PollMessage(context.Background(), 500*time.Millisecond)
	require.Error(t, err, "retained message should have been cleared")

	// 非法主题仍被拒绝
	_, err = c.Publish("bad/*", nil, PublishOptions{Retain: true})
	require.ErrorIs(t, err, ErrInvalidTopic)
}

// TestClientInfoPendingAck 验证详情中的在途 QoS1 水位。
func TestClientInfoPendingAck(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	// 手动确认模式且不轮询：消息留在 pendingAck 中
	sub := testSetupClient(t, addr, "ack-sub",
		client.NewClientOptions().WithAutoAck(false))
	defer sub.Close()
	require.NoError(t, sub.SubscribeWithOptions([]string{"ack/topic"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)))

	_, err := c.Publish("ack/topic", []byte("x"), PublishOptions{QoS: packet.QoS1})
	require.NoError(t, err)

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "pending ack recorded", func() bool {
		info, err := c.GetClientInfo("ack-sub")
		return err == nil && info.PendingAck == 1 && info.OldestPendingAckAge > 0
	}))

	info, err := c.GetClientInfo("ack-sub")
	require.NoError(t, err)
	require.Equal(t, 1, info.PendingAck)
	require.Positive(t, info.OldestPendingAckAge)

	_, err = c.GetClientInfo("nobody")
	require.ErrorIs(t, err, &ClientNotFoundError{})
}

// TestSubscribeEvents 验证事件流的类型、顺序与每订阅者连续序号。
func TestSubscribeEvents(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := c.SubscribeEvents(ctx, EventOptions{Buffer: 256})
	require.NoError(t, err)
	defer stream.Close()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "event-client", client.NewClientOptions().WithSessionTimeout(60))
	defer cli.Close()

	require.NoError(t, cli.Subscribe("event/topic"))
	_, err = c.Publish("event/topic", []byte("x"), PublishOptions{})
	require.NoError(t, err)
	require.NoError(t, cli.Disconnect())

	want := []EventType{
		EventClientConnected,
		EventSubscribed,
		EventMessagePublished,
		EventClientDisconnected,
	}

	var got []Event
	timeout := time.After(5 * time.Second)
	for len(got) < len(want) {
		select {
		case ev := <-stream.Events():
			got = append(got, ev)
		case <-timeout:
			t.Fatalf("timeout waiting for events, got %v", typesOf(got))
		}
	}

	for i, wantType := range want {
		require.Equal(t, wantType, got[i].Type, "event %d", i)
		require.Equal(t, uint64(i+1), got[i].Seq, "seq must be contiguous per subscriber")
		require.False(t, got[i].Time.IsZero())
	}
	require.Equal(t, "event-client", got[0].ClientID)
	require.NotEmpty(t, got[0].RemoteAddr)
	require.Equal(t, "event/topic", got[1].Topic)
	require.Equal(t, packet.ReasonNormalDisconnect.String(), got[3].Reason)
	require.Greater(t, got[3].Duration, time.Duration(0))
	require.Zero(t, stream.Dropped())
}

// TestSubscribeEventsFilterAndDrop 验证过滤、每订阅者序号与慢消费者丢弃。
func TestSubscribeEventsFilterAndDrop(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := c.SubscribeEvents(ctx, EventOptions{Types: []EventType{EventAdminAction}, Buffer: 1})
	require.NoError(t, err)
	defer stream.Close()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "drop-client", nil)
	defer cli.Close()

	// 大量非 admin 事件既不入队、也不占用序号
	for i := 0; i < 50; i++ {
		_, err := c.Publish("drop/topic", []byte("x"), PublishOptions{})
		require.NoError(t, err)
	}
	select {
	case ev := <-stream.Events():
		t.Fatalf("unexpected event %v", ev.Type)
	case <-time.After(100 * time.Millisecond):
	}

	// 三个 admin.action，缓冲只有 1 条：后两条被丢弃
	for _, actor := range []string{"admin-1", "admin-2", "admin-3"} {
		_, err = c.Publish("drop/topic", []byte("x"), PublishOptions{PublisherID: actor})
		require.NoError(t, err)
	}

	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "events dropped", func() bool {
		return stream.Dropped() == 2
	}))

	ev := <-stream.Events()
	require.Equal(t, EventAdminAction, ev.Type)
	require.Equal(t, uint64(1), ev.Seq)
	require.Equal(t, "admin-1", ev.Actor)
	require.Equal(t, uint64(2), stream.Dropped())

	// 再发布一条：序号 4 与已见的 1 相差 3，正好等于丢弃数（2）+ 本条
	_, err = c.Publish("drop/topic", []byte("x"), PublishOptions{PublisherID: "admin-4"})
	require.NoError(t, err)
	ev = <-stream.Events()
	require.Equal(t, uint64(4), ev.Seq)

	// 取消订阅后通道关闭
	cancel()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "event channel closed", func() bool {
		select {
		case _, ok := <-stream.Events():
			return !ok
		default:
			return false
		}
	}))
}

// TestSubscribeEventsValidation 验证参数校验与显式关闭。
func TestSubscribeEventsValidation(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	// 故意传入 nil context 验证防护（避免无人关闭的订阅泄漏）
	var nilCtx context.Context
	_, err := c.SubscribeEvents(nilCtx, EventOptions{})
	require.ErrorIs(t, err, ErrNilContext)

	_, err = c.SubscribeEvents(context.Background(), EventOptions{Types: []EventType{"no.such.event"}})
	require.ErrorIs(t, err, ErrInvalidEventType)

	stream, err := c.SubscribeEvents(context.Background(), EventOptions{})
	require.NoError(t, err)

	// 重复 Close 安全
	stream.Close()
	stream.Close()

	_, ok := <-stream.Events()
	require.False(t, ok, "channel must be closed")
}

// TestEmitHotPathNoAlloc 验证事件发布在无订阅者/有订阅者时都不产生堆分配。
func TestEmitHotPathNoAlloc(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	ev := Event{Type: EventMessagePublished, ClientID: "c1", Topic: "hot/topic", Bytes: 8}

	require.Zero(t, testing.AllocsPerRun(1000, func() { c.emit(ev) }),
		"emit without subscribers must not allocate")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := c.SubscribeEvents(ctx, EventOptions{Buffer: 4096})
	require.NoError(t, err)
	defer stream.Close()

	require.Zero(t, testing.AllocsPerRun(1000, func() { c.emit(ev) }),
		"emit with a subscriber must not allocate")
	require.Zero(t, stream.Dropped())
}

// TestAuthErrorReason 验证认证器可通过 AuthError 指定 CONNACK 原因码。
func TestAuthErrorReason(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithAuthHandler(&AuthHandlerFunc{
		ConnectFunc: func(ctx *AuthConnectContext) error {
			if ctx.ClientID == "banned" {
				return &AuthError{Reason: packet.ReasonBanned}
			}
			return &AuthError{Reason: packet.ReasonBadAuthMethodOrAuthData}
		},
	}))
	defer c.Stop()

	addr := testServe(t, c)

	readConnack := func(clientID string) packet.ReasonCode {
		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		connect := packet.NewConnectPacket()
		connect.ClientID = clientID
		require.NoError(t, packet.WritePacket(conn, connect, 0))

		require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
		pkt, err := packet.ReadPacket(conn, 0)
		require.NoError(t, err)

		connack, ok := pkt.(*packet.ConnackPacket)
		require.True(t, ok)
		return connack.ReasonCode
	}

	require.Equal(t, packet.ReasonBanned, readConnack("banned"))
	require.Equal(t, packet.ReasonBadAuthMethodOrAuthData, readConnack("other"))
}

// TestDisconnectContextReason 验证断开原因（正常断开、被踢、普通错误）被透出。
func TestDisconnectContextReason(t *testing.T) {
	reasons := make(chan packet.ReasonCode, 8)

	c := testSetupCore(t, NewCoreOptions().WithConnectionHandler(&ConnectionHandlerFunc{
		DisconnectFunc: func(ctx *DisconnectContext) {
			reasons <- ctx.Reason
		},
	}))
	defer c.Stop()

	addr := testServe(t, c)

	// 正常断开
	normal := testSetupClient(t, addr, "dc-normal", nil)
	require.NoError(t, normal.Disconnect())
	select {
	case reason := <-reasons:
		require.Equal(t, packet.ReasonNormalDisconnect, reason)
	case <-time.After(3 * time.Second):
		t.Fatal("no disconnect hook call")
	}
	_ = normal.Close()

	// 被管理员踢下线
	kicked := testSetupClient(t, addr, "dc-kicked", nil)
	defer kicked.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "connected", func() bool {
		return c.IsClientOnline("dc-kicked")
	}))
	require.NoError(t, c.DisconnectClientAndWait(context.Background(), "dc-kicked", packet.ReasonBanned))

	select {
	case reason := <-reasons:
		require.Equal(t, packet.ReasonBanned, reason)
	case <-time.After(3 * time.Second):
		t.Fatal("no disconnect hook call for kicked client")
	}
}

// TestDisconnectClientAndWait 验证等待语义与异常路径。
func TestDisconnectClientAndWait(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	// 会话保留 60s：断开后客户端仍在 clients map 中，便于验证幂等路径。
	cli := testSetupClient(t, addr, "wait-client", client.NewClientOptions().WithSessionTimeout(60))
	defer cli.Close()

	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "connected", func() bool {
		return c.IsClientOnline("wait-client")
	}))

	require.NoError(t, c.DisconnectClientAndWait(context.Background(), "wait-client", packet.ReasonNormalDisconnect))
	require.False(t, c.IsClientOnline("wait-client"))

	// 已断开的客户端：幂等返回 nil
	require.NoError(t, c.DisconnectClientAndWait(context.Background(), "wait-client", packet.ReasonNormalDisconnect))

	// 不存在
	require.ErrorIs(t, c.DisconnectClientAndWait(context.Background(), "nobody", packet.ReasonNormalDisconnect),
		&ClientNotFoundError{})
}

func typesOf(events []Event) []EventType {
	out := make([]EventType, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}

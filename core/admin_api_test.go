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
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	require.True(t, c.Ready())

	c.SetBuildInfo("1.2.3", "abc123", "2026-01-01T00:00:00Z")

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

	// 保留消息
	res, err = c.Publish("api/retained", []byte("keep"), PublishOptions{Retain: true})
	require.NoError(t, err)
	require.True(t, res.Retained)
	require.Len(t, c.GetRetainedMessages("api/retained"), 1)

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

// TestListClientsPage 验证分页、前缀过滤与在线过滤。
func TestListClientsPage(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	for i := 0; i < 5; i++ {
		cli := testSetupClient(t, addr, fmt.Sprintf("page-%02d", i), nil)
		defer cli.Close()
	}
	// 一个离线会话
	off := testSetupClient(t, addr, "page-offline", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, off.Disconnect())
	defer off.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline recorded", func() bool {
		return c.isOfflineSession("page-offline")
	}))

	seen := make(map[string]bool)
	cursor := ""
	pages := 0
	for {
		infos, page, err := c.ListClientsPage(PageOptions{Limit: 2, Cursor: cursor})
		require.NoError(t, err)
		pages++
		for _, info := range infos {
			require.False(t, seen[info.ClientID], "duplicate entry %s", info.ClientID)
			seen[info.ClientID] = true
		}
		if page.NextCursor == "" {
			break
		}
		require.Equal(t, 2, len(infos))
		cursor = page.NextCursor
		require.Less(t, pages, 10)
	}
	require.Equal(t, 6, len(seen))
	require.True(t, seen["page-offline"])

	// 在线过滤
	online := true
	infos, _, err := c.ListClientsPage(PageOptions{Limit: 100, Online: &online})
	require.NoError(t, err)
	require.Len(t, infos, 5)

	offline := false
	infos, _, err = c.ListClientsPage(PageOptions{Limit: 100, Online: &offline})
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "page-offline", infos[0].ClientID)
	require.False(t, infos[0].ExpiresAt.IsZero())
	require.False(t, infos[0].LastSeenAt.IsZero())

	// 前缀过滤
	infos, _, err = c.ListClientsPage(PageOptions{Limit: 100, Prefix: "page-0"})
	require.NoError(t, err)
	require.Len(t, infos, 5)
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

// TestListSessionsAndDeleteSession 验证会话语义与删除。
func TestListSessionsAndDeleteSession(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	cli := testSetupClient(t, addr, "sess-1", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, cli.Subscribe("sess/topic"))
	require.NoError(t, cli.Disconnect())
	defer cli.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline recorded", func() bool {
		return c.isOfflineSession("sess-1")
	}))

	sessions, _, err := c.ListSessions(PageOptions{})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, "sess-1", sessions[0].ClientID)
	require.False(t, sessions[0].Online)
	require.False(t, sessions[0].ExpiresAt.IsZero())
	require.Len(t, sessions[0].Subscriptions, 1)

	require.NoError(t, c.DeleteSession("sess-1"))
	_, err = c.GetClientInfo("sess-1")
	require.ErrorIs(t, err, &ClientNotFoundError{})
	require.False(t, c.isOfflineSession("sess-1"))

	sessions, _, err = c.ListSessions(PageOptions{})
	require.NoError(t, err)
	require.Empty(t, sessions)

	// 订阅也应一并清理
	require.Empty(t, c.subTree.matchTopic("sess/topic"))

	require.ErrorIs(t, c.DeleteSession("nobody"), &ClientNotFoundError{})
}

// TestDeleteSessionOnline 验证删除在线会话会先踢下线并同步在线计数。
func TestDeleteSessionOnline(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "sess-online", client.NewClientOptions().WithSessionTimeout(60))
	defer cli.Close()

	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "client connected", func() bool {
		return c.IsClientOnline("sess-online")
	}))
	before := c.GetStats().ClientsConnected

	require.NoError(t, c.DeleteSession("sess-online"))
	require.False(t, c.IsClientOnline("sess-online"))
	require.Equal(t, before-1, c.GetStats().ClientsConnected)
	require.False(t, c.isOfflineSession("sess-online"))
}

// TestRetainedListAndDelete 验证保留消息元数据列表与删除。
func TestRetainedListAndDelete(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	watcher := testSetupClient(t, addr, "retained-watcher", nil)
	defer watcher.Close()
	require.NoError(t, watcher.Subscribe("r/**"))

	_, err := c.Publish("r/a", []byte("aaa"), PublishOptions{Retain: true})
	require.NoError(t, err)
	_, err = c.Publish("r/b", []byte("bbbb"), PublishOptions{Retain: true, QoS: packet.QoS1})
	require.NoError(t, err)

	// 先消费掉实时投递，避免干扰后续断言
	for {
		if _, err := watcher.PollMessage(context.Background(), 200*time.Millisecond); err != nil {
			break
		}
	}

	infos, page, err := c.ListRetainedPage(PageOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "r/a", infos[0].Topic)
	require.Equal(t, 3, infos[0].PayloadSize)
	require.Equal(t, packet.QoS0, infos[0].QoS)
	require.False(t, infos[0].UpdatedAt.IsZero())
	require.NotEmpty(t, page.NextCursor)

	infos, _, err = c.ListRetainedPage(PageOptions{Limit: 10, Cursor: page.NextCursor})
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "r/b", infos[0].Topic)
	require.Equal(t, 4, infos[0].PayloadSize)
	require.Equal(t, packet.QoS1, infos[0].QoS)

	// 前缀过滤
	infos, _, err = c.ListRetainedPage(PageOptions{Prefix: "r/"})
	require.NoError(t, err)
	require.Len(t, infos, 2)

	require.NoError(t, c.DeleteRetained("r/a"))
	require.Empty(t, c.GetRetainedMessages("r/a"))
	require.Len(t, c.GetRetainedMessages("r/b"), 1)

	// 幂等
	require.NoError(t, c.DeleteRetained("r/a"))
	require.ErrorIs(t, c.DeleteRetained("bad/*"), ErrInvalidTopic)

	// 不存在的保留消息不匹配
	topics, _, err := c.ListTopics(PageOptions{})
	require.NoError(t, err)
	found := map[string]bool{}
	for _, tp := range topics {
		found[tp.Topic] = true
	}
	require.True(t, found["r/b"])
	require.True(t, found["r/**"])
	require.False(t, found["r/a"])
}

// TestListSubscriptionsAndTopics 验证订阅与主题枚举。
func TestListSubscriptionsAndTopics(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	a := testSetupClient(t, addr, "list-a", nil)
	defer a.Close()
	b := testSetupClient(t, addr, "list-b", nil)
	defer b.Close()

	require.NoError(t, a.Subscribe("list/x"))
	require.NoError(t, b.SubscribeWithOptions([]string{"list/x", "list/y"}, client.NewSubscribeOptions().WithQoS(packet.QoS1)))

	subs, _, err := c.ListSubscriptions(PageOptions{})
	require.NoError(t, err)
	require.Len(t, subs, 3)
	require.Equal(t, "list/x", subs[0].Topic)
	require.Equal(t, "list-a", subs[0].ClientID)
	require.Equal(t, "list/x", subs[1].Topic)
	require.Equal(t, "list-b", subs[1].ClientID)
	require.Equal(t, packet.QoS1, subs[1].QoS)

	subs, _, err = c.ListSubscriptions(PageOptions{Prefix: "list/y"})
	require.NoError(t, err)
	require.Len(t, subs, 1)

	topics, _, err := c.ListTopics(PageOptions{})
	require.NoError(t, err)
	require.Len(t, topics, 2)
	require.Equal(t, "list/x", topics[0].Topic)
	require.Equal(t, 2, topics[0].SubscriberCount)
	require.Equal(t, "list/y", topics[1].Topic)
	require.Equal(t, 1, topics[1].SubscriberCount)
}

// TestListPendingAck 验证在途 QoS1 消息枚举。
func TestListPendingAck(t *testing.T) {
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
		infos, err := c.ListPendingAck("ack-sub")
		return err == nil && len(infos) == 1
	}))

	infos, err := c.ListPendingAck("ack-sub")
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, "ack/topic", infos[0].Topic)
	require.Equal(t, packet.QoS1, infos[0].QoS)
	require.False(t, infos[0].SentAt.IsZero())
	require.GreaterOrEqual(t, infos[0].Age, time.Duration(0))

	_, err = c.ListPendingAck("nobody")
	require.ErrorIs(t, err, &ClientNotFoundError{})
}

// TestPurgeClientQueue 验证清空积压。
func TestPurgeClientQueue(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "purge-client", client.NewClientOptions().WithSessionTimeout(60))
	require.NoError(t, cli.Subscribe("purge/topic"))
	require.NoError(t, cli.Disconnect())
	defer cli.Close()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "offline recorded", func() bool {
		return c.isOfflineSession("purge-client")
	}))

	for i := 0; i < 3; i++ {
		_, err := c.Publish("purge/topic", []byte("x"), PublishOptions{})
		require.NoError(t, err)
	}
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "queued", func() bool {
		info, err := c.GetClientInfo("purge-client")
		return err == nil && info.QueueSize >= 3
	}))

	n, err := c.PurgeClientQueue("purge-client")
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, int64(3))

	info, err := c.GetClientInfo("purge-client")
	require.NoError(t, err)
	require.Equal(t, int64(0), info.QueueSize)

	_, err = c.PurgeClientQueue("nobody")
	require.ErrorIs(t, err, &ClientNotFoundError{})
}

// TestSubscribeEvents 验证事件流的类型、顺序与过滤。
func TestSubscribeEvents(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.SubscribeEvents(ctx, EventFilter{Buffer: 256})
	require.NoError(t, err)

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
		case ev := <-events:
			got = append(got, ev)
		case <-timeout:
			t.Fatalf("timeout waiting for events, got %v", typesOf(got))
		}
	}

	for i, wantType := range want {
		require.Equal(t, wantType, got[i].Type, "event %d", i)
		require.NotZero(t, got[i].Seq)
		require.False(t, got[i].Time.IsZero())
		if i > 0 {
			require.Greater(t, got[i].Seq, got[i-1].Seq, "seq must be monotonic")
		}
	}
	require.Equal(t, "event-client", got[0].ClientID)
	require.Equal(t, "event/topic", got[1].Topic)
	require.Equal(t, packet.ReasonNormalDisconnect.String(), got[3].Reason)
}

// TestSubscribeEventsFilterAndDrop 验证过滤器与慢消费者丢弃计数。
func TestSubscribeEventsFilterAndDrop(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	only, err := c.SubscribeEvents(ctx, EventFilter{Types: []EventType{EventAdminAction}, Buffer: 1})
	require.NoError(t, err)

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "drop-client", nil)
	defer cli.Close()

	// 大量非 admin 事件不应进入通道
	for i := 0; i < 50; i++ {
		_, err := c.Publish("drop/topic", []byte("x"), PublishOptions{})
		require.NoError(t, err)
	}
	select {
	case ev := <-only:
		t.Fatalf("unexpected event %v", ev.Type)
	case <-time.After(100 * time.Millisecond):
	}

	// 两个 admin 事件，缓冲只有 1 条：第二条必须被丢弃并计数
	_, err = c.Publish("drop/retained", []byte("x"), PublishOptions{Retain: true})
	require.NoError(t, err)
	require.NoError(t, c.DeleteRetained("drop/retained"))
	_, err = c.Publish("drop/topic", []byte("x"), PublishOptions{PublisherID: "admin"})
	require.NoError(t, err)

	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "events dropped", func() bool {
		return c.GetStats().EventsDropped > 0
	}))

	ev := <-only
	require.Equal(t, EventAdminAction, ev.Type)

	// 取消订阅后通道关闭
	cancel()
	require.True(t, eventually(t, 2*time.Second, 10*time.Millisecond, "event channel closed", func() bool {
		select {
		case _, ok := <-only:
			return !ok
		default:
			return false
		}
	}))
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

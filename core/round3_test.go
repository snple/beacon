package core

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
)

func round3Core(t *testing.T, opts *CoreOptions) *Core {
	t.Helper()
	if opts == nil {
		opts = NewCoreOptions()
	}
	opts.WithLogger(testIntegrationLogger())

	c, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	return c
}

// TestCoreCannotRestartAfterStop 确认 Stop 之后同一实例不能再 Start。
// 否则会得到一个 ctx 已取消、存储已关闭但 running=true 的"僵尸"实例。
func TestCoreCannotRestartAfterStop(t *testing.T) {
	c := round3Core(t, nil)
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if err := c.Start(); !errors.Is(err, ErrCoreStopped) {
		t.Fatalf("expected ErrCoreStopped, got %v", err)
	}

	// 重复 Stop 必须幂等
	if err := c.Stop(); err != nil {
		t.Fatalf("second stop: %v", err)
	}
}

// TestStopBeforeStartClosesStore 确认未 Start 就 Stop 也会关闭存储，
// 否则 badger 句柄与目录锁会泄漏。
func TestStopBeforeStartClosesStore(t *testing.T) {
	c := round3Core(t, nil)
	if err := c.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	err := c.store.db().View(func(txn *badger.Txn) error { return nil })
	if !errors.Is(err, badger.ErrDBClosed) {
		t.Fatalf("expected store to be closed, got %v", err)
	}
}

// TestPreConnectHandshakeLimit 确认认证前（CONNECT 尚未发送）的连接数受
// MaxHandshakeConnections 限制：超限连接收到 CONNACK(ServerBusy) 并被关闭。
func TestPreConnectHandshakeLimit(t *testing.T) {
	c := round3Core(t, NewCoreOptions().
		WithMaxHandshakeConnections(1).
		WithConnectTimeout(5))
	if err := c.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	addr := testServe(t, c)

	// 第一个连接占住唯一名额：只建立 TCP，不发 CONNECT。
	first, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial first: %v", err)
	}
	defer first.Close()

	handshakeCount := func() int {
		c.handshakeConnsMu.Lock()
		defer c.handshakeConnsMu.Unlock()
		return len(c.handshakeConns)
	}
	if !eventually(t, 2*time.Second, 10*time.Millisecond, "first connection tracked", func() bool {
		return handshakeCount() == 1
	}) {
		t.Fatalf("server did not track the handshaking connection")
	}

	second, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial second: %v", err)
	}
	defer second.Close()

	_ = second.SetReadDeadline(time.Now().Add(3 * time.Second))
	pkt, err := packet.ReadPacket(second, 0)
	if err != nil {
		t.Fatalf("expected CONNACK rejection, got %v", err)
	}
	connack, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", pkt)
	}
	if connack.ReasonCode != packet.ReasonServerBusy {
		t.Fatalf("expected ServerBusy, got 0x%02X", uint8(connack.ReasonCode))
	}
}

// TestExpiredSessionIsNotResumed 确认会话过期后、清理协程尚未运行时重连，
// 服务端返回 SessionPresent=false（不能把过期会话当成有效会话恢复）。
func TestExpiredSessionIsNotResumed(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	cli := testSetupClient(t, addr, "expiring-client", client.NewClientOptions().WithSessionTimeout(1))
	defer cli.Close()

	// 建立一次会话（离开时服务端会保留 1 秒）
	if err := cli.Subscribe("expiring/topic"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := cli.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	// 超过会话超时，但远小于清理周期（默认 180s），会话记录仍留在内存中。
	time.Sleep(1200 * time.Millisecond)

	if err := cli.ConnectWithDialer(&client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if cli.SessionPresent() {
		t.Fatal("expired session must not be resumed")
	}
}

// TestInvalidRetainHandlingRejected 确认 RetainHandling=3 被显式拒绝，
// 且被拒绝的订阅不会留在客户端本地订阅表中。
func TestInvalidRetainHandlingRejected(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "bad-retain", nil)
	defer cli.Close()

	err := cli.SubscribeWithOptions([]string{"bad/retain"},
		client.NewSubscribeOptions().WithQoS(packet.QoS1).WithRetainHandling(3))
	if err == nil {
		t.Fatal("expected subscribe to fail for RetainHandling=3")
	}

	var subErr *client.SubscriptionError
	if !errors.As(err, &subErr) {
		t.Fatalf("expected SubscriptionError, got %T: %v", err, err)
	}
	if len(subErr.Codes) != 1 || subErr.Codes[0] != packet.ReasonProtocolError {
		t.Fatalf("expected ProtocolError reason code, got %v", subErr.Codes)
	}
	if cli.HasSubscription("bad/retain") {
		t.Fatal("rejected subscription must not be recorded")
	}
}

// TestDuplicateTopicInOneSubscribeReplaysRetainedOnce 确认同一 SUBSCRIBE 包内的
// 重复过滤器只重放一次保留消息，同时仍逐条返回原因码。
func TestDuplicateTopicInOneSubscribeReplaysRetainedOnce(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	// 直接写入保留消息（不经过分发队列），避免"发布分发"与"订阅"并发导致
	// 订阅者额外收到一条实时投递，从而无法判断保留消息是否被重放两次。
	retainedPacket := packet.NewPublishPacket("dup/topic", []byte("retained"))
	retainedPacket.Retain = true
	retained := c.newPublishMessage(retainedPacket)
	c.handleRetainedMessage(&retained)

	sub := testSetupClient(t, addr, "dup-subscriber", nil)
	defer sub.Close()

	if err := sub.SubscribeWithOptions([]string{"dup/topic", "dup/topic"},
		client.NewSubscribeOptions().WithQoS(packet.QoS0)); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	ctx := context.Background()
	received := 0
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		msg, err := sub.PollMessage(ctx, 300*time.Millisecond)
		if err != nil {
			break
		}
		if msg.Topic() == "dup/topic" {
			received++
		}
	}
	if received != 1 {
		t.Fatalf("expected 1 retained replay, got %d", received)
	}
}

// TestClientReplaysSubscriptionOptions 确认会话未恢复时客户端按原始选项重放订阅，
// 而不是退回默认 QoS0。CleanStart 保证每次连接都没有会话。
func TestClientReplaysSubscriptionOptions(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "replay-client",
		client.NewClientOptions().WithCleanStart(true))
	defer cli.Close()

	opts := client.NewSubscribeOptions().
		WithQoS(packet.QoS1).
		WithNoLocal(true).
		WithRetainAsPublished(true).
		WithRetainHandling(packet.RetainHandlingNever)
	if err := cli.SubscribeWithOptions([]string{"replay/topic"}, opts); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// 手动断开重连：服务端因 CleanStart 丢弃会话，客户端必须重放订阅。
	if err := cli.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if err := cli.ConnectWithDialer(&client.TCPDialer{Address: addr, DialTimeout: 10 * time.Second}); err != nil {
		t.Fatalf("reconnect: %v", err)
	}
	if cli.SessionPresent() {
		t.Fatal("CleanStart connection must not resume a session")
	}

	var got packet.SubscribeOptions
	var ok bool
	if !eventually(t, 2*time.Second, 10*time.Millisecond, "subscription replayed", func() bool {
		c.clientsMu.RLock()
		svClient := c.clients["replay-client"]
		c.clientsMu.RUnlock()
		if svClient == nil {
			return false
		}
		svClient.session.subsMu.RLock()
		got, ok = svClient.session.subscriptions["replay/topic"]
		svClient.session.subsMu.RUnlock()
		return ok
	}) {
		t.Fatal("client did not replay its subscription")
	}

	if got.QoS != packet.QoS1 || !got.NoLocal || !got.RetainAsPublished ||
		got.RetainHandling != packet.RetainHandlingNever {
		t.Fatalf("subscription options not preserved: %+v", got)
	}
}

// TestProgrammaticSubscribeIsPersisted 确认编程接口 Core.Subscribe 会同步会话状态，
// 并在 core 重启后从持久化会话中恢复。
func TestProgrammaticSubscribeIsPersisted(t *testing.T) {
	dir := t.TempDir()
	newOpts := func() *CoreOptions {
		return NewCoreOptions().
			WithStoreDir(dir).
			WithLogger(testIntegrationLogger()).
			WithConnectTimeout(5)
	}

	first := round3Core(t, newOpts())
	if err := first.Start(); err != nil {
		t.Fatalf("start first: %v", err)
	}
	addr := testServe(t, first)

	cli := testSetupClient(t, addr, "api-client", client.NewClientOptions().WithSessionTimeout(300))
	if err := cli.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	first.Subscribe("api-client", packet.Subscription{
		Topic:   "api/topic",
		Options: packet.SubscribeOptions{QoS: packet.QoS1},
	})

	first.clientsMu.RLock()
	svClient := first.clients["api-client"]
	first.clientsMu.RUnlock()
	if svClient == nil {
		t.Fatal("offline session should still exist")
	}
	svClient.session.subsMu.RLock()
	opts, ok := svClient.session.subscriptions["api/topic"]
	svClient.session.subsMu.RUnlock()
	if !ok || opts.QoS != packet.QoS1 {
		t.Fatalf("programmatic subscribe must update session state, got %+v (ok=%v)", opts, ok)
	}

	if err := first.Stop(); err != nil {
		t.Fatalf("stop first: %v", err)
	}
	_ = cli.Close()

	second := round3Core(t, newOpts())
	if err := second.Start(); err != nil {
		t.Fatalf("start second: %v", err)
	}
	defer second.Stop()

	matches := second.subTree.matchTopic("api/topic")
	restored, ok := matches["api-client"]
	if !ok || restored.QoS != packet.QoS1 {
		t.Fatalf("subscription not restored after restart: %+v (ok=%v)", restored, ok)
	}
}

// TestTakeoverDoesNotPublishWill 确认会话接管（同 ClientID 新连接替换旧连接）
// 不发布旧连接的遗嘱：接管视为客户端主动行为。
func TestTakeoverDoesNotPublishWill(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	watcher := testSetupClient(t, addr, "will-watcher", nil)
	defer watcher.Close()
	if err := watcher.Subscribe("will/takeover"); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// 第一个连接带遗嘱。用 ConnectWithConn 关闭自动重连，避免被踢后抢回会话。
	netConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	first, err := client.NewWithOptions(client.NewClientOptions().
		WithClientID("takeover-client").
		WithLogger(testIntegrationLogger()).
		WithWillSimple("will/takeover", []byte("boom"), packet.QoS0, false))
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	defer first.Close()
	if err := first.ConnectWithConn(netConn); err != nil {
		t.Fatalf("connect first: %v", err)
	}

	// 同 ClientID 的第二个连接接管会话
	second := testSetupClient(t, addr, "takeover-client", nil)
	defer second.Close()

	if !eventually(t, 2*time.Second, 20*time.Millisecond, "session taken over", func() bool {
		return !first.IsConnected()
	}) {
		t.Fatal("first connection was not taken over")
	}

	// 遗嘱不应发布
	ctx := context.Background()
	if _, err := watcher.PollMessage(ctx, 500*time.Millisecond); err == nil {
		t.Fatal("takeover must not publish the old connection's will")
	}
}

// TestCleanStartClearsOfflineSessionEntry 确认 CleanStart 丢弃旧会话时，
// 离线会话表里的过期时间戳也一并清除（否则后续会误判会话已过期）。
func TestCleanStartClearsOfflineSessionEntry(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)

	first := testSetupClient(t, addr, "clean-client", client.NewClientOptions().WithSessionTimeout(300))
	if err := first.Disconnect(); err != nil {
		t.Fatalf("disconnect: %v", err)
	}

	hasOfflineEntry := func() bool {
		c.offlineSessionsMu.Lock()
		defer c.offlineSessionsMu.Unlock()
		_, ok := c.offlineSessions["clean-client"]
		return ok
	}
	if !eventually(t, 2*time.Second, 10*time.Millisecond, "offline session recorded", hasOfflineEntry) {
		t.Fatal("expected an offline session entry after disconnect")
	}

	second := testSetupClient(t, addr, "clean-client",
		client.NewClientOptions().WithSessionTimeout(300).WithCleanStart(true))
	defer second.Close()
	_ = first.Close()

	if hasOfflineEntry() {
		t.Fatal("CleanStart must remove the stale offline session entry")
	}
}

// TestAutoReconnectReplaysSubscriptionOptions 确认自动重连路径同样会按原始选项重放订阅
// （CleanStart 保证重连后会话一定不存在）。
func TestAutoReconnectReplaysSubscriptionOptions(t *testing.T) {
	c := testSetupCore(t, nil)
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "auto-replay",
		client.NewClientOptions().WithCleanStart(true))
	defer cli.Close()

	if err := cli.SubscribeWithOptions([]string{"auto/topic"},
		client.NewSubscribeOptions().WithQoS(packet.QoS1)); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// 服务端主动断开，客户端会自动重连（初始退避 1s）。
	if err := c.DisconnectClient("auto-replay", packet.ReasonServerShuttingDown); err != nil {
		t.Fatalf("disconnect client: %v", err)
	}

	got := false
	if !eventually(t, 6*time.Second, 20*time.Millisecond, "subscription replayed after auto reconnect", func() bool {
		if !cli.IsConnected() {
			return false
		}
		c.clientsMu.RLock()
		svClient := c.clients["auto-replay"]
		c.clientsMu.RUnlock()
		if svClient == nil {
			return false
		}
		svClient.session.subsMu.RLock()
		opts, ok := svClient.session.subscriptions["auto/topic"]
		svClient.session.subsMu.RUnlock()
		got = ok && opts.QoS == packet.QoS1
		return got
	}) {
		t.Fatal("subscription not replayed after auto reconnect")
	}
}

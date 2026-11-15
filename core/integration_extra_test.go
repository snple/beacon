package core_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

func newTestClientWithOptions(t *testing.T, addr, id, channels string, opts *client.ClientOptions) *client.Client {
	t.Helper()
	opts = opts.WithCore(addr).WithClientID(id).WithAutoReconnect(false).WithLogger(zap.NewNop())
	if channels != "" {
		opts.WithChannels(channels)
	}
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// 会话总数上限必须覆盖离线保留会话。
func TestMaxSessionsRejected(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxSessions(1))
	c1 := newTestClient(t, addr, "max-1", "ch/a")
	defer func() { _ = c1.Close() }()

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("max-2").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	err = c2.ConnectContext(context.Background())
	if err == nil {
		t.Fatal("second session should be rejected")
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonServerBusy {
		t.Fatalf("expected ServerBusy, got %v", err)
	}
}

// 全局在途字节上限必须阻止无界内存增长。
func TestMaxInflightBytesRejected(t *testing.T) {
	// 上限必须覆盖第一条消息，但放不下两条（Message.Size 含约 160B 固定开销）。
	_, addr := startCore(t, core.NewCoreOptions().WithMaxInflightBytes(250))
	sub := newTestClient(t, addr, "inflight-sub", "ch/a")
	defer func() { _ = sub.Close() }()
	pub := newTestClient(t, addr, "inflight-pub")
	defer func() { _ = pub.Close() }()

	ctx := context.Background()
	if err := pub.Push(ctx, "ch/a", []byte("first"), client.PushOptions{}); err != nil {
		t.Fatalf("first push should be accepted: %v", err)
	}
	err := pub.Push(ctx, "ch/a", []byte("second"), client.PushOptions{})
	if err == nil {
		t.Fatal("second push should exceed the in-flight limit")
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("expected QuotaExceeded, got %v", err)
	}
}

// 重复 Connect 必须显式失败，不能泄漏旧连接或启动多个 supervise。
func TestDuplicateConnectRejected(t *testing.T) {
	_, addr := startCore(t, nil)
	c := newTestClient(t, addr, "dup-connect", "ch/a")
	defer func() { _ = c.Close() }()

	if err := c.ConnectContext(context.Background()); !errors.Is(err, client.ErrAlreadyConnected) {
		t.Fatalf("expected ErrAlreadyConnected, got %v", err)
	}
}

func TestPollMessage(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := pub.Push(ctx, "ch/a", []byte{byte('a' + i)}, client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for i := 0; i < 3; i++ {
		m, err := sub.PollMessage(ctx, 2*time.Second)
		if err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
		if string(m.Payload) != string([]byte{byte('a' + i)}) {
			t.Fatalf("unexpected payload %q", m.Payload)
		}
	}
}

func TestPollMessageTimeout(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")

	start := time.Now()
	_, err := sub.PollMessage(context.Background(), 300*time.Millisecond)
	if !errors.Is(err, client.ErrPollTimeout) {
		t.Fatalf("expected ErrPollTimeout, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout took too long: %v", elapsed)
	}
}

func TestPushSingleRecord(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, m := range []struct {
		channel string
		payload string
	}{
		{"ch/a", "1"},
		{"ch/b", "2"},
		{"other/c", "3"},
	} {
		if err := pub.Push(ctx, m.channel, []byte(m.payload), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{"1", "2"} {
		msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil || string(msg.Payload) != want {
			t.Fatalf("expected %q, got %+v", want, msg)
		}
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPushWithoutAck(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/a", []byte("noack"), client.PushOptions{}.WithoutAck()); err != nil {
		t.Fatal(err)
	}

	// 异步到达，稍等
	deadline := time.Now().Add(2 * time.Second)
	for b.GetStats().MessagesReceived == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "noack" {
		t.Fatalf("expected noack message, got %+v", msg)
	}
}

func TestOfflineQueueDeliveredOnReconnect(t *testing.T) {
	b, addr := startCore(t, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c1 := newTestClient(t, addr, "device-1", "cmd/**")
	if err := c1.Close(); err != nil {
		t.Fatal(err)
	}

	// 等会话解绑
	deadline := time.Now().Add(2 * time.Second)
	for b.GetStats().ClientsConnected != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := b.PushToClient("device-1", "cmd/x", []byte("queued"),
		core.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatalf("publish to offline session: %v", err)
	}

	c2, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("device-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if !c2.SessionPresent() {
		t.Fatal("session should be present after reconnect")
	}

	msg, err := c2.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "queued" {
		t.Fatalf("offline queue not delivered: %+v", msg)
	}
}

func TestWebSocketTransport(t *testing.T) {
	b, err := core.NewWithOptions(core.NewCoreOptions().WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = b.Stop() }()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = b.ServeWSListener(l, "/ws") }()
	time.Sleep(100 * time.Millisecond)

	addr := "ws://" + l.Addr().String() + "/ws"
	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("ws-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Declare(ctx, "ws/**"); err != nil {
		t.Fatal(err)
	}

	pub, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("ws-2").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	if err := pub.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()

	if err := pub.Push(ctx, "ws/hello", []byte("over ws"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := c.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "over ws" {
		t.Fatalf("ws delivery failed: %+v", msg)
	}
}

func TestSessionExpiry(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().
		WithSessionTimeout(1).
		WithSessionCheckInterval(100*time.Millisecond))

	c, err := client.NewWithOptions(client.NewClientOptions().
		WithCore(addr).
		WithClientID("tmp-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	_ = c.Close()

	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := b.GetClientInfo("tmp-1"); err != nil {
			return // 会话已过期并清理
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("session should have expired")
}

func TestDeclareReplacesChannels(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "a/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if _, err := sub.Declare(ctx, "b/**"); err != nil {
		t.Fatal(err)
	}

	if err := pub.Push(ctx, "a/x", []byte("no"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "b/x", []byte("yes"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.Channel != "b/x" {
		t.Fatalf("declare did not replace: %+v", msg)
	}
}

func TestMessageAck(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected 1 message")
	}
	if err := msg.Ack(ctx); err != nil {
		t.Fatal(err)
	}

	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("acked message should not be redelivered, got %+v", again)
	}
}

func TestPriorityHighBeforeMedium(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	medOpt := client.PushOptions{}.WithPriority(packet.PriorityMedium).WithReliable(true)
	highOpt := client.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(true)

	if err := pub.Push(ctx, "ch/a", []byte("med"), medOpt); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/a", []byte("high"), highOpt); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "high" {
		t.Fatalf("expected high priority first, got %+v", msg)
	}
}

// 高优先级消息必须能绕过未确认的中优先级消息，占用第二个投递位。
func TestHighPriorityBypassesUnackedMedium(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	medOpt := client.PushOptions{}.WithPriority(packet.PriorityMedium).WithReliable(true)
	highOpt := client.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(true)

	if err := pub.Push(ctx, "ch/a", []byte("med"), medOpt); err != nil {
		t.Fatal(err)
	}
	med, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if med == nil || string(med.Payload) != "med" {
		t.Fatalf("expected medium message, got %+v", med)
	}
	// 故意不 commit med。

	if err := pub.Push(ctx, "ch/a", []byte("high"), highOpt); err != nil {
		t.Fatal(err)
	}
	high, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if high == nil || string(high.Payload) != "high" || high.Dup {
		t.Fatalf("high priority should bypass unacked medium, got %+v", high)
	}

	info, err := b.GetClientInfo("sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Delivered != 2 {
		t.Fatalf("delivered=%d, want 2 occupied slots", info.Delivered)
	}

	if n, err := sub.CommitAll(ctx); err != nil || n != 2 {
		t.Fatalf("commit: n=%d err=%v", n, err)
	}
	if next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100}); err != nil || next != nil {
		t.Fatalf("expected empty after commit, got %+v err=%v", next, err)
	}
}

func TestOnDeliverRejectsMessage(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Deliver: core.DeliverHandlerFunc(func(ctx *core.DeliverContext) bool {
			return ctx.Channel != "ch/blocked"
		}),
	})
	b, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "sub-1", "ch/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/ok", []byte("keep"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/blocked", []byte("drop"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.Channel != "ch/ok" {
		t.Fatalf("OnDeliver filter failed: %+v", msg)
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	// 第二次拉取会取出 ch/blocked，OnDeliver 拒绝后直接丢弃。
	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("rejected message should be gone, got %+v", again)
	}
	if b.GetStats().MessagesDropped != 1 {
		t.Fatalf("dropped=%d", b.GetStats().MessagesDropped)
	}
}

// 投递钩子对未确认消息的重发同样生效：拒绝后必须从 delivered 移除，
// 否则该消息会永久占用窗口并被反复重发。
func TestOnDeliverRejectsRedelivery(t *testing.T) {
	var calls atomic.Int32
	var sawDup atomic.Bool
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Deliver: core.DeliverHandlerFunc(func(ctx *core.DeliverContext) bool {
			if ctx.Dup {
				sawDup.Store(true)
				return false
			}
			calls.Add(1)
			return true
		}),
	})
	b, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "dup-sub", "dup/**")
	pub := newTestClient(t, addr, "dup-pub")
	ctx := context.Background()

	if err := pub.Push(ctx, "dup/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("first pull: %+v err=%v", msg, err)
	}
	if msg.Dup {
		t.Fatal("first delivery must not be marked dup")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("deliver hook calls = %d, want 1", got)
	}

	// 故意不确认：再次拉取会重发，钩子拒绝后消息应被丢弃。
	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("rejected redelivery should be dropped, got %+v", again)
	}
	if !sawDup.Load() {
		t.Fatal("deliver hook should observe the redelivery")
	}
	// 已被丢弃的消息在服务端是 unknown；客户端提交必须幂等成功。
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatalf("commit after hook drop: %v", err)
	}
	if got := b.GetStats().MessagesDropped; got != 1 {
		t.Fatalf("dropped=%d, want 1", got)
	}
	// 窗口已释放：后续消息可以继续投递。
	if err := pub.Push(ctx, "dup/a", []byte("w"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || next == nil || string(next.Payload) != "w" {
		t.Fatalf("window should be free after hook drop, got %+v err=%v", next, err)
	}
}

// 回归：队列满丢弃新消息时不能重复释放引用，否则在途计数会变成负数。
func TestQueueFullDoesNotLeakInflight(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(1))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	res := b.Push("ch/a", []byte("high"), core.PushOptions{}.WithPriority(packet.PriorityHigh))
	if res.Matched != 1 {
		t.Fatalf("first push should match: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after first push = %d, want 1", got)
	}

	// 队列已满且新消息优先级更低 → 新消息被丢弃
	res = b.Push("ch/a", []byte("low"), core.PushOptions{}.WithPriority(packet.PriorityLow))
	if res.Matched != 0 || res.Dropped != 1 {
		t.Fatalf("low-priority push should be dropped: %+v", res)
	}
	if got := b.GetStats().InFlightMessages; got != 1 {
		t.Fatalf("inflight after drop = %d, want 1 (no double release)", got)
	}
	if got := b.GetStats().InFlightBytes; got <= 0 {
		t.Fatalf("inflight bytes = %d", got)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "high" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	if got := b.GetStats().InFlightMessages; got != 0 {
		t.Fatalf("inflight after commit = %d, want 0", got)
	}
	if got := b.GetStats().InFlightBytes; got != 0 {
		t.Fatalf("inflight bytes after commit = %d, want 0", got)
	}
}

func TestAdminPushDefaultsToMedium(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	b.Push("ch/a", []byte("v"), core.PushOptions{})

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil {
		t.Fatal("expected 1 message")
	}
	if msg.Priority != packet.PriorityMedium {
		t.Fatalf("default priority = %v, want medium", msg.Priority)
	}
	if msg.NeedsCommit() {
		t.Fatal("messages must default to unreliable")
	}
	if err := msg.Ack(ctx); err != nil {
		t.Fatal(err)
	}

	b.Push("ch/a", []byte("r"), core.PushOptions{}.WithReliable(true))
	reliable, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if reliable == nil || !reliable.NeedsCommit() {
		t.Fatalf("explicit reliable push should require commit: %+v", reliable)
	}
}

func TestAdminPushExplicitLow(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	b.Push("ch/a", []byte("v"), core.PushOptions{}.WithPriority(packet.PriorityLow))

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.NeedsCommit() {
		t.Fatalf("explicit low priority should not need commit: %+v", msg)
	}
}

func TestAdminPushInvalidChannel(t *testing.T) {
	b, _ := startCore(t, nil)
	if _, err := b.PushToClient("x", "bad/*/channel", nil, core.PushOptions{}); err == nil {
		t.Fatal("expected error for invalid channel")
	}
}

// 回归：payload 在推送后被调用方修改，不应影响已入队消息。
func TestAdminPushCopiesPayload(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	ctx := context.Background()

	payload := []byte("original")
	b.Push("ch/a", payload, core.PushOptions{})
	copy(payload, "MODIFIED")

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "original" {
		t.Fatalf("payload not copied: %+v", msg)
	}
}

// channel 名/过滤器的长度上限必须按服务端配置生效。
func TestChannelLengthLimitEnforced(t *testing.T) {
	ctx := context.Background()

	t.Run("default", func(t *testing.T) {
		_, addr := startCore(t, nil)
		c := newTestClient(t, addr, "len-default")

		assertPushChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength), true)
		assertPushChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength+1), false)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength), packet.ReasonSuccess)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", packet.MinChannelLength+1), packet.ReasonChannelFilterInvalid)
	})

	t.Run("configured", func(t *testing.T) {
		b, addr := startCore(t, core.NewCoreOptions().WithMaxChannelLength(512))
		if got := b.Info().MaxChannelLength; got != 512 {
			t.Fatalf("Info().MaxChannelLength=%d, want 512", got)
		}
		c := newTestClient(t, addr, "len-512")

		assertPushChannel(t, c, ctx, strings.Repeat("a", 512), true)
		assertPushChannel(t, c, ctx, strings.Repeat("a", 513), false)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", 512), packet.ReasonSuccess)
		assertDeclareChannel(t, c, ctx, strings.Repeat("a", 513), packet.ReasonChannelFilterInvalid)
	})
}

func assertPushChannel(t *testing.T, c *client.Client, ctx context.Context, channel string, wantOK bool) {
	t.Helper()
	err := c.Push(ctx, channel, nil, client.PushOptions{})
	if wantOK {
		// 没有订阅者时返回 NoMatch（非错误），说明 channel 本身已通过校验。
		if err != nil {
			t.Fatalf("channel len %d should be accepted: %v", len(channel), err)
		}
		return
	}
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonChannelInvalid {
		t.Fatalf("channel len %d: got %v, want ChannelInvalid", len(channel), err)
	}
}

func assertDeclareChannel(t *testing.T, c *client.Client, ctx context.Context, filter string, want packet.ReasonCode) {
	t.Helper()
	res, err := c.Declare(ctx, filter)
	if err != nil {
		t.Fatalf("declare len %d: %v", len(filter), err)
	}
	// DECLARE_RESP 只回显失败项；接受时结果为空。
	if want == packet.ReasonSuccess {
		if len(res) != 0 {
			t.Fatalf("declare len %d: expected no failures, got %+v", len(filter), res)
		}
		return
	}
	if len(res) != 1 || res[0].Reason != want {
		t.Fatalf("declare len %d: got %+v, want reason %s", len(filter), res, want)
	}
}

// 离线会话保留全部可靠消息；不可靠消息直接丢弃。队列满后沿用既有淘汰策略。
func TestOfflineQueuePolicy(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(2))
	sub := newTestClient(t, addr, "offline-sub", "off/**")
	pub := newTestClient(t, addr, "offline-pub")
	ctx := context.Background()

	_ = sub.Close()
	waitClientDetached(t, b, "offline-sub")

	push := func(channel string, prio packet.Priority, reliable bool) {
		t.Helper()
		opts := client.PushOptions{}.WithPriority(prio).WithReliable(reliable)
		if err := pub.Push(ctx, channel, []byte(channel), opts); err != nil {
			t.Fatalf("push %s: %v", channel, err)
		}
	}

	push("off/m1", packet.PriorityMedium, true)
	push("off/h1", packet.PriorityHigh, true)
	// 不可靠消息在离线时被丢弃，不占用队列。
	push("off/l1", packet.PriorityLow, false)
	// 队列已满（cap=2）：挤掉最旧的 Medium。
	push("off/m2", packet.PriorityMedium, true)

	info, err := b.GetClientInfo("offline-sub")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pending != [3]uint32{0, 1, 1} {
		t.Fatalf("pending=%v, want [0 1 1] (Low dropped, oldest Medium evicted)", info.Pending)
	}
	if info.MessagesDropped != 2 {
		t.Fatalf("MessagesDropped=%d, want 2 (1 Low skipped + 1 evicted)", info.MessagesDropped)
	}

	// 非 CleanSession 重连：复用会话并取回 High → Medium。
	sub2 := newTestClient(t, addr, "offline-sub")
	if !sub2.SessionPresent() {
		t.Fatal("session should be present on reconnect")
	}
	for _, want := range []string{"off/h1", "off/m2"} {
		msg, err := sub2.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil || msg.Channel != want {
			t.Fatalf("offline queue = %+v, want %s", msg, want)
		}
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

// CleanSession 必须丢弃既有 pending 并返回 SessionPresent=false。
func TestCleanSessionClearsState(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "clean-1", "c/**")
	pub := newTestClient(t, addr, "clean-pub")
	ctx := context.Background()

	_ = sub.Close()
	waitClientDetached(t, b, "clean-1")

	if err := pub.Push(ctx, "c/x", []byte("stale"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	clean := client.NewClientOptions().
		WithCore(addr).
		WithClientID("clean-1").
		WithCleanSession(true).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(clean)
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if c2.SessionPresent() {
		t.Fatal("CleanSession should report SessionPresent=false")
	}
	msg, err := c2.Pull(ctx, client.PullOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("CleanSession should clear pending, got %+v", msg)
	}
}

// CleanSession 在旧连接仍存活时同样生效（接管 + 清理）。
func TestCleanSessionTakeover(t *testing.T) {
	b, addr := startCore(t, nil)
	old := newTestClient(t, addr, "clean-2", "c/**")
	pub := newTestClient(t, addr, "clean-pub2")
	ctx := context.Background()

	if err := pub.Push(ctx, "c/1", []byte("stale"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	// 先确认旧会话确实收到了积压。
	if info, err := b.GetClientInfo("clean-2"); err != nil || info.Pending[1] != 1 {
		t.Fatalf("pending before clean = %+v, err=%v", info, err)
	}

	clean := client.NewClientOptions().
		WithCore(addr).
		WithClientID("clean-2").
		WithCleanSession(true).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c2, err := client.NewWithOptions(clean)
	if err != nil {
		t.Fatal(err)
	}
	if err := c2.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.Close() }()

	if c2.SessionPresent() {
		t.Fatal("CleanSession takeover should report SessionPresent=false")
	}
	msg, err := c2.Pull(ctx, client.PullOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("CleanSession takeover should clear pending, got %+v", msg)
	}
	if old.Connected() {
		t.Fatal("old connection should be taken over")
	}
}

// 服务端分配 ClientID 时，重连必须沿用同一个 ID 并恢复会话。
func TestAutoClientIDReconnectResumesSession(t *testing.T) {
	b, addr := startCore(t, nil)

	opts := client.NewClientOptions().
		WithCore(addr).
		WithAutoReconnect(true).
		WithChannels("auto/**").
		WithLogger(zap.NewNop())
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	id := c.ClientID()
	if id == "" {
		t.Fatal("server should assign a client id")
	}
	if c.SessionPresent() {
		t.Fatal("first connect must not reuse a session")
	}

	pub := newTestClient(t, addr, "auto-pub")
	if err := pub.Push(ctx, "auto/x", []byte("queued"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}

	if err := b.DisconnectClient(id, packet.ReasonServerShuttingDown); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if c.Connected() && c.SessionPresent() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !c.Connected() {
		t.Fatal("client did not reconnect")
	}
	if c.ClientID() != id {
		t.Fatalf("assigned client id changed across reconnect: %s -> %s", id, c.ClientID())
	}
	if !c.SessionPresent() {
		t.Fatal("reconnect should resume the same session")
	}

	msg, err := c.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "queued" {
		t.Fatalf("pending message lost across reconnect: %+v", msg)
	}
}

// UncommittedTimeout 必须从重连时刻重新计时，不能把离线时间算作消费卡住。
func TestUncommittedTimeoutRestartsOnReconnect(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().
		WithSessionCheckInterval(50*time.Millisecond).
		WithUncommittedTimeout(800*time.Millisecond))

	sub := newTestClient(t, addr, "unc-1", "ch/a")
	pub := newTestClient(t, addr, "unc-pub")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := pub.Push(ctx, "ch/a", []byte("x"),
		client.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil || msg == nil {
		t.Fatalf("pull: msg=%+v err=%v", msg, err)
	}
	_ = sub.Close()
	waitClientDetached(t, b, "unc-1")

	// 离线时间超过 UncommittedTimeout。
	time.Sleep(1200 * time.Millisecond)

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("unc-1").
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !c.SessionPresent() {
		t.Fatal("session should be resumed")
	}

	// 重连后计时重新开始：短时间内不应被踢。
	time.Sleep(200 * time.Millisecond)
	if !c.Connected() {
		t.Fatal("resumed session must not be kicked immediately")
	}

	// 但持续不 commit 仍会在超时后被断开（特性仍然有效）。
	deadline := time.Now().Add(3 * time.Second)
	for c.Connected() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c.Connected() {
		t.Fatal("uncommitted timeout should still kick a stalled consumer")
	}
}

// 队列满导致的定向投递被拒必须返回 error，而不是只体现在 Dropped。
func TestAdminPushToClientBacklogReturnsError(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithQueueCap(1))
	newTestClient(t, addr, "cap-sub", "ch/a")

	if res := b.Push("ch/a", []byte("high"), core.PushOptions{}.WithPriority(packet.PriorityHigh)); res.Matched != 1 {
		t.Fatalf("first push should match: %+v", res)
	}
	if _, err := b.PushToClient("cap-sub", "ch/a", []byte("low"),
		core.PushOptions{}.WithPriority(packet.PriorityLow)); err == nil {
		t.Fatal("targeted push rejected by consumer backlog must return an error")
	}
}

// 单客户端字节配额：超过上限的新消息会被拒绝，已有消息保留。
func TestPerClientPendingBytesCap(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithMaxPendingBytesPerClient(300))
	newTestClient(t, addr, "bytes-sub", "b/a")

	if res := b.Push("b/a", make([]byte, 100), core.PushOptions{}); res.Matched != 1 || res.Dropped != 0 {
		t.Fatalf("first push should fit: %+v", res)
	}
	if res := b.Push("b/a", make([]byte, 100), core.PushOptions{}); res.Matched != 0 || res.Dropped != 1 {
		t.Fatalf("second push should exceed the byte budget: %+v", res)
	}
	info, err := b.GetClientInfo("bytes-sub")
	if err != nil {
		t.Fatal(err)
	}
	if info.Pending[1] != 1 {
		t.Fatalf("original message should remain queued: %+v", info.Pending)
	}
}

// 运行时 Declare 必须被记住，并在重连（CleanSession 清空）后自动重放。
func TestRuntimeDeclarationReplayedOnReconnect(t *testing.T) {
	b, addr := startCore(t, nil)

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("decl-1").
		WithCleanSession(true).
		WithAutoReconnect(true).
		WithLogger(zap.NewNop())
	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Declare(ctx, "decl/**"); err != nil {
		t.Fatal(err)
	}

	if err := b.DisconnectClient("decl-1", packet.ReasonServerShuttingDown); err != nil {
		t.Fatal(err)
	}
	waitClientDetached(t, b, "decl-1")

	deadline := time.Now().Add(10 * time.Second)
	var restored bool
	for time.Now().Before(deadline) {
		if info, err := b.GetClientInfo("decl-1"); err == nil && info.Connected && len(info.Channels) > 0 {
			restored = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !restored {
		t.Fatal("runtime declaration was not replayed after reconnect")
	}

	pub := newTestClient(t, addr, "decl-pub")
	if err := pub.Push(ctx, "decl/x", []byte("ok"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := c.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "ok" {
		t.Fatalf("declaration was not restored: %+v", msg)
	}
}

// 接收上限只限制读取，不应限制发送。
func TestSmallReceiveLimitDoesNotRestrictSending(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "big-sub", "big/**")
	ctx := context.Background()

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("small-recv").
		WithMaxPacketSize(512).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	pub, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()
	if err := pub.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, 2048)
	if err := pub.Push(ctx, "big/x", payload, client.PushOptions{}); err != nil {
		t.Fatalf("send must not be limited by the receive limit: %v", err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || len(msg.Payload) != len(payload) {
		t.Fatalf("subscriber should receive payload, got %+v", msg)
	}
}

// 显式发送上限仍然生效。
func TestMaxSendPacketSizeIsEnforced(t *testing.T) {
	_, addr := startCore(t, nil)
	newTestClient(t, addr, "send-sub", "send/**")
	ctx := context.Background()

	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("small-send").
		WithMaxSendPacketSize(512).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	pub, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = pub.Close() }()
	if err := pub.ConnectContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "send/x", make([]byte, 2048), client.PushOptions{}); err == nil {
		t.Fatal("send should be rejected by MaxSendPacketSize")
	}
}

// 可靠性必须独立于优先级：Low + Reliable 离线保留，High + Unreliable 直接丢弃。
func TestReliabilityIndependentOfPriority(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "rel-1", "r/**")
	pub := newTestClient(t, addr, "rel-pub")
	ctx := context.Background()

	if err := sub.Close(); err != nil {
		t.Fatal(err)
	}
	waitClientDetached(t, b, "rel-1")

	lowReliable := client.PushOptions{}.WithPriority(packet.PriorityLow).WithReliable(true)
	res, err := pub.PushWithResult(ctx, "r/low", []byte("keep"), lowReliable)
	if err != nil || res.Matched != 1 || res.Dropped != 0 {
		t.Fatalf("low+reliable should be queued offline: %+v err=%v", res, err)
	}

	highUnreliable := client.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(false)
	res, err = pub.PushWithResult(ctx, "r/high", []byte("drop"), highUnreliable)
	if err != nil || res.Matched != 0 || res.Dropped != 1 {
		t.Fatalf("high+unreliable should be dropped offline: %+v err=%v", res, err)
	}

	sub2 := newTestClient(t, addr, "rel-1", "r/**")
	msg, err := sub2.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "keep" || msg.Priority != packet.PriorityLow || !msg.NeedsCommit() {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if err := msg.Ack(ctx); err != nil {
		t.Fatal(err)
	}
}

// 不可靠消息不占用 ReceiveMaximum 窗口。
func TestUnreliableDoesNotConsumeWindow(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithReceiveMaximum(1))
	sub := newTestClient(t, addr, "win-1", "w/**")
	pub := newTestClient(t, addr, "win-pub")
	ctx := context.Background()

	highUnreliable := client.PushOptions{}.WithPriority(packet.PriorityHigh).WithReliable(false)
	if err := pub.Push(ctx, "w/high", []byte("u"), highUnreliable); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "w/med", []byte("r"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}

	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || first == nil || first.NeedsCommit() || first.Dup {
		t.Fatalf("first should be unreliable high: %+v err=%v", first, err)
	}
	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || second == nil || !second.NeedsCommit() || second.Dup {
		t.Fatalf("unreliable must not consume the window: %+v err=%v", second, err)
	}
	if err := second.Ack(ctx); err != nil {
		t.Fatal(err)
	}
}

// 发布默认不可靠：不占用窗口，也不需要 COMMIT。
func TestPushDefaultsToUnreliable(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithReceiveMaximum(1))
	sub := newTestClient(t, addr, "def-1", "d/**")
	pub := newTestClient(t, addr, "def-pub")
	ctx := context.Background()

	for _, v := range []string{"1", "2"} {
		if err := pub.Push(ctx, "d/a", []byte(v), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	for _, want := range []string{"1", "2"} {
		msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil || msg == nil || string(msg.Payload) != want {
			t.Fatalf("pull %s: %+v err=%v", want, msg, err)
		}
		if msg.Reliable || msg.NeedsCommit() || msg.Dup {
			t.Fatalf("default message must be unreliable: %+v", msg)
		}
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := sub.CommitAll(ctx); err != nil || n != 0 {
		t.Fatalf("unreliable messages must not be tracked: n=%d err=%v", n, err)
	}
}

// MaxBytes 限制单次批量响应的大小。
func TestPullBatchMaxBytes(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "mb-1", "m/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if err := pub.Push(ctx, "m/a", make([]byte, 100), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := sub.PullBatch(ctx, client.PullOptions{WaitMs: 300, MaxCount: 10, MaxBytes: 150})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 {
		t.Fatalf("MaxBytes should limit batch to 1, got %d", len(msgs))
	}
	if err := msgs[0].Ack(ctx); err != nil {
		t.Fatal(err)
	}
	rest, err := sub.PullBatch(ctx, client.PullOptions{WaitMs: 300, MaxCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(rest) != 1 {
		t.Fatalf("remaining message: %d", len(rest))
	}
	if err := rest[0].Ack(ctx); err != nil {
		t.Fatal(err)
	}
}

// PUSH_OK 的 error 属性提供人类可读的拒绝原因。
func TestPushErrorDetail(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
			return errors.New("blocked by policy")
		}),
	})
	_, addr := startCore(t, opts)
	pub := newTestClient(t, addr, "err-pub")

	_, err := pub.PushWithResult(context.Background(), "ch/a", []byte("x"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Detail == "" {
		t.Fatalf("expected ServerError with detail, got %v", err)
	}
}

// 钩子可以用 HookError 指定拒绝原因码，并只回传显式设置的 Detail。
func TestPushHookRejectReason(t *testing.T) {
	var hookID atomic.Value
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
			hookID.Store(ctx.ID)
			if ctx.FromServer {
				return &core.HookError{Reason: packet.ReasonQuotaExceeded, Detail: "system quota"}
			}
			return nil
		}),
	})
	b, addr := startCore(t, opts)
	pub := newTestClient(t, addr, "reason-pub")
	ctx := context.Background()

	res, err := pub.PushWithResult(ctx, "ch/a", []byte("x"), client.PushOptions{})
	if err != nil {
		t.Fatalf("client push should be accepted: %v", err)
	}
	if id, _ := hookID.Load().(packet.PacketID); id != res.ID {
		t.Fatalf("PushContext.ID = %v, want %v", id, res.ID)
	}

	// 服务端推送同样经过钩子，System 标记发布者不是客户端。
	sysRes := b.Push("ch/a", []byte("y"), core.PushOptions{})
	if sysRes.Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("system push should be rejected by hook: %+v", sysRes)
	}

	// 普通 error 不泄露内部信息：Detail 只回落到原因码文本。
	opts = core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.PushHandlerFunc(func(*core.PushContext) error {
			return errors.New("internal secret")
		}),
	})
	_, addr = startCore(t, opts)
	pub = newTestClient(t, addr, "reason-pub-2")
	_, err = pub.PushWithResult(ctx, "ch/a", []byte("x"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonNotAuthorized {
		t.Fatalf("plain hook error should map to NotAuthorized, got %v", err)
	}
	if strings.Contains(se.Detail, "internal secret") {
		t.Fatalf("internal error must not be exposed: %q", se.Detail)
	}
}

// CONNACK.Properties 过大时必须显式拒绝，而不是写出超过客户端上限的包。
func TestConnackPropertiesRespectClientMaxPacket(t *testing.T) {
	big := map[string]string{"k": strings.Repeat("x", 100)}
	cases := []struct {
		name string
		opts *core.CoreOptions
	}{
		{"static", core.NewCoreOptions().WithServerProperties(big)},
		{"auth", core.NewCoreOptions().WithHooks(&core.Hooks{
			Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
				ctx.ResponseProperties = big
				return nil
			}),
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, addr := startCore(t, tc.opts)
			opts := client.NewClientOptions().
				WithCore(addr).
				WithClientID("prop-big").
				WithMaxPacketSize(packet.MinPacketSize).
				WithAutoReconnect(false).
				WithLogger(zap.NewNop())
			c, err := client.NewWithOptions(opts)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.Close() }()

			err = c.ConnectContext(context.Background())
			var se *client.ServerError
			if !errors.As(err, &se) || se.Reason != packet.ReasonPacketTooLarge {
				t.Fatalf("expected PacketTooLarge, got %v", err)
			}
		})
	}
}

// CONNACK.Limits 必须反映服务端实际配置。
func TestConnackServerLimits(t *testing.T) {
	opts := core.NewCoreOptions().
		WithQueueCap(7).
		WithMaxRecordBytes(1024).
		WithMaxPullBatch(3).
		WithMaxChannelsPerClient(5).
		WithMaxChannelLength(300).
		WithWaitLimit(5000).
		WithSessionTimeout(60)
	_, addr := startCore(t, opts)
	c := newTestClient(t, addr, "limits-1")

	got := c.ServerLimits()
	if got.MaxQueueCap != 7 || got.MaxRecordBytes != 1024 || got.MaxPullBatch != 3 ||
		got.MaxChannelsPerClient != 5 || got.MaxChannelLength != 300 ||
		got.MaxWaitMs != 5000 || got.SessionTimeout != 60 {
		t.Fatalf("unexpected CONNACK limits: %+v", got)
	}
}

// 发布时已经过期的消息应被拒绝。
func TestMessageExpiredAtPush(t *testing.T) {
	_, addr := startCore(t, nil)
	pub := newTestClient(t, addr, "exp-pub")

	_, err := pub.PushWithResult(context.Background(), "ch/a", []byte("x"),
		client.PushOptions{}.WithExpiresAt(time.Now().Add(-time.Second).UnixMilli()))
	if !errors.Is(err, client.ErrExpired) {
		t.Fatalf("expected ErrExpired, got %v", err)
	}
}

// 已入队但超过 TTL 的消息在 PULL 时被丢弃。
func TestMessageTTLDroppedBeforePull(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "ttl-sub", "ttl/**")
	pub := newTestClient(t, addr, "ttl-pub")
	ctx := context.Background()

	if err := pub.Push(ctx, "ttl/a", []byte("x"),
		client.PushOptions{}.WithReliable(true).WithTTL(50*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("expired message must not be delivered: %+v", msg)
	}
	if got := b.GetStats().MessagesExpired; got == 0 {
		t.Fatal("MessagesExpired should increase")
	}
}

// 离线队列中的过期消息也会被丢弃。
func TestOfflineMessageExpires(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "ttl-off", "off/**")
	pub := newTestClient(t, addr, "ttl-pub")
	ctx := context.Background()

	_ = sub.Close()
	waitClientDetached(t, b, "ttl-off")

	if err := pub.Push(ctx, "off/a", []byte("x"),
		client.PushOptions{}.WithReliable(true).WithTTL(50*time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	sub2 := newTestClient(t, addr, "ttl-off", "off/**")
	msg, err := sub2.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if msg != nil {
		t.Fatalf("expired offline message must not be delivered: %+v", msg)
	}
}

// CONNECT.MaxPacketSize 必须容纳含 ReceiveMaximum 的 CONNACK。
func TestConnectMaxPacketTooSmallForConnack(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions())

	clientID := strings.Repeat("c", 40)
	const connackBase = 5 + 1 + 1 + 2 + 8 + 2 + 2 + 12 + 26
	need := uint32(connackBase + len(clientID))

	connectWith := func(maxPacket uint32) (packet.Packet, error) {
		nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			return nil, err
		}
		defer nc.Close()
		_ = nc.SetDeadline(time.Now().Add(5 * time.Second))
		r := &rawConn{nc: nc, t: t}
		connect := packet.NewConnectPacket(clientID)
		connect.MaxPacketSize = maxPacket
		r.send(connect)
		return r.recv()
	}

	resp, err := connectWith(need - 1)
	if err != nil {
		t.Fatal(err)
	}
	if connack, ok := resp.(*packet.ConnackPacket); !ok || connack.ReasonCode != packet.ReasonPacketTooLarge {
		t.Fatalf("expected PacketTooLarge CONNACK, got %#v", resp)
	}

	resp, err = connectWith(need)
	if err != nil {
		t.Fatal(err)
	}
	if connack, ok := resp.(*packet.ConnackPacket); !ok || connack.ReasonCode != packet.ReasonSuccess {
		t.Fatalf("expected success CONNACK at boundary, got %#v", resp)
	}
}

// 服务端用 MaxPullBatch 裁剪客户端请求的批量大小。
func TestMaxPullBatchClamp(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxPullBatch(2))
	sub := newTestClient(t, addr, "batch-1", "b/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := pub.Push(ctx, "b/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}
	msgs, err := sub.PullBatch(ctx, client.PullOptions{WaitMs: 300, MaxCount: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("MaxPullBatch should clamp to 2, got %d", len(msgs))
	}
	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}
}

// ReceiveMaximum 完全由服务端决定，客户端只读取 CONNACK 结果。
func TestReceiveMaximumFromServer(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithReceiveMaximum(4))
	sub := newTestClient(t, addr, "win-2", "x/**")
	if got := sub.ReceiveMaximum(); got != 4 {
		t.Fatalf("server value should be used: got %d", got)
	}
}

// CONNECT/CONNACK 的 Properties 可以双向交换，认证钩子可覆盖响应项。
func TestConnectPropertiesExchange(t *testing.T) {
	connectProps := make(chan map[string]string, 1)
	authProps := make(chan map[string]string, 1)
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			authProps <- ctx.Properties
			ctx.ResponseProperties = map[string]string{"session": "ok"}
			return nil
		}),
		Connect: core.ConnectHandlerFunc(func(ctx *core.ConnectContext) { connectProps <- ctx.Properties }),
	}
	_, addr := startCore(t, core.NewCoreOptions().
		WithServerProperties(map[string]string{"region": "cn"}).
		WithHooks(hooks))

	c := newTestClientWithOptions(t, addr, "prop-1", "p/**",
		client.NewClientOptions().WithProperty("model", "x1"))
	defer func() { _ = c.Close() }()

	want := map[string]string{"model": "x1"}
	for name, ch := range map[string]chan map[string]string{"auth": authProps, "connect": connectProps} {
		select {
		case got := <-ch:
			if got["model"] != want["model"] {
				t.Fatalf("%s properties: %+v", name, got)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s hook did not see properties", name)
		}
	}

	got := c.ServerProperties()
	if got["region"] != "cn" || got["session"] != "ok" {
		t.Fatalf("server properties: %+v", got)
	}
}

// OnConnect/OnDisconnect 必须带上身份与会话租约，且二者可互相关联。
func TestConnectAndDisconnectContext(t *testing.T) {
	connectCh := make(chan *core.ConnectContext, 1)
	disconnectCh := make(chan *core.DisconnectContext, 1)
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{ID: ctx.ClientID, Name: "dev", ReadChannels: []string{"**"}}
			return nil
		}),
		Connect:    core.ConnectHandlerFunc(func(ctx *core.ConnectContext) { connectCh <- ctx }),
		Disconnect: core.DisconnectHandlerFunc(func(ctx *core.DisconnectContext) { disconnectCh <- ctx }),
	}
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))
	c := newTestClient(t, addr, "ctx-1", "a/**")

	var cc *core.ConnectContext
	select {
	case cc = <-connectCh:
	case <-time.After(2 * time.Second):
		t.Fatal("connect hook not called")
	}
	if cc.SessionID == 0 || cc.RemoteAddr == "" {
		t.Fatalf("connect context missing session/remote: %+v", cc)
	}
	if cc.Identity == nil || cc.Identity.Name != "dev" {
		t.Fatalf("connect context missing identity: %+v", cc.Identity)
	}

	_ = c.Close()
	var dc *core.DisconnectContext
	select {
	case dc = <-disconnectCh:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect hook not called")
	}
	if dc.SessionID != cc.SessionID {
		t.Fatalf("disconnect session %d != connect session %d", dc.SessionID, cc.SessionID)
	}
	if dc.RemoteAddr != cc.RemoteAddr {
		t.Fatalf("disconnect remote %q != %q", dc.RemoteAddr, cc.RemoteAddr)
	}
	if dc.Identity == nil || dc.Identity.ID != "ctx-1" {
		t.Fatalf("disconnect context missing identity: %+v", dc.Identity)
	}
}

// OnPush 可以改写 Headers/Payload/Priority；改写会作用到所有命中客户端。
func TestPushHookMutatesMessage(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
			ctx.Payload = append([]byte("hooked:"), ctx.Payload...)
			ctx.SetHeader("x-injected", "1")
			ctx.Priority = packet.PriorityHigh
			return nil
		}),
	})
	_, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "mut-sub", "mut/**")
	pub := newTestClient(t, addr, "mut-pub")
	ctx := context.Background()

	if err := pub.Push(ctx, "mut/a", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("pull: %+v err=%v", msg, err)
	}
	if string(msg.Payload) != "hooked:v" {
		t.Fatalf("payload = %q", msg.Payload)
	}
	if msg.Headers["x-injected"] != "1" {
		t.Fatalf("headers = %+v", msg.Headers)
	}
	if msg.Priority != packet.PriorityHigh {
		t.Fatalf("priority = %v", msg.Priority)
	}
}

// 钩子改写后的记录超过单条上限时必须整条拒绝，而不是绕过限制入队。
func TestPushHookOutputSizeLimited(t *testing.T) {
	opts := core.NewCoreOptions().
		WithMaxRecordBytes(256).
		WithHooks(&core.Hooks{
			Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
				ctx.Payload = []byte(strings.Repeat("x", 4096))
				return nil
			}),
		})
	_, addr := startCore(t, opts)
	pub := newTestClient(t, addr, "mut-big")

	_, err := pub.PushWithResult(context.Background(), "mut/a", []byte("v"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonPacketTooLarge {
		t.Fatalf("expected PacketTooLarge, got %v", err)
	}
}

// 响应属性由对应操作的钩子自己设置：CONNACK←Auth、PUSH_OK←Push、
// PULL_OK←Pull、DECLARE_RESP←Declare。
func TestResponseProperties(t *testing.T) {
	var pullSeen atomic.Value
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  []string{"**"},
				WriteChannels: []string{"**"},
			}
			ctx.ResponseProperties = map[string]string{"session-hint": "warm"}
			return nil
		}),
		Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
			ctx.ResponseProperties = map[string]string{"trace": "push"}
			return nil
		}),
		Pull: core.PullHandlerFunc(func(ctx *core.PullContext) {
			pullSeen.Store(*ctx)
			ctx.ResponseProperties = map[string]string{"trace": "pull"}
		}),
		Declare: core.DeclareHandlerFunc(func(ctx *core.DeclareContext) {
			ctx.ResponseProperties = map[string]string{"trace": "declare"}
		}),
	}
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))
	c := newTestClient(t, addr, "rsp-1")
	ctx := context.Background()

	if got := c.ServerProperties()["session-hint"]; got != "warm" {
		t.Fatalf("CONNACK properties = %+v", c.ServerProperties())
	}

	declared, err := c.DeclareChannelsResponse(ctx, nil, client.Channel{Filter: "rsp/**"})
	if err != nil {
		t.Fatal(err)
	}
	if declared.Properties["trace"] != "declare" || declared.Accepted != 1 {
		t.Fatalf("DECLARE_RESP: accepted=%d props=%+v", declared.Accepted, declared.Properties)
	}

	pushed, err := c.PushWithResult(ctx, "rsp/a", []byte("v"), client.PushOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pushed.Properties["trace"] != "push" {
		t.Fatalf("PUSH_OK properties = %+v", pushed.Properties)
	}

	pulled, err := c.PullBatchResult(ctx, client.PullOptions{
		WaitMs:     300,
		Properties: map[string]string{"client-cursor": "42"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pulled.Messages) != 1 || pulled.Properties["trace"] != "pull" {
		t.Fatalf("PULL_OK: messages=%d props=%+v", len(pulled.Messages), pulled.Properties)
	}
	if pc, ok := pullSeen.Load().(core.PullContext); !ok || pc.ClientID != "rsp-1" ||
		pc.Properties["client-cursor"] != "42" || pc.WaitMs != 300 {
		t.Fatalf("pull hook context = %+v", pc)
	}
}

// 声明变更事件携带请求/接受/拒绝三份视图。
func TestDeclareHookEvent(t *testing.T) {
	declared := make(chan *core.DeclareContext, 1)
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(&core.Hooks{
		Declare: core.DeclareHandlerFunc(func(ctx *core.DeclareContext) { declared <- ctx }),
	}))
	c := newTestClient(t, addr, "decl-1")

	if _, err := c.Declare(context.Background(), "a/**", "$sys/x"); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-declared:
		if ev.ClientID != "decl-1" || len(ev.Requested) != 2 {
			t.Fatalf("declare event: %+v", ev)
		}
		if ev.Requested[0].Channel != "a/**" || ev.Requested[1].Channel != "$sys/x" {
			t.Fatalf("requested = %+v", ev.Requested)
		}
		if len(ev.Accepted) != 1 || ev.Accepted[0].Channel != "a/**" {
			t.Fatalf("accepted = %+v", ev.Accepted)
		}
		if len(ev.Rejected) != 1 || ev.Rejected[0].Channel != "$sys/x" {
			t.Fatalf("rejected = %+v", ev.Rejected)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("declare hook not called")
	}
}

// 会话销毁（例如断开即清理）时必须触发 SessionEnd 事件。
func TestSessionEndHook(t *testing.T) {
	ended := make(chan *core.SessionEndContext, 1)
	_, addr := startCore(t, core.NewCoreOptions().
		WithSessionTimeout(0).
		WithHooks(&core.Hooks{
			SessionEnd: core.SessionEndHandlerFunc(func(ctx *core.SessionEndContext) { ended <- ctx }),
		}))
	sub := newTestClient(t, addr, "end-1", "end/**")
	pub := newTestClient(t, addr, "end-pub")

	// 留一条未消费的可靠消息，随会话一起释放。
	if err := pub.Push(context.Background(), "end/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	_ = sub.Close()

	select {
	case ev := <-ended:
		if ev.ClientID != "end-1" || ev.SessionID == 0 {
			t.Fatalf("session end event: %+v", ev)
		}
		if ev.Reason != packet.ReasonNormalDisconnect {
			t.Fatalf("reason = %v", ev.Reason)
		}
		if ev.MessagesDropped != 1 {
			t.Fatalf("dropped messages = %d, want 1", ev.MessagesDropped)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("session end hook not called")
	}
}

// 组合器按顺序执行：Push/Deliver 遇到拒绝即中止，Connect 等事件型全部执行。
func TestChainHooks(t *testing.T) {
	var first, second, connects atomic.Int32
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.ChainPush(
			core.PushHandlerFunc(func(ctx *core.PushContext) error {
				first.Add(1)
				ctx.SetHeader("x-chain", "1")
				return nil
			}),
			core.PushHandlerFunc(func(ctx *core.PushContext) error {
				second.Add(1)
				if ctx.Headers["x-chain"] != "1" {
					return errors.New("chain order broken")
				}
				if ctx.Channel == "chain/blocked" {
					return &core.HookError{Reason: packet.ReasonQuotaExceeded}
				}
				return nil
			}),
		),
		Connect: core.ChainConnect(
			core.ConnectHandlerFunc(func(*core.ConnectContext) { connects.Add(1) }),
			core.ConnectHandlerFunc(func(*core.ConnectContext) { connects.Add(1) }),
		),
	})
	_, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "chain-sub", "chain/**")
	pub := newTestClient(t, addr, "chain-pub")
	ctx := context.Background()

	if err := pub.Push(ctx, "chain/ok", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("pull: %+v err=%v", msg, err)
	}
	if msg.Headers["x-chain"] != "1" {
		t.Fatalf("chained mutation missing: %+v", msg.Headers)
	}

	_, err = pub.PushWithResult(ctx, "chain/blocked", []byte("v"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("second handler should veto: %v", err)
	}
	if first.Load() != 2 || second.Load() != 2 {
		t.Fatalf("push chain calls: first=%d second=%d", first.Load(), second.Load())
	}
	// 两个连接 × 两个 Connect 处理器。
	if connects.Load() != 4 {
		t.Fatalf("connect chain calls = %d, want 4", connects.Load())
	}
}

// 钩子调用与 panic 计入统计，便于通过 /metrics 发现“钩子一直在拒绝/崩溃”。
func TestHookStats(t *testing.T) {
	b, addr := startCore(t, core.NewCoreOptions().WithHooks(&core.Hooks{
		Push: core.PushHandlerFunc(func(ctx *core.PushContext) error {
			if ctx.Channel == "boom/x" {
				panic("boom")
			}
			return nil
		}),
	}))
	pub := newTestClient(t, addr, "stat-pub")
	ctx := context.Background()

	_, err := pub.PushWithResult(ctx, "boom/x", []byte("v"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonUnspecifiedError {
		t.Fatalf("expected UnspecifiedError from panicking hook, got %v", err)
	}

	stats := b.GetStats()
	if got := stats.Hooks["push"]; got.Calls != 1 || got.Panics != 1 {
		t.Fatalf("push hook stats = %+v", got)
	}
	if got := stats.Hooks["deliver"]; got.Calls != 0 || got.Panics != 0 {
		t.Fatalf("unregistered hook should stay zero: %+v", got)
	}
	if _, ok := stats.Hooks["pull"]; !ok {
		t.Fatalf("pull hook stats missing: %+v", stats.Hooks)
	}
}

// NACK 后消息不再重发，并触发钩子与统计。
func TestNackRemovesDelivered(t *testing.T) {
	nackCh := make(chan *core.NackContext, 1)
	hooks := &core.Hooks{
		Nack: core.NackHandlerFunc(func(ctx *core.NackContext) { nackCh <- ctx }),
	}
	b, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))
	sub := newTestClient(t, addr, "nack-1", "n/**")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "n/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("pull: %+v err=%v", msg, err)
	}
	if err := msg.Nack(ctx, packet.ReasonUnspecifiedError); err != nil {
		t.Fatal(err)
	}

	select {
	case n := <-nackCh:
		if n.ID != msg.ID || n.ClientID != "nack-1" || n.Channel != "n/a" {
			t.Fatalf("unexpected nack context: %+v", n)
		}
		if string(n.Payload) != "v" {
			t.Fatalf("nack context should carry the payload, got %q", n.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("nack hook not called")
	}

	if next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200}); err != nil || next != nil {
		t.Fatalf("nacked message must not be redelivered: %+v err=%v", next, err)
	}
	if got := b.GetStats().MessagesNacked; got != 1 {
		t.Fatalf("MessagesNacked=%d", got)
	}
	if n, err := sub.CommitAll(ctx); err != nil || n != 0 {
		t.Fatalf("CommitAll after Nack: n=%d err=%v", n, err)
	}
}

func waitClientDetached(t *testing.T, b *core.Core, clientID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := b.GetClientInfo(clientID); err == nil && !info.Connected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client %q did not detach in time", clientID)
}

// MaxClients 必须在握手阶段原子预占，并发建连不能超卖。
func TestMaxClientsNotOversold(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxClients(1))

	const dials = 8
	results := make(chan error, dials)
	var wg sync.WaitGroup
	var keepMu sync.Mutex
	var keep []net.Conn
	t.Cleanup(func() {
		keepMu.Lock()
		defer keepMu.Unlock()
		for _, nc := range keep {
			_ = nc.Close()
		}
	})

	for i := 0; i < dials; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			nc, err := net.DialTimeout("tcp", addr, 3*time.Second)
			if err != nil {
				results <- err
				return
			}
			// 成功的连接保持打开：关闭会释放在线名额，让后续握手被接受。
			keepMu.Lock()
			keep = append(keep, nc)
			keepMu.Unlock()
			_ = nc.SetDeadline(time.Now().Add(5 * time.Second))

			connect := packet.NewConnectPacket(fmt.Sprintf("limit-%d", i))
			connect.KeepAlive = 60
			if err := packet.WritePacket(nc, connect, 0); err != nil {
				results <- err
				return
			}
			pkt, err := packet.ReadPacket(nc, 1<<20)
			if err != nil {
				results <- err
				return
			}
			connack := pkt.(*packet.ConnackPacket)
			if connack.ReasonCode == packet.ReasonSuccess {
				results <- nil
				return
			}
			results <- fmt.Errorf("rejected: %s", connack.ReasonCode)
		}(i)
	}
	wg.Wait()
	close(results)

	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("MaxClients=1 but %d connections were accepted", ok)
	}
}

// 定向投递需要独立授权：未授予 AllowTargeted 的身份必须被拒绝。
func TestTargetedPublishRequiresPermission(t *testing.T) {
	var allow atomic.Bool
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  []string{"**"},
				WriteChannels: []string{"**"},
				AllowTargeted: allow.Load(),
			}
			return nil
		}),
	}
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))

	sub := newTestClient(t, addr, "acl-sub", "cmd/**")
	pub := newTestClient(t, addr, "acl-pub")
	ctx := context.Background()

	err := pub.PushTo(ctx, "acl-sub", "cmd/x", []byte("v"), client.PushOptions{})
	var se *client.ServerError
	if !errors.As(err, &se) || se.Reason != packet.ReasonNotAuthorized {
		t.Fatalf("expected NotAuthorized for targeted publish, got %v", err)
	}

	allow.Store(true)
	// 身份在建连时确定，授权变化后必须重新建连。
	pubAllowed := newTestClient(t, addr, "acl-pub-2")
	if err := pubAllowed.PushTo(ctx, "acl-sub", "cmd/x", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatalf("targeted publish should be allowed: %v", err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || msg == nil {
		t.Fatalf("target should receive message, got %+v err=%v", msg, err)
	}
}

// 会话恢复时必须按本次身份重新校验保留的声明。
func TestSessionDeclarationsRevalidatedOnReconnect(t *testing.T) {
	var restricted atomic.Bool
	hooks := &core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			read := []string{"a/**"}
			if restricted.Load() {
				read = []string{"b/**"}
			}
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  read,
				WriteChannels: []string{"**"},
			}
			return nil
		}),
	}
	b, addr := startCore(t, core.NewCoreOptions().WithHooks(hooks))

	c := newTestClient(t, addr, "dev-1", "a/**")
	info, err := b.GetClientInfo("dev-1")
	if err != nil || len(info.Channels) != 1 {
		t.Fatalf("channels before reconnect = %+v err=%v", info, err)
	}
	_ = c.Close()
	waitClientDetached(t, b, "dev-1")

	restricted.Store(true)
	_ = newTestClient(t, addr, "dev-1")

	info, err = b.GetClientInfo("dev-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Channels) != 0 {
		t.Fatalf("declarations must be re-filtered by the new identity, got %v", info.Channels)
	}
}

// DECLARE 数量超限时整批拒绝，返回单条紧凑错误。
func TestDeclareOverLimitRejectedCompactly(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithMaxChannelsPerClient(2))
	c := newTestClient(t, addr, "dc-1")

	res, err := c.Declare(context.Background(), "a/1", "a/2", "a/3")
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Reason != packet.ReasonQuotaExceeded {
		t.Fatalf("expected single QuotaExceeded result, got %+v", res)
	}
}

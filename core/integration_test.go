package core_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

func startCore(t *testing.T, opts *core.CoreOptions) (*core.Core, string) {
	t.Helper()
	if opts == nil {
		opts = core.NewCoreOptions()
	}
	opts.WithLogger(zap.NewNop())

	b, err := core.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("start core: %v", err)
	}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = b.Serve(l) }()

	t.Cleanup(func() { _ = b.Stop() })
	return b, l.Addr().String()
}

func newTestClient(t *testing.T, addr, id string, channels ...string) *client.Client {
	t.Helper()
	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID(id).
		WithSessionTimeout(60).
		WithAutoReconnect(false).
		WithLogger(zap.NewNop())
	if len(channels) > 0 {
		opts.WithChannels(channels...)
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

func TestPushPullCommit(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "sensors/**")
	pub := newTestClient(t, addr, "pub-1")

	ctx := context.Background()
	if err := pub.Push(ctx, "sensors/temp", []byte("21.5"), client.PushOptions{}); err != nil {
		t.Fatalf("push: %v", err)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 1000})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Channel != "sensors/temp" || string(msgs[0].Payload) != "21.5" {
		t.Fatalf("unexpected message: %+v", msgs[0])
	}
	if msgs[0].Source != "pub-1" {
		t.Fatalf("source should be filled by broker, got %q", msgs[0].Source)
	}
	if msgs[0].Priority != packet.PriorityMedium {
		t.Fatalf("default priority should be medium, got %v", msgs[0].Priority)
	}

	if n, err := sub.CommitAll(ctx); err != nil || n != 1 {
		t.Fatalf("commit: n=%d err=%v", n, err)
	}

	// 确认后再拉应为空
	msgs, err = sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 200})
	if err != nil {
		t.Fatalf("pull after commit: %v", err)
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty after commit, got %d", len(msgs))
	}

	stats := b.GetStats()
	if stats.MessagesReceived != 1 || stats.MessagesSent != 1 {
		t.Fatalf("stats: received=%d sent=%d", stats.MessagesReceived, stats.MessagesSent)
	}
}

func TestReliableResentUntilCommit(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")

	ctx := context.Background()
	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	first, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 1 || first[0].Dup {
		t.Fatalf("first pull should be a fresh message, got %+v", first)
	}

	// 不 commit 再拉：应收到重复
	second, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || !second[0].Dup || second[0].ID != first[0].ID {
		t.Fatalf("expected duplicate, got %+v", second)
	}

	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	third, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if len(third) != 0 {
		t.Fatalf("expected empty after commit, got %d", len(third))
	}
}

func TestLowPriorityIsBestEffort(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")

	ctx := context.Background()
	opts := client.PushOptions{}.WithPriority(packet.PriorityLow)
	if err := pub.Push(ctx, "ch/a", []byte("fire-and-forget"), opts); err != nil {
		t.Fatal(err)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || msgs[0].NeedsCommit() {
		t.Fatalf("low priority message must not need commit: %+v", msgs)
	}

	// 低优先级发出即删除，再拉不会重复
	again, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("low priority message should be gone, got %d", len(again))
	}
}

func TestPriorityOrdering(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	lowOpt := client.PushOptions{}.WithPriority(packet.PriorityLow)
	highOpt := client.PushOptions{}.WithPriority(packet.PriorityHigh)

	if err := pub.Push(ctx, "ch/a", []byte("low"), lowOpt); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/a", []byte("high"), highOpt); err != nil {
		t.Fatal(err)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if string(msgs[0].Payload) != "high" || string(msgs[1].Payload) != "low" {
		t.Fatalf("priority order violated: %s then %s", msgs[0].Payload, msgs[1].Payload)
	}
}

func TestTargetDeliveryBypassesMatching(t *testing.T) {
	b, addr := startCore(t, nil)
	sub1 := newTestClient(t, addr, "sub-1", "other/**")
	sub2 := newTestClient(t, addr, "sub-2", "other/**")
	_ = sub2

	ctx := context.Background()
	if _, err := b.PushToClient("sub-1", "cmd/x", []byte("only-sub-1"), core.PushOptions{}); err != nil {
		t.Fatalf("publish to client: %v", err)
	}

	// sub-1 没有声明 cmd/**，仍然能收到定向消息
	msgs, err := sub1.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 1 || string(msgs[0].Payload) != "only-sub-1" {
		t.Fatalf("target client should receive message, got %+v", msgs)
	}
}

func TestTargetNotFound(t *testing.T) {
	b, _ := startCore(t, nil)
	if _, err := b.PushToClient("ghost", "cmd/x", []byte("x"), core.PushOptions{}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

func TestWildcardMatchingAndDeclare(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	results, err := sub.Declare(ctx, "a/**", "b/*")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Reason != packet.ReasonSuccess {
			t.Fatalf("declare failed: %+v", r)
		}
	}

	for _, ch := range []string{"a", "a/x", "a/x/y", "b/1", "b/1/2", "c/1"} {
		if err := pub.Push(ctx, ch, []byte(ch), client.PushOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, m := range msgs {
		got[m.Channel] = true
	}
	for _, want := range []string{"a", "a/x", "a/x/y", "b/1"} {
		if !got[want] {
			t.Errorf("expected %q to match", want)
		}
	}
	for _, notWant := range []string{"b/1/2", "c/1"} {
		if got[notWant] {
			t.Errorf("expected %q not to match", notWant)
		}
	}
}

func TestLongPollWakesOnPush(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")

	done := make(chan []*client.Message, 1)
	errc := make(chan error, 1)

	go func() {
		msgs, err := sub.Pull(context.Background(), client.PullOptions{MaxRecords: 10, WaitMs: 3000})
		if err != nil {
			errc <- err
			return
		}
		done <- msgs
	}()

	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	if err := pub.Push(context.Background(), "ch/a", []byte("wake"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	select {
	case msgs := <-done:
		if len(msgs) != 1 || string(msgs[0].Payload) != "wake" {
			t.Fatalf("unexpected messages: %+v", msgs)
		}
		if elapsed := time.Since(start); elapsed > 2*time.Second {
			t.Fatalf("long poll woke too slowly: %v", elapsed)
		}
	case err := <-errc:
		t.Fatalf("pull failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("long poll did not wake")
	}
}

func TestTakeoverFencesOldConnection(t *testing.T) {
	_, addr := startCore(t, nil)

	c1 := newTestClient(t, addr, "dup", "ch/a")
	oldSID := c1.SessionID()

	c2 := newTestClient(t, addr, "dup", "ch/a")
	defer func() { _ = c2.Close() }()

	if c2.SessionID() == oldSID {
		t.Fatal("takeover should rotate SessionID")
	}
	if !c2.SessionPresent() {
		t.Fatal("takeover should report SessionPresent")
	}

	// 旧连接应被关闭
	deadline := time.Now().Add(3 * time.Second)
	for c1.Connected() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if c1.Connected() {
		t.Fatal("old connection should be closed after takeover")
	}

	// 旧连接的请求应失败
	err := c1.Push(context.Background(), "ch/a", []byte("x"), client.PushOptions{})
	if err == nil {
		t.Fatal("push on taken-over connection should fail")
	}
}

func TestNoProgressPenalty(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithNoProgressLimits(2, 8))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 200}); err != nil {
		t.Fatal(err)
	}

	var lastErr error
	for i := 0; i < 5; i++ {
		_, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 100})
		if err != nil {
			lastErr = err
			break
		}
	}
	if !errors.Is(lastErr, client.ErrCommitRequired) {
		t.Fatalf("expected ErrCommitRequired, got %v", lastErr)
	}
}

func TestQueueCapDropsLowestPriority(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithQueueCap(2))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	lowOpt := client.PushOptions{}.WithPriority(packet.PriorityLow)

	if err := pub.Push(ctx, "ch/a", []byte("low-1"), lowOpt); err != nil {
		t.Fatal(err)
	}
	if err := pub.Push(ctx, "ch/a", []byte("low-2"), lowOpt); err != nil {
		t.Fatal(err)
	}
	// 第三条高优先级应挤出最旧的 low
	if err := pub.Push(ctx, "ch/a", []byte("high"), client.PushOptions{}.WithPriority(packet.PriorityHigh)); err != nil {
		t.Fatal(err)
	}

	msgs, err := sub.Pull(ctx, client.PullOptions{MaxRecords: 10, WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after eviction, got %d", len(msgs))
	}

	payloads := []string{string(msgs[0].Payload), string(msgs[1].Payload)}
	if payloads[0] != "high" {
		t.Fatalf("high priority should be delivered first, got %v", payloads)
	}
	for _, p := range payloads {
		if p == "low-1" {
			t.Fatalf("oldest low-priority message should have been evicted, got %v", payloads)
		}
	}
}

func TestUnauthorizedPublish(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{
				ID:            ctx.ClientID,
				ReadChannels:  []string{"ro/**"},
				WriteChannels: []string{"wo/**"},
			}
			return nil
		}),
	})
	_, addr := startCore(t, opts)
	pub := newTestClient(t, addr, "pub-1")

	err := pub.Push(context.Background(), "ro/x", []byte("x"), client.PushOptions{})
	if err == nil {
		t.Fatal("publishing to a read-only channel should fail")
	}
}

func TestDeclareUnauthorized(t *testing.T) {
	opts := core.NewCoreOptions().WithHooks(&core.Hooks{
		Auth: core.AuthHandlerFunc(func(ctx *core.AuthContext) error {
			ctx.Identity = &core.Identity{
				ID:           ctx.ClientID,
				ReadChannels: []string{"ro/**"},
			}
			return nil
		}),
	})
	_, addr := startCore(t, opts)
	sub := newTestClient(t, addr, "sub-1")

	results, err := sub.Declare(context.Background(), "other/**")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Reason != packet.ReasonNotAuthorized {
		t.Fatalf("expected NotAuthorized, got %+v", results)
	}
}

func TestStatsAndAdmin(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-1", "ch/a")

	info := b.Info()
	if info.ProtocolVersion != packet.ProtocolVersion {
		t.Fatalf("protocol version: %d", info.ProtocolVersion)
	}
	if info.Connected != 1 {
		t.Fatalf("connected: %d", info.Connected)
	}

	ci, err := b.GetClientInfo("sub-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(ci.Channels) != 1 || ci.Channels[0] != "ch/a" {
		t.Fatalf("channels: %v", ci.Channels)
	}

	if err := b.DisconnectClient("sub-1", packet.ReasonKicked); err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for sub.Connected() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if sub.Connected() {
		t.Fatal("client should be disconnected")
	}
}

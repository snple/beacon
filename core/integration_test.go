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
	if err := pub.Push(ctx, "sensors/temp", []byte("21.5"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatalf("push: %v", err)
	}

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 1000})
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if msg == nil {
		t.Fatal("expected 1 message")
	}
	if msg.Channel != "sensors/temp" || string(msg.Payload) != "21.5" {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if msg.Source != "pub-1" {
		t.Fatalf("source should be filled by broker, got %q", msg.Source)
	}
	if msg.Priority != packet.PriorityMedium {
		t.Fatalf("default priority should be medium, got %v", msg.Priority)
	}

	if n, err := sub.CommitAll(ctx); err != nil || n != 1 {
		t.Fatalf("commit: n=%d err=%v", n, err)
	}

	// 确认后再拉应为空
	msg, err = sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatalf("pull after commit: %v", err)
	}
	if msg != nil {
		t.Fatalf("expected empty after commit, got %+v", msg)
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
	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}

	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.Dup {
		t.Fatalf("first pull should be a fresh message, got %+v", first)
	}

	// 不 commit 再拉：应收到重复
	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || !second.Dup || second.ID != first.ID {
		t.Fatalf("expected duplicate, got %+v", second)
	}

	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}

	third, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if third != nil {
		t.Fatalf("expected empty after commit, got %+v", third)
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

	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || msg.NeedsCommit() {
		t.Fatalf("low priority message must not need commit: %+v", msg)
	}

	// 低优先级发出即删除，再拉不会重复
	again, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil {
		t.Fatalf("low priority message should be gone, got %+v", again)
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

	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || string(first.Payload) != "high" {
		t.Fatalf("high priority should be delivered first, got %+v", first)
	}
	if err := first.Ack(ctx); err != nil {
		t.Fatal(err)
	}

	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || string(second.Payload) != "low" {
		t.Fatalf("low priority should follow, got %+v", second)
	}
}

func TestTargetDeliveryRequiresDeclaration(t *testing.T) {
	b, addr := startCore(t, nil)
	sub1 := newTestClient(t, addr, "sub-1", "other/**")
	sub2 := newTestClient(t, addr, "sub-2", "cmd/**")
	_ = sub1
	_ = sub2

	ctx := context.Background()

	// 目标未声明该 channel：定向投递被拒绝，不会进入目标队列。
	res, err := b.PushToClient("sub-1", "cmd/x", []byte("only-sub-1"), core.PushOptions{})
	if !errors.Is(err, core.ErrTargetNotSubscribed) {
		t.Fatalf("expected ErrNoMatch for undeclared target channel, got %v (%+v)", err, res)
	}

	// 目标声明后即可收到，且不会命中其他声明 other/** 的客户端。
	if _, err := b.PushToClient("sub-2", "cmd/x", []byte("only-sub-2"), core.PushOptions{}); err != nil {
		t.Fatalf("publish to client: %v", err)
	}
	msg, err := sub2.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "only-sub-2" {
		t.Fatalf("target client should receive message, got %+v", msg)
	}
}

func TestTargetNotFound(t *testing.T) {
	b, _ := startCore(t, nil)
	if _, err := b.PushToClient("ghost", "cmd/x", []byte("x"), core.PushOptions{}); err == nil {
		t.Fatal("expected error for unknown target")
	}
}

// 客户端定向投递到未订阅的目标必须返回明确错误，不能静默成功。
func TestTargetedPushToUnsubscribedReturnsError(t *testing.T) {
	_, addr := startCore(t, nil)
	newTestClient(t, addr, "nm-sub", "ch/a")
	pub := newTestClient(t, addr, "nm-pub")

	err := pub.PushTo(context.Background(), "nm-sub", "ch/other", []byte("x"), client.PushOptions{})
	if !errors.Is(err, client.ErrTargetNotSubscribed) {
		t.Fatalf("expected ErrTargetNotSubscribed, got %v", err)
	}
}

// PushWithResult 必须暴露服务端的 Matched/Dropped，便于发布方感知丢弃。
func TestPushResultExposesMatchedAndDropped(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithQueueCap(1))
	newTestClient(t, addr, "res-sub", "ch/a")
	pub := newTestClient(t, addr, "res-pub")
	ctx := context.Background()

	res, err := pub.PushWithResult(ctx, "ch/a", []byte("one"), client.PushOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Dropped != 0 || res.Reason != packet.ReasonSuccess || res.ID.IsZero() {
		t.Fatalf("unexpected result: %+v", res)
	}

	// 队列已满：同优先级挤掉最旧消息，Matched=1 / Dropped=1。
	res, err = pub.PushWithResult(ctx, "ch/a", []byte("two"), client.PushOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Matched != 1 || res.Dropped != 1 {
		t.Fatalf("expected eviction to be visible, got %+v", res)
	}

	// 无匹配客户端：NoMatch 不算错误，但结果必须可见。
	res, err = pub.PushWithResult(ctx, "none/x", []byte("x"), client.PushOptions{})
	if err != nil {
		t.Fatalf("no-match broadcast should not error: %v", err)
	}
	if res.Reason != packet.ReasonNoMatch || res.Matched != 0 {
		t.Fatalf("unexpected no-match result: %+v", res)
	}
}

// 只接收定向投递的声明不得命中广播，但仍可接收定向投递。
func TestTargetOnlyDeclarationSkipsBroadcast(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-to")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	results, err := sub.DeclareChannels(ctx, client.Channel{Filter: "cmd/**", TargetOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("declare rejected: %+v", results)
	}

	if err := pub.Push(ctx, "cmd/x", []byte("broadcast"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if msg, err := sub.Pull(ctx, client.PullOptions{Immediate: true}); err != nil || msg != nil {
		t.Fatalf("target-only declaration must not receive broadcast: msg=%+v err=%v", msg, err)
	}

	res, err := b.PushToClient("sub-to", "cmd/x", []byte("targeted"), core.PushOptions{})
	if err != nil {
		t.Fatalf("targeted push: %v (%+v)", err, res)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "targeted" {
		t.Fatalf("target should receive targeted message, got %+v", msg)
	}
}

// 同一客户端的不同声明各自生效：广播声明命中广播，只定向声明不命中。
func TestTargetOnlyIsPerDeclaration(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-mixed")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	results, err := sub.DeclareChannels(ctx,
		client.Channel{Filter: "a/**", TargetOnly: true},
		client.Channel{Filter: "a/b"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("declare rejected: %+v", results)
	}

	if err := pub.Push(ctx, "a/c", []byte("no"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	if msg, err := sub.Pull(ctx, client.PullOptions{Immediate: true}); err != nil || msg != nil {
		t.Fatalf("a/c is only covered by a target-only declaration: msg=%+v err=%v", msg, err)
	}

	if err := pub.Push(ctx, "a/b", []byte("yes"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "yes" {
		t.Fatalf("a/b has a broadcast declaration and must receive broadcast, got %+v", msg)
	}
}

// 同一过滤器重复声明时取更宽松的模式：广播可达覆盖只定向投递。
func TestDuplicateDeclarationKeepsBroadcast(t *testing.T) {
	b, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-dup")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	results, err := sub.DeclareChannels(ctx,
		client.Channel{Filter: "d/1", TargetOnly: true},
		client.Channel{Filter: "d/1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("declare rejected: %+v", results)
	}

	ci, err := b.GetClientInfo("sub-dup")
	if err != nil {
		t.Fatal(err)
	}
	if len(ci.Channels) != 1 || ci.Channels[0].Filter != "d/1" || ci.Channels[0].TargetOnly {
		t.Fatalf("channels: %+v", ci.Channels)
	}

	if err := pub.Push(ctx, "d/1", []byte("v"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}
	msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || string(msg.Payload) != "v" {
		t.Fatalf("duplicate declaration should stay broadcast-capable, got %+v", msg)
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

	got := map[string]bool{}
	for i := 0; i < 4; i++ {
		msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 500})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil {
			t.Fatalf("expected 4 matching messages, got %d", i)
		}
		got[msg.Channel] = true
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
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

	done := make(chan *client.Message, 1)
	errc := make(chan error, 1)

	go func() {
		msg, err := sub.Pull(context.Background(), client.PullOptions{WaitMs: 3000})
		if err != nil {
			errc <- err
			return
		}
		done <- msg
	}()

	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	if err := pub.Push(context.Background(), "ch/a", []byte("wake"), client.PushOptions{}); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-done:
		if msg == nil || string(msg.Payload) != "wake" {
			t.Fatalf("unexpected message: %+v", msg)
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
	disconnects := make(chan *core.DisconnectContext, 4)
	_, addr := startCore(t, core.NewCoreOptions().WithHooks(&core.Hooks{
		Disconnect: core.DisconnectHandlerFunc(func(ctx *core.DisconnectContext) { disconnects <- ctx }),
	}))

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

	// 断开事件必须保留被接管连接自己的租约，而不是新连接的。
	select {
	case dc := <-disconnects:
		if dc.ClientID != "dup" || dc.SessionID != oldSID {
			t.Fatalf("taken-over connection reported wrong lease: %+v", dc)
		}
		if dc.Reason != packet.ReasonSessionTakenOver {
			t.Fatalf("disconnect reason = %v", dc.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect hook not called for taken-over connection")
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

	if err := pub.Push(ctx, "ch/a", []byte("v"), client.PushOptions{}.WithReliable(true)); err != nil {
		t.Fatal(err)
	}
	if _, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200}); err != nil {
		t.Fatal(err)
	}

	var lastErr error
	for i := 0; i < 5; i++ {
		_, err := sub.Pull(ctx, client.PullOptions{WaitMs: 100})
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

	var payloads []string
	for i := 0; i < 2; i++ {
		msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
		if err != nil {
			t.Fatal(err)
		}
		if msg == nil {
			t.Fatalf("expected 2 messages after eviction, got %d", i)
		}
		payloads = append(payloads, string(msg.Payload))
		if err := msg.Ack(ctx); err != nil {
			t.Fatal(err)
		}
	}
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

// "$" 前缀保留给 broker：客户端不得声明该命名空间。
func TestReservedChannelDeclarationRejected(t *testing.T) {
	b, addr := startCore(t, nil)
	c := newTestClient(t, addr, "resv-1")
	ctx := context.Background()

	for _, filter := range []string{"$sys/**", "$exact"} {
		results, err := c.Declare(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 || results[0].Channel != filter ||
			results[0].Reason != packet.ReasonChannelFilterInvalid {
			t.Fatalf("declare %q should be rejected, got %+v", filter, results)
		}
	}

	if res := b.Push("$sys/x", []byte("x"), core.PushOptions{}); res.Matched != 0 {
		t.Fatalf("reserved channel must not match clients: %+v", res)
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
	if len(ci.Channels) != 1 || ci.Channels[0].Filter != "ch/a" {
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

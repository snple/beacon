package core_test

import (
	"context"
	"sync"
	"testing"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
)

// ReceiveMaximum=1 时窗口满，未确认前只能重发同一条消息。
func TestReceiveMaximumGatesNewDelivery(t *testing.T) {
	_, addr := startCore(t, core.NewCoreOptions().WithReceiveMaximum(1))
	sub := newTestClient(t, addr, "sub-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"m1", "m2", "m3"} {
		if _, err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}

	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || string(first.Payload) != "m1" || first.Dup {
		t.Fatalf("first pull: %+v", first)
	}

	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if second == nil || !second.Dup || second.ID != first.ID {
		t.Fatalf("expected duplicate of m1, got %+v", second)
	}

	if _, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	}
	third, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if third == nil || string(third.Payload) != "m2" || third.Dup {
		t.Fatalf("third pull should be m2, got %+v", third)
	}
}

// 窗口内允许先拉取多条再统一确认。
func TestReceiveWindowAllowsPipelinedDelivery(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "pipe-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"m1", "m2"} {
		if _, err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}

	first, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || first == nil || first.Dup {
		t.Fatalf("first pull: %+v err=%v", first, err)
	}
	second, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil || second == nil || second.Dup {
		t.Fatalf("second pull should be fresh: %+v err=%v", second, err)
	}
	if first.ID == second.ID {
		t.Fatal("pipelined pulls must return distinct messages")
	}

	if res, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	} else if res.Committed != 2 {
		t.Fatalf("committed %d, want 2", res.Committed)
	}
}

// 批量拉取一次返回多条，并可在一次 COMMIT 中全部确认。
func TestPullBatchAndCommitBatch(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "batch-1", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"m1", "m2", "m3"} {
		if _, err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}

	res, err := sub.PullBatch(ctx, client.PullOptions{WaitMs: 300, MaxCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Messages) != 3 {
		t.Fatalf("expected 3 records, got %d", len(res.Messages))
	}
	ids := make([]packet.PacketID, len(res.Messages))
	for i, m := range res.Messages {
		ids[i] = m.ID
		if m.Dup {
			t.Fatalf("batch record %d should not be dup", i)
		}
	}
	if _, err := sub.Commit(ctx, ids...); err != nil {
		t.Fatal(err)
	}
	if res, err := sub.CommitAll(ctx); err != nil || res.Committed != 0 {
		t.Fatalf("CommitAll after CommitBatch: committed=%d err=%v", res.Committed, err)
	}
}

// 低优先级消息不进入 delivered，因此不会阻塞新消息。
func TestLowPriorityDoesNotBlock(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "sub-3", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	low := client.PushOptions{}.WithPriority(client.PriorityLow)
	if _, err := pub.Push(ctx, "ch/a", []byte("l1"), low); err != nil {
		t.Fatal(err)
	}
	if _, err := pub.Push(ctx, "ch/a", []byte("l2"), low); err != nil {
		t.Fatal(err)
	}

	if _, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300}); err != nil {
		t.Fatal(err)
	}
	// Low 已删除，不需要 commit；第二次应拿到剩下那条而不是重复
	next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 300})
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Dup || string(next.Payload) != "l2" {
		t.Fatalf("low priority should not be gated: %+v", next)
	}
}

// 客户端并发 Pull 会被 SDK 串行化，本地未确认集合不会互相覆盖。
func TestConcurrentPullSerializedByClient(t *testing.T) {
	_, addr := startCore(t, nil)
	sub := newTestClient(t, addr, "cc-2", "ch/a")
	pub := newTestClient(t, addr, "pub-1")
	ctx := context.Background()

	for _, v := range []string{"1", "2"} {
		if _, err := pub.Push(ctx, "ch/a", []byte(v), client.PushOptions{}.WithReliable(true)); err != nil {
			t.Fatal(err)
		}
	}

	var wg sync.WaitGroup
	results := make([]*client.Message, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			msg, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
			if err != nil {
				t.Errorf("pull: %v", err)
				return
			}
			results[i] = msg
		}(i)
	}
	wg.Wait()

	// 串行化后两次拉取应各拿到一条不同消息，且都不是重复投递。
	if results[0] == nil || results[1] == nil {
		t.Fatalf("both pulls should return a message: %+v", results)
	}
	if results[0].ID == results[1].ID {
		t.Fatalf("serialized pulls should advance: %+v", results)
	}
	if results[0].Dup || results[1].Dup {
		t.Fatalf("fresh window should not redeliver: %+v", results)
	}

	if res, err := sub.CommitAll(ctx); err != nil {
		t.Fatal(err)
	} else if res.Committed != 2 {
		t.Fatalf("committed %d, want 2 messages", res.Committed)
	}

	next, err := sub.Pull(ctx, client.PullOptions{WaitMs: 200})
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("queue should be drained, got %+v", next)
	}
}

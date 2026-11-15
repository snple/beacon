package client

import (
	"context"
	"testing"

	"github.com/snple/beacon/packet"
)

// 回归：COMMIT 失败后未确认集合必须保持可重试，不能被标记为已确认。
func TestCommitAllRetriesAfterFailure(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("t").WithServer("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	m := &Message{Reliable: true, client: c}
	m.ID = packet.NewPacketID()
	c.uncommitted = []*Message{m}

	if _, err := c.CommitAll(context.Background()); err == nil {
		t.Fatal("expected commit to fail without a connection")
	}
	if m.acked.Load() {
		t.Fatal("message must not be marked acked after a failed commit")
	}
	if len(c.uncommitted) != 1 {
		t.Fatalf("uncommitted should still hold the message, got %d", len(c.uncommitted))
	}

	// 第二次仍应真正尝试提交（而不是静默返回成功）。
	if _, err := c.CommitAll(context.Background()); err == nil {
		t.Fatal("retry should attempt the commit again")
	}
	if len(c.uncommitted) != 1 {
		t.Fatalf("retry must keep the message queued, got %d", len(c.uncommitted))
	}
}

func TestMarkCommittedRemovesEntries(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("t").WithServer("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	keep := &Message{Reliable: true, client: c}
	drop := &Message{Reliable: true, client: c}
	keep.ID = packet.NewPacketID()
	drop.ID = packet.NewPacketID()
	c.uncommitted = []*Message{keep, drop}

	c.markCommittedBatch([]packet.PacketID{drop.ID})

	if len(c.uncommitted) != 1 || c.uncommitted[0] != keep {
		t.Fatalf("unexpected uncommitted set: %+v", c.uncommitted)
	}
	if !drop.acked.Load() {
		t.Fatal("committed message should be marked acked")
	}
}

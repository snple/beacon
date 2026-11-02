package client

import "testing"

func TestEffectiveWait(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("t").WithCore("127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}

	if got := c.effectiveWait(PullOptions{}); got != 30000 {
		t.Fatalf("zero WaitMs should fall back to DefaultWaitMs, got %d", got)
	}
	if got := c.effectiveWait(PullOptions{WaitMs: 500}); got != 500 {
		t.Fatalf("explicit WaitMs ignored: %d", got)
	}
	if got := c.effectiveWait(PullOptions{Immediate: true}); got != 0 {
		t.Fatalf("Immediate should force 0, got %d", got)
	}
	if got := c.effectiveWait(PullOptions{Immediate: true, WaitMs: 900}); got != 0 {
		t.Fatalf("Immediate should win over WaitMs, got %d", got)
	}

	c.opts.DefaultWaitMs = 0
	if got := c.effectiveWait(PullOptions{}); got != 0 {
		t.Fatalf("DefaultWaitMs=0 should disable long polling, got %d", got)
	}
}

func TestDefaultWaitMsIsLongPoll(t *testing.T) {
	// 默认 30s，确保「空队列时等待」是默认行为而不是忙循环。
	if NewClientOptions().DefaultWaitMs != 30000 {
		t.Fatal("DefaultWaitMs should default to 30s")
	}
	if NewClientOptions().DefaultWaitMs < 1000 {
		t.Fatal("DefaultWaitMs should be at least 1s")
	}
}

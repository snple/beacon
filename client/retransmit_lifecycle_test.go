package client

import (
	"sync"
	"testing"
)

// TestTriggerRetransmit_AfterCloseIsSafe 验证 Close 之后触发重传不会
// 向已 Wait 的 WaitGroup 添加任务，也不会访问已关闭的存储。
func TestTriggerRetransmit_AfterCloseIsSafe(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("retransmit-closed"))
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("关闭客户端失败: %v", err)
	}

	for range 50 {
		c.TriggerRetransmit()
	}
}

// TestTriggerRetransmit_ConcurrentWithClose 验证 Close 与 TriggerRetransmit 并发安全。
func TestTriggerRetransmit_ConcurrentWithClose(t *testing.T) {
	c, err := NewWithOptions(NewClientOptions().WithClientID("retransmit-race"))
	if err != nil {
		t.Fatalf("创建客户端失败: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 200 {
			c.TriggerRetransmit()
		}
	}()

	if err := c.Close(); err != nil {
		t.Fatalf("关闭客户端失败: %v", err)
	}
	wg.Wait()
}

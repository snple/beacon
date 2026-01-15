package core

import (
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// TestSendQueue 测试发送队列的基本功能
func TestSendQueue(t *testing.T) {
	capacity := 10
	queue := NewSendQueue(capacity)
	defer queue.Close()

	// 测试容量
	if queue.Capacity() != capacity {
		t.Errorf("Expected capacity %d, got %d", capacity, queue.Capacity())
	}

	// 测试空队列
	if !queue.IsEmpty() {
		t.Error("Queue should be empty initially")
	}
	if queue.IsFull() {
		t.Error("Queue should not be full initially")
	}
	if queue.Available() != capacity {
		t.Errorf("Expected available %d, got %d", capacity, queue.Available())
	}

	// 测试入队
	msg := &Message{
		Topic:   "test/topic",
		Payload: []byte("test message"),
		QoS:     packet.QoS1,
	}

	for i := 0; i < capacity; i++ {
		if !queue.TryEnqueue(msg, packet.QoS1, false) {
			t.Errorf("Failed to enqueue message %d", i)
		}
	}

	// 测试满队列
	if !queue.IsFull() {
		t.Error("Queue should be full")
	}
	if queue.Available() != 0 {
		t.Errorf("Expected available 0, got %d", queue.Available())
	}

	// 尝试继续入队应该失败
	if queue.TryEnqueue(msg, packet.QoS1, false) {
		t.Error("Should not be able to enqueue to full queue")
	}

	// 测试出队
	for i := 0; i < capacity; i++ {
		qm, ok := queue.TryDequeue()
		if !ok {
			t.Errorf("Failed to dequeue message %d", i)
		}
		if qm.Message.Topic != "test/topic" {
			t.Errorf("Expected topic 'test/topic', got '%s'", qm.Message.Topic)
		}
	}

	// 测试空队列出队
	if !queue.IsEmpty() {
		t.Error("Queue should be empty after dequeuing all")
	}
	if _, ok := queue.TryDequeue(); ok {
		t.Error("Should not be able to dequeue from empty queue")
	}
}

// TestSendQueueUsageRatio 测试队列使用率
func TestSendQueueUsageRatio(t *testing.T) {
	queue := NewSendQueue(10)
	defer queue.Close()

	msg := &Message{
		Topic:   "test",
		Payload: []byte("data"),
		QoS:     packet.QoS0,
	}

	// 空队列
	if ratio := queue.UsageRatio(); ratio != 0.0 {
		t.Errorf("Expected usage ratio 0.0, got %f", ratio)
	}

	// 入队 5 个消息 (50%)
	for i := 0; i < 5; i++ {
		queue.TryEnqueue(msg, packet.QoS0, false)
	}
	if ratio := queue.UsageRatio(); ratio != 0.5 {
		t.Errorf("Expected usage ratio 0.5, got %f", ratio)
	}

	// 满队列 (100%)
	for i := 0; i < 5; i++ {
		queue.TryEnqueue(msg, packet.QoS0, false)
	}
	if ratio := queue.UsageRatio(); ratio != 1.0 {
		t.Errorf("Expected usage ratio 1.0, got %f", ratio)
	}
}

// TestSendQueueConcurrency 测试并发场景
func TestSendQueueConcurrency(t *testing.T) {
	queue := NewSendQueue(100)
	defer queue.Close()

	msg := &Message{
		Topic:   "concurrent/test",
		Payload: []byte("data"),
		QoS:     packet.QoS1,
	}

	// 并发入队
	done := make(chan bool, 2)
	go func() {
		for i := 0; i < 50; i++ {
			queue.TryEnqueue(msg, packet.QoS1, false)
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// 并发出队
	go func() {
		for i := 0; i < 50; i++ {
			queue.TryDequeue()
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	// 等待完成
	<-done
	<-done

	// 验证队列状态一致性
	used := queue.Used()
	if used < 0 || used > queue.Capacity() {
		t.Errorf("Invalid queue state: used=%d, capacity=%d", used, queue.Capacity())
	}
}

// TestQueuedMessageEnqueueTime 测试消息入队时间戳
func TestQueuedMessageEnqueueTime(t *testing.T) {
	queue := NewSendQueue(10)
	defer queue.Close()

	msg := &Message{
		Topic:   "test",
		Payload: []byte("data"),
		QoS:     packet.QoS0,
	}

	beforeEnqueue := time.Now().Unix()
	queue.TryEnqueue(msg, packet.QoS0, false)
	afterEnqueue := time.Now().Unix()

	qm, ok := queue.TryDequeue()
	if !ok {
		t.Fatal("Failed to dequeue message")
	}

	if qm.EnqueueTime < beforeEnqueue || qm.EnqueueTime > afterEnqueue {
		t.Errorf("Invalid enqueue time: %d, expected between %d and %d",
			qm.EnqueueTime, beforeEnqueue, afterEnqueue)
	}
}

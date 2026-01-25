package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueuePersistence 测试消息队列持久化
func TestQueuePersistence(t *testing.T) {
	broker := testSetupCore(t)
	defer broker.Stop()

	opts := NewClientOptions().
		WithCore(broker.GetAddress()).
		WithClientID("test-queue-client").
		WithSessionTimeout(60)

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 连接到服务器
	err = c.Connect()
	require.NoError(t, err)

	// 发布一条 QoS1 消息
	topic := fmt.Sprintf("test/queue/%d", time.Now().UnixNano())
	payload := []byte("test message")

	err = c.Publish(topic, payload, NewPublishOptions().WithQoS(packet.QoS1))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	// 验证队列已清空（消息已发送并确认）
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty, "Queue should be empty after successful publish")
}

// TestOfflineQueueing 测试离线时消息入队
func TestOfflineQueueing(t *testing.T) {
	broker := testSetupCore(t)
	defer broker.Stop()

	opts := NewClientOptions().
		WithCore(broker.GetAddress()).
		WithClientID("test-offline-queue").
		WithSessionTimeout(60).
		WithRetransmitInterval(100 * time.Millisecond) // 快速重传

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 先连接一次（初始化 queue）
	err = c.Connect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 断开连接
	err = c.Disconnect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 验证已断开
	assert.False(t, c.IsConnected())

	// 离线时发布 QoS1 消息（应该入队）
	topic := fmt.Sprintf("test/offline/%d", time.Now().UnixNano())
	payload := []byte("offline message")

	err = c.Publish(topic, payload, NewPublishOptions().WithQoS(packet.QoS1))
	// 离线发布 QoS1 应该成功（消息已入队）
	require.NoError(t, err)

	// 验证消息在队列中
	size, err := c.queue.Size()
	require.NoError(t, err)
	assert.Greater(t, size, uint64(0), "Queue should contain offline message")

	// 重新连接
	err = c.Connect()
	require.NoError(t, err)

	// 订阅主题，以便能收到消息并发送 PUBACK
	err = c.Subscribe(topic)
	require.NoError(t, err)

	// 等待重传机制处理和 PUBACK
	time.Sleep(300 * time.Millisecond)

	// 检查队列大小
	size, err = c.queue.Size()
	require.NoError(t, err)
	t.Logf("Queue size after retransmit: %d", size)

	// 验证队列已清空（消息已重传并确认）
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty, "Queue should be empty after retransmit")
}

// TestRetransmitMechanism 测试重传机制
func TestRetransmitMechanism(t *testing.T) {
	broker := testSetupCore(t)
	defer broker.Stop()

	opts := NewClientOptions().
		WithCore(broker.GetAddress()).
		WithClientID("test-retransmit").
		WithSessionTimeout(60).
		WithRetransmitInterval(200 * time.Millisecond) // 快速重传用于测试

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 连接
	err = c.Connect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 订阅主题
	topic := fmt.Sprintf("test/retransmit/%d", time.Now().UnixNano())
	err = c.Subscribe(topic)
	require.NoError(t, err)

	// 发布消息
	payload := []byte("retransmit test")
	err = c.Publish(topic, payload, NewPublishOptions().WithQoS(packet.QoS1))
	require.NoError(t, err)

	// 接收消息
	msg, err := c.PollMessage(c.ctx, 1*time.Second)
	require.NoError(t, err)
	assert.Equal(t, topic, msg.Topic())
	assert.Equal(t, payload, msg.Payload())

	// 验证队列已清空
	time.Sleep(200 * time.Millisecond)
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty, "Queue should be empty after successful delivery")
}

// TestQoS0Queueing 测试 QoS0 消息的队列行为
func TestQoS0Queueing(t *testing.T) {
	broker := testSetupCore(t)
	defer broker.Stop()

	opts := NewClientOptions().
		WithCore(broker.GetAddress()).
		WithClientID("test-qos0-queue").
		WithSessionTimeout(60)

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 先连接
	err = c.Connect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 发布 QoS0 消息
	topic := fmt.Sprintf("test/qos0/%d", time.Now().UnixNano())
	payload := []byte("qos0 message")

	err = c.Publish(topic, payload, NewPublishOptions().WithQoS(packet.QoS0))
	require.NoError(t, err)

	// 等待发送完成
	time.Sleep(200 * time.Millisecond)

	// QoS0 消息发送后应从队列中删除
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty, "Queue should be empty after QoS0 message sent")
}

// TestQueueExpiredMessages 测试过期消息的处理
func TestQueueExpiredMessages(t *testing.T) {
	broker := testSetupCore(t)
	defer broker.Stop()

	opts := NewClientOptions().
		WithCore(broker.GetAddress()).
		WithClientID("test-expired-queue").
		WithSessionTimeout(60).
		WithRetransmitInterval(200 * time.Millisecond)

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 连接
	err = c.Connect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 断开连接
	err = c.Disconnect()
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)

	// 离线发布一条很快过期的消息
	topic := fmt.Sprintf("test/expired/%d", time.Now().UnixNano())
	payload := []byte("expired message")

	err = c.Publish(topic, payload, NewPublishOptions().
		WithQoS(packet.QoS1).
		WithExpiry(1)) // 1秒后过期
	require.NoError(t, err)

	// 等待消息过期
	time.Sleep(1500 * time.Millisecond)

	// 重新连接
	err = c.Connect()
	require.NoError(t, err)

	// 等待重传机制处理（应该删除过期消息）
	time.Sleep(500 * time.Millisecond)

	// 验证队列已清空（过期消息已删除）
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty, "Queue should be empty after expired messages cleaned")
}

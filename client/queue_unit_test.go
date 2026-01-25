package client

import (
	"testing"

	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueueBasicOperations 测试队列的基本操作（不需要服务器）
func TestQueueBasicOperations(t *testing.T) {
	// 创建客户端配置
	opts := NewClientOptions().
		WithClientID("test-queue-basic").
		WithSessionTimeout(60)

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 验证 queue 已初始化
	require.NotNil(t, c.queue)

	// 验证队列初始为空
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty)

	// 创建测试消息
	pub := packet.NewPublishPacket("test/topic", []byte("test payload"))
	pub.QoS = packet.QoS1
	pub.PacketID = c.allocatePacketID()

	msg := &Message{
		Packet:    pub,
		Timestamp: 0,
	}

	// 入队
	err = c.queue.Enqueue(msg)
	require.NoError(t, err)

	// 验证队列不为空
	isEmpty, err = c.queue.IsEmpty()
	require.NoError(t, err)
	assert.False(t, isEmpty)

	// 验证队列大小
	size, err := c.queue.Size()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), size)

	// Peek 查看消息（不删除）
	peeked, err := c.queue.Peek()
	require.NoError(t, err)
	assert.Equal(t, msg.Packet.Topic, peeked.Packet.Topic)
	assert.Equal(t, msg.Packet.PacketID, peeked.Packet.PacketID)

	// 验证队列大小不变
	size, err = c.queue.Size()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), size)

	// 出队
	dequeued, err := c.queue.Dequeue()
	require.NoError(t, err)
	assert.Equal(t, msg.Packet.Topic, dequeued.Packet.Topic)
	assert.Equal(t, msg.Packet.PacketID, dequeued.Packet.PacketID)

	// 验证队列为空
	isEmpty, err = c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty)
}

// TestQueueDeleteByPacketID 测试根据 PacketID 删除消息
func TestQueueDeleteByPacketID(t *testing.T) {
	opts := NewClientOptions().
		WithClientID("test-queue-delete")

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 创建两条消息
	msg1 := &Message{
		Packet: &packet.PublishPacket{
			Topic:    "test/1",
			Payload:  []byte("msg1"),
			QoS:      packet.QoS1,
			PacketID: c.allocatePacketID(),
		},
	}

	msg2 := &Message{
		Packet: &packet.PublishPacket{
			Topic:    "test/2",
			Payload:  []byte("msg2"),
			QoS:      packet.QoS1,
			PacketID: c.allocatePacketID(),
		},
	}

	// 入队
	err = c.queue.Enqueue(msg1)
	require.NoError(t, err)
	err = c.queue.Enqueue(msg2)
	require.NoError(t, err)

	// 验证队列大小
	size, err := c.queue.Size()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), size)

	// 根据 PacketID 删除第一条消息
	err = c.queue.Delete(msg1.Packet.PacketID)
	require.NoError(t, err)

	// 验证队列大小
	size, err = c.queue.Size()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), size)

	// 出队应该得到第二条消息
	dequeued, err := c.queue.Dequeue()
	require.NoError(t, err)
	assert.Equal(t, msg2.Packet.Topic, dequeued.Packet.Topic)
}

// TestQueueClear 测试清空队列
func TestQueueClear(t *testing.T) {
	opts := NewClientOptions().
		WithClientID("test-queue-clear")

	c, err := NewWithOptions(opts)
	require.NoError(t, err)
	defer c.Close()

	// 入队多条消息
	for i := 0; i < 5; i++ {
		msg := &Message{
			Packet: &packet.PublishPacket{
				Topic:    "test/topic",
				Payload:  []byte("payload"),
				QoS:      packet.QoS1,
				PacketID: c.allocatePacketID(),
			},
		}
		err = c.queue.Enqueue(msg)
		require.NoError(t, err)
	}

	// 验证队列大小
	size, err := c.queue.Size()
	require.NoError(t, err)
	assert.Equal(t, uint64(5), size)

	// 清空队列
	err = c.queue.Clear()
	require.NoError(t, err)

	// 验证队列为空
	isEmpty, err := c.queue.IsEmpty()
	require.NoError(t, err)
	assert.True(t, isEmpty)
}

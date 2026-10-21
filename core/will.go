package core

import (
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// setWillPacket 原子替换遗嘱消息
func (c *Client) setWillPacket(will *packet.PublishPacket) {
	c.session.willMu.Lock()
	c.session.willPacket = will
	c.session.willMu.Unlock()
}

// takeWillPacket 取出并清空遗嘱消息（调用方在锁外发布，避免持锁执行 IO）
func (c *Client) takeWillPacket() *packet.PublishPacket {
	c.session.willMu.Lock()
	will := c.session.willPacket
	c.session.willPacket = nil
	c.session.willMu.Unlock()
	return will
}

// extractWillMessage 提取遗嘱消息
func (c *Client) extractWillMessage(connect *packet.ConnectPacket) {
	// 遗嘱属于单次连接：没有遗嘱时也要清除上一条连接遗留的遗嘱
	if !connect.Will || connect.WillPacket == nil {
		c.setWillPacket(nil)
		return
	}

	pub := connect.WillPacket

	if pub.Properties == nil {
		pub.Properties = packet.NewPublishProperties()
	}

	// 填充 SourceClientID（发送者标识，由 core 填充确保可信）
	pub.Properties.SourceClientID = c.ID

	// !!! will 消息的超时时间由客户端掉线时确定

	c.setWillPacket(pub)
}

// publishWill 发布遗嘱消息
func (c *Client) publishWill() {
	// 先取出并清空：保证并发调用时最多发布一次，且发布（可能阻塞）不持锁
	pub := c.takeWillPacket()
	if pub == nil {
		return
	}

	// 遗嘱包中的 ExpiryTime 表示**相对 TTL（秒）**，在遗嘱实际发布时才开始计算：
	// CONNECT 时携带的绝对时间在长时间连接后已经失效，不能直接沿用。
	switch {
	case pub.Properties != nil && pub.Properties.ExpiryTime > 0:
		pub.Properties.ExpiryTime = time.Now().Add(time.Duration(pub.Properties.ExpiryTime) * time.Second).Unix()
	case c.core.options.DefaultMessageExpiry > 0:
		pub.Properties.ExpiryTime = time.Now().Add(c.core.options.DefaultMessageExpiry).Unix()
	default:
		// DefaultMessageExpiry=0 表示永不过期
		if pub.Properties != nil {
			pub.Properties.ExpiryTime = 0
		}
	}

	// QoS/Retain 必须带到 Message 层：投递链路取的是 Message.QoS / Message.Retain，
	// 只设置 Packet 上的标志会被丢弃。
	msg := Message{
		Packet:    pub,
		QoS:       pub.QoS,
		Retain:    pub.Retain,
		Timestamp: time.Now().Unix(),
	}

	c.core.logger.Info("Publishing will message",
		zap.String("clientID", c.ID),
		zap.String("topic", pub.Topic))

	c.core.publish(msg)
}

// clearWill 清除遗嘱消息（正常断开时调用）
func (c *Client) clearWill() {
	c.setWillPacket(nil)
}

package core

import (
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// extractWillMessage 提取遗嘱消息
func (c *Client) extractWillMessage(connect *packet.ConnectPacket) {
	// 遗嘱属于单次连接：没有遗嘱时也要清除上一条连接遗留的遗嘱
	c.session.willPacket = nil

	if !connect.Will {
		return
	}

	pub := connect.WillPacket

	if pub.Properties == nil {
		pub.Properties = packet.NewPublishProperties()
	}

	// 填充 SourceClientID（发送者标识，由 core 填充确保可信）
	pub.Properties.SourceClientID = c.ID

	// !!! will 消息的超时时间由客户端掉线时确定

	c.session.willPacket = pub
}

// publishWill 发布遗嘱消息
func (c *Client) publishWill() {
	if c.session.willPacket == nil {
		return
	}

	// !!! 遗嘱消息没比要复制
	pub := c.session.willPacket

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
		zap.String("topic", c.session.willPacket.Topic))

	c.core.publish(msg)

	// 清除遗嘱，防止重复发送
	c.session.willPacket = nil
}

// clearWill 清除遗嘱消息（正常断开时调用）
func (c *Client) clearWill() {
	c.session.willPacket = nil
}

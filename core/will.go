package core

import (
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// extractWillMessage 提取遗嘱消息
func (c *Client) extractWillMessage(connect *packet.ConnectPacket) {
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

	// !!! 遗嘱消息的超时时间由客户端掉线时确定
	// DefaultMessageExpiry=0 表示永不过期：此时不设置 ExpiryTime，
	// 否则过期时间会被设为“现在”，消息下一秒即被判为过期而丢弃（M5）。
	// 语义与 applyPublishExpiry 保持一致（>0 才设置过期）。
	if c.core.options.DefaultMessageExpiry > 0 {
		expiry := time.Now().Add(c.core.options.DefaultMessageExpiry)
		pub.Properties.ExpiryTime = expiry.Unix()
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

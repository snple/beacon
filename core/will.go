package core

import (
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// extractWillMessage 提取遗嘱消息
func (c *Client) extractWillMessage(connect *packet.ConnectPacket) {
	if !connect.Flags.Will {
		return
	}
	c.WillPacket = connect.WillPacket
}

// publishWill 发布遗嘱消息
func (c *Client) publishWill() {
	if c.WillPacket == nil {
		return
	}

	pub := packet.NewPublishPacket(c.WillPacket.Topic, c.WillPacket.Payload)
	pub.QoS = c.WillPacket.QoS
	pub.Retain = c.WillPacket.Retain

	if c.WillPacket.Properties != nil {
		props := *c.WillPacket.Properties // shallow copy: avoid mutating WillPacket
		pub.Properties = &props
	} else {
		pub.Properties = packet.NewPublishProperties()
	}

	// 自动填充 SourceClientID（发送者标识，由 core 填充确保可信）
	pub.Properties.SourceClientID = c.ID

	// 如果没有设置过期时间，使用默认值
	if pub.Properties.ExpiryTime <= 0 && c.core.options.DefaultMessageExpiry > 0 {
		expiry := time.Now().Add(c.core.options.DefaultMessageExpiry)
		pub.Properties.ExpiryTime = expiry.Unix()
	}

	msg := Message{Packet: pub, Timestamp: time.Now().Unix()}

	c.core.logger.Info("Publishing will message",
		zap.String("clientID", c.ID),
		zap.String("topic", c.WillPacket.Topic))

	c.core.publish(msg)

	// 清除遗嘱，防止重复发送
	c.WillPacket = nil
}

// ClearWill 清除遗嘱消息（正常断开时调用）
func (c *Client) ClearWill() {
	c.WillPacket = nil
}

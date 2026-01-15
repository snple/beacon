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

	msg := &Message{
		Topic:     c.WillPacket.Topic,
		Payload:   c.WillPacket.Payload,
		QoS:       c.WillPacket.QoS,
		Retain:    c.WillPacket.Retain,
		Timestamp: time.Now().Unix(),
	}

	// 设置遗嘱消息属性
	if c.WillPacket.Properties != nil {
		msg.Priority = c.WillPacket.Properties.Priority
		msg.TraceID = c.WillPacket.Properties.TraceID
		msg.ContentType = c.WillPacket.Properties.ContentType
		msg.ExpiryTime = c.WillPacket.Properties.ExpiryTime
		msg.UserProperties = c.WillPacket.Properties.UserProperties
		msg.TargetClientID = c.WillPacket.Properties.TargetClientID
		msg.ResponseTopic = c.WillPacket.Properties.ResponseTopic
		msg.CorrelationData = c.WillPacket.Properties.CorrelationData
	}

	// 自动填充 SourceClientID（发送者标识，由 core 填充确保可信）
	msg.SourceClientID = c.ID

	// 如果没有设置过期时间，使用默认值
	if msg.ExpiryTime <= 0 && c.core.options.DefaultMessageExpiry > 0 {
		expiry := time.Now().Add(c.core.options.DefaultMessageExpiry)
		msg.ExpiryTime = expiry.Unix()
	}

	c.core.logger.Info("Publishing will message",
		zap.String("clientID", c.ID),
		zap.String("topic", c.WillPacket.Topic))

	c.core.Publish(msg)

	// 清除遗嘱，防止重复发送
	c.WillPacket = nil
}

// ClearWill 清除遗嘱消息（正常断开时调用）
func (c *Client) ClearWill() {
	c.WillPacket = nil
}

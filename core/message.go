package core

import (
	"time"

	"github.com/snple/beacon/packet"
)

// Message 内部消息表示
//
// 设计原则：
// - 内容层：直接引用原始 *packet.PublishPacket（视为只读、可共享）
// - 投递层（PacketID/QoS/Dup）由发送队列/持久化层单独管理
type Message struct {
	// 原始 PUBLISH 包（视为只读）
	Packet *packet.PublishPacket `nson:"pkt"`

	// 固定头部标志
	Dup    bool       `nson:"dup"` // 重发标志
	QoS    packet.QoS `nson:"qos"` // 服务质量
	Retain bool       `nson:"ret"` // 保留标志

	// 发送相关
	PacketID uint16 `nson:"pid"` // 分配的包 ID（仅 QoS 1 有效）

	// 时间相关（非协议字段，仅用于调试/观察）
	Timestamp int64 `nson:"ts"`
}

// IsExpired 检查消息是否已过期
func (m *Message) IsExpired() bool {
	if m.Packet == nil || m.Packet.Properties == nil || m.Packet.Properties.ExpiryTime == 0 {
		return false
	}
	return time.Now().Unix() > m.Packet.Properties.ExpiryTime
}

// Copy 复制消息结构体（浅拷贝 Packet 指针）
func (m *Message) Copy() Message {
	if m == nil {
		return Message{}
	}

	return Message{
		Packet:    m.Packet,
		Dup:       m.Dup,
		QoS:       m.QoS,
		Retain:    m.Retain,
		PacketID:  m.PacketID,
		Timestamp: m.Timestamp,
	}
}

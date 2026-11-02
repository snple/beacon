package core

import (
	"time"

	"github.com/snple/beacon/packet"
)

// messageOverhead 是每条消息除内容之外的近似固定开销（结构体、字符串头等），
// 仅用于内存水位的粗略估算。
const messageOverhead int64 = 256

// Message 是 broker 已接收、正在分发的一条消息的「内容层」。
//
// 内容层在一次发布的生命周期内不可变：主题、属性、载荷由所有订阅者共享。
// 每个订阅者只额外持有一个 delivery（逐客户端的投递参数），因此 1→N 分发
// 不会复制消息内容。
//
// 生命周期由 messageStore 的引用计数管理：全局队列、各客户端队列、待确认
// 列表与保留消息各自持有一个引用，最后一个引用释放后条目即从 map 删除，
// 内容随 Go 的垃圾回收自然回收。
type Message struct {
	// ID 是 broker 生成的唯一 ID，也是 messageStore 的键。
	ID packet.PacketID

	// Packet 是只读的内容层（主题、属性、载荷）。
	Packet *packet.PublishPacket

	// QoS 与 Retain 是发布时的取值。
	QoS    packet.QoS
	Retain bool

	// Timestamp 是 broker 接收该消息的时间（Unix 秒），仅用于观测。
	Timestamp int64

	// body 是预编码的出站帧共享部分，发布时只编码一次。
	body *packet.PublishBody

	// refs 是引用计数，由 messageStore 的锁保护。
	refs int32
}

// newMessage 为已接收的 PUBLISH 包创建共享消息，并预编码出站帧。
// 返回的 Message 尚未注册到 messageStore。
func newMessage(pub *packet.PublishPacket) (*Message, error) {
	body, err := packet.NewPublishBody(pub.Topic, pub.Properties, pub.Payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        packet.NewPacketID(),
		Packet:    pub,
		QoS:       pub.QoS,
		Retain:    pub.Retain,
		Timestamp: time.Now().Unix(),
		body:      body,
	}, nil
}

// IsExpired 检查消息是否已过期
func (m *Message) IsExpired() bool {
	if m == nil || m.Packet == nil || m.Packet.Properties == nil || m.Packet.Properties.ExpiryTime == 0 {
		return false
	}
	return time.Now().Unix() > m.Packet.Properties.ExpiryTime
}

// Topic 返回消息主题。
func (m *Message) Topic() string {
	return m.Packet.Topic
}

// PayloadLen 返回载荷长度。
func (m *Message) PayloadLen() int {
	return len(m.Packet.Payload)
}

// frame 生成面向某个接收者的帧描述（不复制内容）。
func (m *Message) frame(d *delivery) packet.PublishFrame {
	return m.body.Frame(d.packetID, d.qos, d.retain, d.dup.Load())
}

// Size 返回消息内容的近似内存占用。
func (m *Message) Size() int64 {
	if m == nil || m.Packet == nil {
		return 0
	}

	size := int64(len(m.Packet.Topic) + len(m.Packet.Payload))
	if m.Packet.Properties != nil {
		p := m.Packet.Properties
		size += int64(len(p.ContentType) + len(p.TraceID) + len(p.TargetClientID) +
			len(p.SourceClientID) + len(p.ResponseTopic) + len(p.CorrelationData))
		for k, v := range p.UserProperties {
			size += int64(len(k) + len(v))
		}
	}

	return size + messageOverhead
}

// sourceClientID 返回发布者 ID（系统发布时为空）。
func (m *Message) sourceClientID() string {
	if m.Packet.Properties == nil {
		return ""
	}
	return m.Packet.Properties.SourceClientID
}

// targetClientID 返回定向投递目标（非定向消息为空）。
func (m *Message) targetClientID() string {
	if m.Packet.Properties == nil {
		return ""
	}
	return m.Packet.Properties.TargetClientID
}

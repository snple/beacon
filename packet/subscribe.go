package packet

import (
	"bytes"
	"io"
	"strings"

	"github.com/danclive/nson-go"
)

// SubscribeOptions 订阅选项

// - RetainAsPublished 选项:
//   - `true`: 保持原始 Retain 标志
//   - `false`: 发送时清除 Retain 标志
//
// - RetainHandling 模式:
//   - `0`: 总是发送保留消息（默认）
//   - `1`: 仅在新订阅时发送
//   - `2`: 不发送保留消息
type SubscribeOptions struct {
	QoS               QoS
	NoLocal           bool  // 不接收自己发布的消息
	RetainAsPublished bool  // 保留消息按原样发送
	RetainHandling    uint8 // 保留消息处理方式 (0, 1, 2)
}

// SubscribeOptions 位掩码常量
// 位分配:
//   - bit 0: QoS (1 bit, 值为 0 或 1)
//   - bit 1: NoLocal
//   - bit 2: RetainAsPublished
//   - bit 3-4: RetainHandling (2 bits)
const (
	subOptQoS             uint8 = 1 << 0      // bit 0
	subOptNoLocal         uint8 = 1 << 1      // bit 1
	subOptRetainPublished uint8 = 1 << 2      // bit 2
	subOptRetainHandling  uint8 = 0x03 << 3   // bit 3-4, 掩码
	subOptRetainShift     uint8 = 3            // RetainHandling 移位量
)

// Subscription 单个订阅
type Subscription struct {
	Topic   string
	Options SubscribeOptions
}

// ValidateTopicFilter 验证订阅主题过滤器是否有效
// 通配符语法:
//   - "*"  : 单层通配符，匹配一个层级（替代 MQTT 的 +）
//   - "**" : 多层通配符，匹配多个层级（替代 MQTT 的 #），必须在末尾
//
// 返回 true 表示有效
func ValidateTopicFilter(topic string) bool {
	if len(topic) == 0 || len(topic) > MaxTopicLength {
		return false
	}

	// 检查 UTF-8 编码中的空字符
	if strings.ContainsRune(topic, 0) {
		return false
	}

	// 不允许旧的 MQTT 通配符
	if strings.ContainsAny(topic, "+#") {
		return false
	}

	parts := strings.Split(topic, "/")
	for i, part := range parts {
		// "**" 多层通配符必须是最后一个层级且单独出现
		if part == TopicWildcardMulti {
			if i != len(parts)-1 {
				return false
			}
			continue
		}
		// "*" 单层通配符必须单独出现在一个层级
		if strings.Contains(part, TopicWildcardSingle) && part != TopicWildcardSingle {
			return false
		}
	}

	return true
}

// ValidateTopicName 验证发布主题名是否有效（不允许通配符）
func ValidateTopicName(topic string) bool {
	if len(topic) == 0 || len(topic) > MaxTopicLength {
		return false
	}

	// 发布主题不允许新通配符
	if strings.Contains(topic, TopicWildcardSingle) || strings.Contains(topic, TopicWildcardMulti) {
		return false
	}

	// 发布主题不允许旧通配符
	if strings.ContainsAny(topic, "+#") {
		return false
	}

	// 检查 UTF-8 编码中的空字符
	if strings.ContainsRune(topic, 0) {
		return false
	}

	return true
}

// MatchTopic 检查主题名是否匹配主题过滤器
// filter: 订阅的主题过滤器（可包含通配符）
// topic: 发布的主题名（不包含通配符）
func MatchTopic(filter, topic string) bool {
	if filter == topic {
		return true
	}

	filterParts := strings.Split(filter, "/")
	topicParts := strings.Split(topic, "/")

	for i, fp := range filterParts {
		// "**" 匹配剩余所有层级
		if fp == TopicWildcardMulti {
			return true
		}

		// 如果 topic 已经结束，但 filter 还有剩余部分
		if i >= len(topicParts) {
			return false
		}

		// "*" 匹配单个层级
		if fp == TopicWildcardSingle {
			continue
		}

		// 精确匹配
		if fp != topicParts[i] {
			return false
		}
	}

	// filter 已经匹配完，检查 topic 是否也结束
	return len(filterParts) == len(topicParts)
}

// SubscribePacket SUBSCRIBE 数据包
type SubscribePacket struct {
	PacketID      nson.Id
	Properties    *ReasonProperties
	Subscriptions []Subscription
}

// NewSubscribePacket 创建新的 SUBSCRIBE 包
func NewSubscribePacket(packetID nson.Id) *SubscribePacket {
	return &SubscribePacket{
		PacketID:   packetID,
		Properties: NewReasonProperties(),
	}
}

// AddSubscription 添加订阅
func (p *SubscribePacket) AddSubscription(topic string, qos QoS) {
	p.Subscriptions = append(p.Subscriptions, Subscription{
		Topic: topic,
		Options: SubscribeOptions{
			QoS: qos,
		},
	})
}

func (p *SubscribePacket) Type() PacketType {
	return SUBSCRIBE
}

func (p *SubscribePacket) encode(w io.Writer) error {
	// 包标识符
	if err := EncodeId(w, p.PacketID); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}
	if err := p.Properties.Encode(w); err != nil {
		return err
	}

	// 订阅列表
	for _, sub := range p.Subscriptions {
		if err := EncodeString(w, sub.Topic); err != nil {
			return err
		}
		// 订阅选项编码 (使用位掩码)
		var options byte
		options |= byte(sub.Options.QoS) & subOptQoS
		if sub.Options.NoLocal {
			options |= subOptNoLocal
		}
		if sub.Options.RetainAsPublished {
			options |= subOptRetainPublished
		}
		options |= (sub.Options.RetainHandling & 0x03) << subOptRetainShift
		if err := WriteByte(w, options); err != nil {
			return err
		}
	}

	return nil
}

func (p *SubscribePacket) decode(r io.Reader, header FixedHeader) error {
	// 包标识符
	var err error
	p.PacketID, err = DecodeId(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewReasonProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 读取剩余数据作为订阅列表
	if br, ok := r.(*bytes.Reader); ok {
		for br.Len() > 0 {
			topic, err := DecodeString(br)
			if err != nil {
				break
			}

			var options [1]byte
			if _, err := io.ReadFull(br, options[:]); err != nil {
				break
			}

			sub := Subscription{
				Topic: topic,
				Options: SubscribeOptions{
					QoS:               QoS(options[0] & subOptQoS),
					NoLocal:           options[0]&subOptNoLocal != 0,
					RetainAsPublished: options[0]&subOptRetainPublished != 0,
					RetainHandling:    (options[0] & subOptRetainHandling) >> subOptRetainShift,
				},
			}
			p.Subscriptions = append(p.Subscriptions, sub)
		}
	}

	return nil
}

// SubackPacket SUBACK 数据包
type SubackPacket struct {
	PacketID    nson.Id
	Properties  *ReasonProperties
	ReasonCodes []ReasonCode
}

// NewSubackPacket 创建新的 SUBACK 包
func NewSubackPacket(packetID nson.Id) *SubackPacket {
	return &SubackPacket{
		PacketID:   packetID,
		Properties: NewReasonProperties(),
	}
}

func (p *SubackPacket) Type() PacketType {
	return SUBACK
}

func (p *SubackPacket) encode(w io.Writer) error {
	// 包标识符
	if err := EncodeId(w, p.PacketID); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}
	if err := p.Properties.Encode(w); err != nil {
		return err
	}

	// 原因码列表
	for _, code := range p.ReasonCodes {
		if err := WriteByte(w, byte(code)); err != nil {
			return err
		}
	}

	return nil
}

func (p *SubackPacket) decode(r io.Reader, header FixedHeader) error {
	// 包标识符
	var err error
	p.PacketID, err = DecodeId(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewReasonProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 原因码列表
	if br, ok := r.(*bytes.Reader); ok {
		for br.Len() > 0 {
			code, err := br.ReadByte()
			if err != nil {
				break
			}
			p.ReasonCodes = append(p.ReasonCodes, ReasonCode(code))
		}
	}

	return nil
}

// UnsubscribePacket UNSUBSCRIBE 数据包
type UnsubscribePacket struct {
	PacketID   nson.Id
	Properties *ReasonProperties
	Topics     []string
}

// NewUnsubscribePacket 创建新的 UNSUBSCRIBE 包
func NewUnsubscribePacket(packetID nson.Id) *UnsubscribePacket {
	return &UnsubscribePacket{
		PacketID:   packetID,
		Properties: NewReasonProperties(),
	}
}

func (p *UnsubscribePacket) Type() PacketType {
	return UNSUBSCRIBE
}

func (p *UnsubscribePacket) encode(w io.Writer) error {
	// 包标识符
	if err := EncodeId(w, p.PacketID); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}
	if err := p.Properties.Encode(w); err != nil {
		return err
	}

	// 主题列表
	for _, topic := range p.Topics {
		if err := EncodeString(w, topic); err != nil {
			return err
		}
	}

	return nil
}

func (p *UnsubscribePacket) decode(r io.Reader, header FixedHeader) error {
	// 包标识符
	var err error
	p.PacketID, err = DecodeId(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewReasonProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 主题列表
	if br, ok := r.(*bytes.Reader); ok {
		for br.Len() > 0 {
			topic, err := DecodeString(br)
			if err != nil {
				break
			}
			p.Topics = append(p.Topics, topic)
		}
	}

	return nil
}

// UnsubackPacket UNSUBACK 数据包
type UnsubackPacket struct {
	PacketID    nson.Id
	Properties  *ReasonProperties
	ReasonCodes []ReasonCode
}

// NewUnsubackPacket 创建新的 UNSUBACK 包
func NewUnsubackPacket(packetID nson.Id) *UnsubackPacket {
	return &UnsubackPacket{
		PacketID:   packetID,
		Properties: NewReasonProperties(),
	}
}

func (p *UnsubackPacket) Type() PacketType {
	return UNSUBACK
}

func (p *UnsubackPacket) encode(w io.Writer) error {

	// 包标识符
	if err := EncodeId(w, p.PacketID); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}
	if err := p.Properties.Encode(w); err != nil {
		return err
	}

	// 原因码列表
	for _, code := range p.ReasonCodes {
		if err := WriteByte(w, byte(code)); err != nil {
			return err
		}
	}

	return nil
}

func (p *UnsubackPacket) decode(r io.Reader, header FixedHeader) error {
	// 包标识符
	var err error
	p.PacketID, err = DecodeId(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewReasonProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 原因码列表
	if br, ok := r.(*bytes.Reader); ok {
		for br.Len() > 0 {
			code, err := br.ReadByte()
			if err != nil {
				break
			}
			p.ReasonCodes = append(p.ReasonCodes, ReasonCode(code))
		}
	}

	return nil
}

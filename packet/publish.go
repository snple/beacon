package packet

import (
	"bytes"
	"fmt"
	"io"

	"github.com/danclive/nson-go"
)

// PublishPacket PUBLISH 数据包
type PublishPacket struct {
	// 固定头部标志
	Dup    bool `nson:"dup"` // 重发标志
	QoS    QoS  `nson:"qos"` // 服务质量
	Retain bool `nson:"ret"` // 保留标志

	// 主题名
	Topic string `nson:"topic"`

	// 包标识符 (QoS > 0 时存在)
	PacketID nson.Id `nson:"pid"`

	// 属性
	Properties *PublishProperties `nson:"props"`

	// 载荷（如果 Compression != None，则为压缩后的数据）
	Payload []byte `nson:"payload"`
}

// NewPublishPacket 创建新的 PUBLISH 包
func NewPublishPacket(topic string, payload []byte) *PublishPacket {
	return &PublishPacket{
		Topic:      topic,
		Payload:    payload,
		QoS:        QoS0,
		Properties: NewPublishProperties(),
	}
}

// Copy 复制 PUBACK 包, 浅拷贝 Properties
func (p *PublishPacket) Copy() PublishPacket {
	if p == nil {
		return PublishPacket{}
	}

	return PublishPacket{
		Dup:        p.Dup,
		QoS:        p.QoS,
		Retain:     p.Retain,
		Topic:      p.Topic,
		PacketID:   p.PacketID,
		Properties: p.Properties,
		Payload:    p.Payload,
	}
}

func (p *PublishPacket) Type() PacketType {
	return PUBLISH
}

func (p *PublishPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// Flags 字节 (更紧凑的布局)
	// Bit 0: QoS (0 或 1)
	// Bit 1: Retain
	// Bit 2: Dup
	// Bit 3-7: 保留位
	var flags uint8
	flags |= uint8(p.QoS) & 0x01 // Bit 0: QoS
	if p.Retain {
		flags |= 0x02 // Bit 1: Retain
	}
	if p.Dup {
		flags |= 0x04 // Bit 2: Dup
	}
	buf.WriteByte(flags)

	// 主题名
	if err := EncodeString(&buf, p.Topic); err != nil {
		return err
	}

	// 包标识符 (QoS > 0)
	if p.QoS > 0 {
		if err := EncodeId(&buf, p.PacketID); err != nil {
			return err
		}
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewPublishProperties()
	}
	if err := p.Properties.Encode(&buf); err != nil {
		return err
	}

	// 载荷
	buf.Write(p.Payload)

	// 固定头部
	header := FixedHeader{
		Type:      PUBLISH,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *PublishPacket) Decode(r io.Reader, header FixedHeader) error {
	// Flags 字节 (紧凑布局)
	// Bit 0: QoS (0 或 1)
	// Bit 1: Retain
	// Bit 2: Dup
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return err
	}
	p.QoS = QoS(flags[0] & 0x01)  // Bit 0: QoS
	p.Retain = flags[0]&0x02 != 0 // Bit 1: Retain
	p.Dup = flags[0]&0x04 != 0    // Bit 2: Dup

	// 主题名
	var err error
	p.Topic, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 包标识符 (QoS > 0)
	if p.QoS > 0 {
		p.PacketID, err = DecodeId(r)
		if err != nil {
			return err
		}
	}

	// 属性
	p.Properties = NewPublishProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 读取剩余数据计算属性长度有点复杂，改用读取所有剩余数据
	// 这里假设 r 是 bytes.Reader，可以获取剩余长度
	if br, ok := r.(*bytes.Reader); ok {
		p.Payload = make([]byte, br.Len())
		_, err = io.ReadFull(r, p.Payload)
	} else {
		// 兼容处理：需要计算属性编码后的长度
		// 简化处理：读取剩余所有数据
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		if err != nil && err != io.EOF {
			return err
		}
		p.Payload = buf.Bytes()
	}

	return err
}

func (p *PublishPacket) String() string {
	return fmt.Sprintf("PUBLISH Topic=%s PacketID=%s QoS=%d Retain=%t Dup=%t Properties=%v PayloadLen=%d",
		p.Topic, p.PacketID.Hex(), p.QoS, p.Retain, p.Dup, p.Properties, len(p.Payload))
}

// PubackPacket PUBACK 数据包
type PubackPacket struct {
	PacketID   nson.Id
	ReasonCode ReasonCode
	Properties *ReasonProperties
}

// NewPubackPacket 创建新的 PUBACK 包
func NewPubackPacket(packetID nson.Id, code ReasonCode) *PubackPacket {
	return &PubackPacket{
		PacketID:   packetID,
		ReasonCode: code,
		Properties: NewReasonProperties(),
	}
}

func (p *PubackPacket) Type() PacketType {
	return PUBACK
}

func (p *PubackPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 包标识符
	if err := EncodeId(&buf, p.PacketID); err != nil {
		return err
	}

	// 原因码 (如果是成功且无属性，可以省略)
	if p.ReasonCode != ReasonSuccess || (p.Properties != nil && len(p.Properties.UserProperties) > 0) {
		buf.WriteByte(byte(p.ReasonCode))

		// 属性
		if p.Properties == nil {
			p.Properties = NewReasonProperties()
		}
		if err := p.Properties.Encode(&buf); err != nil {
			return err
		}
	}

	// 固定头部
	header := FixedHeader{
		Type:      PUBACK,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *PubackPacket) Decode(r io.Reader, header FixedHeader) error {
	// 包标识符
	var err error
	p.PacketID, err = DecodeId(r)
	if err != nil {
		return err
	}

	if header.Remaining > 2 {
		// 原因码
		var code [1]byte
		if _, err := io.ReadFull(r, code[:]); err != nil {
			return err
		}
		p.ReasonCode = ReasonCode(code[0])

		// 属性
		if header.Remaining > 3 {
			p.Properties = NewReasonProperties()
			if err := p.Properties.Decode(r); err != nil {
				return err
			}
		}
	} else {
		p.ReasonCode = ReasonSuccess
	}

	return nil
}

func (p *PubackPacket) String() string {
	return fmt.Sprintf("PUBACK PacketID=%s ReasonCode=%d Properties=%v",
		p.PacketID.Hex(), p.ReasonCode, p.Properties)
}

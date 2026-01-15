package packet

import (
	"bytes"
	"io"
)

// ConnectFlags CONNECT 包的标志位
type ConnectFlags struct {
	CleanSession bool // 清理会话
	Will         bool // 遗嘱标志
}

// ConnectPacket CONNECT 数据包
type ConnectPacket struct {
	// 协议信息
	ProtocolName    string
	ProtocolVersion uint8

	// 连接标志
	Flags ConnectFlags

	// 客户端标识
	ClientID string

	// Keep Alive
	KeepAlive uint16

	// 属性
	Properties *ConnectProperties

	// 遗嘱消息
	WillPacket *PublishPacket
}

// NewConnectPacket 创建新的 CONNECT 包
func NewConnectPacket() *ConnectPacket {
	return &ConnectPacket{
		ProtocolName:    ProtocolName,
		ProtocolVersion: ProtocolVersion,
		Properties:      NewConnectProperties(),
		KeepAlive:       DefaultKeepAlive,
	}
}

func (p *ConnectPacket) Type() PacketType {
	return CONNECT
}

func (p *ConnectPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 协议名称
	if err := EncodeString(&buf, p.ProtocolName); err != nil {
		return err
	}

	// 协议版本
	buf.WriteByte(p.ProtocolVersion)

	// 连接标志 (包含 CleanSession, Will)
	// 注意: Will QoS 和 Will Retain 现在在遗嘱消息的 Flags 字节中编码
	var flags byte
	if p.Flags.CleanSession {
		flags |= 0x02
	}
	if p.Flags.Will {
		flags |= 0x04
	}
	buf.WriteByte(flags)

	// 客户端标识
	if err := EncodeString(&buf, p.ClientID); err != nil {
		return err
	}

	// Keep Alive
	if err := EncodeUint16(&buf, p.KeepAlive); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewConnectProperties()
	}
	if err := p.Properties.Encode(&buf); err != nil {
		return err
	}

	// 遗嘱消息
	if p.Flags.Will && p.WillPacket != nil {
		// 遗嘱 Flags 字节 (紧凑布局) - 与 PublishPacket 一致
		// Bit 0: QoS (0 或 1)
		// Bit 1: Retain
		// Bit 2: Dup
		var willFlags uint8
		willFlags |= uint8(p.WillPacket.QoS) & 0x01 // Bit 0: QoS
		if p.WillPacket.Retain {
			willFlags |= 0x02 // Bit 1: Retain
		}
		if p.WillPacket.Dup {
			willFlags |= 0x04 // Bit 2: Dup
		}
		buf.WriteByte(willFlags)

		// 遗嘱主题
		if err := EncodeString(&buf, p.WillPacket.Topic); err != nil {
			return err
		}

		// 遗嘱属性
		if p.WillPacket.Properties == nil {
			p.WillPacket.Properties = NewPublishProperties()
		}
		if err := p.WillPacket.Properties.Encode(&buf); err != nil {
			return err
		}

		// 遗嘱载荷 (直接写入，与 PublishPacket 一致)
		buf.Write(p.WillPacket.Payload)
	}

	// 固定头部
	header := FixedHeader{
		Type:      CONNECT,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *ConnectPacket) Decode(r io.Reader, header FixedHeader) error {
	// 协议名称
	var err error
	p.ProtocolName, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 协议版本
	var version [1]byte
	if _, err := io.ReadFull(r, version[:]); err != nil {
		return err
	}
	p.ProtocolVersion = version[0]

	// 连接标志
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return err
	}
	p.Flags.CleanSession = flags[0]&0x02 != 0
	p.Flags.Will = flags[0]&0x04 != 0

	// 注意: Will QoS 和 Will Retain 现在在遗嘱消息的 Flags 字节中编码，而不是在 Connect Flags 中

	// 客户端标识
	p.ClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	// Keep Alive
	p.KeepAlive, err = DecodeUint16(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewConnectProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 遗嘱消息
	if p.Flags.Will {
		p.WillPacket = &PublishPacket{}

		// 遗嘱 Flags 字节 (紧凑布局) - 与 PublishPacket 一致
		// Bit 0: QoS (0 或 1)
		// Bit 1: Retain
		// Bit 2: Dup
		var willFlags [1]byte
		if _, err := io.ReadFull(r, willFlags[:]); err != nil {
			return err
		}
		p.WillPacket.QoS = QoS(willFlags[0] & 0x01)  // Bit 0: QoS
		p.WillPacket.Retain = willFlags[0]&0x02 != 0 // Bit 1: Retain
		p.WillPacket.Dup = willFlags[0]&0x04 != 0    // Bit 2: Dup

		// 遗嘱主题
		p.WillPacket.Topic, err = DecodeString(r)
		if err != nil {
			return err
		}

		// 遗嘱属性
		p.WillPacket.Properties = NewPublishProperties()
		if err := p.WillPacket.Properties.Decode(r); err != nil {
			return err
		}

		// 遗嘱载荷 (读取剩余所有数据，与 PublishPacket 一致)
		if br, ok := r.(*bytes.Reader); ok {
			p.WillPacket.Payload = make([]byte, br.Len())
			_, err = io.ReadFull(r, p.WillPacket.Payload)
		} else {
			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, r)
			if err != nil && err != io.EOF {
				return err
			}
			p.WillPacket.Payload = buf.Bytes()
		}
	}

	return nil
}

// ConnackPacket CONNACK 数据包
type ConnackPacket struct {
	// 会话存在标志
	SessionPresent bool

	// 原因码
	ReasonCode ReasonCode

	// 属性
	Properties *ConnackProperties
}

// NewConnackPacket 创建新的 CONNACK 包
func NewConnackPacket(code ReasonCode) *ConnackPacket {
	return &ConnackPacket{
		ReasonCode: code,
		Properties: NewConnackProperties(),
	}
}

func (p *ConnackPacket) Type() PacketType {
	return CONNACK
}

func (p *ConnackPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 会话存在标志
	var flags byte
	if p.SessionPresent {
		flags |= 0x01
	}
	buf.WriteByte(flags)

	// 原因码
	buf.WriteByte(byte(p.ReasonCode))

	// 属性
	if p.Properties == nil {
		p.Properties = NewConnackProperties()
	}
	if err := p.Properties.Encode(&buf); err != nil {
		return err
	}

	// 固定头部
	header := FixedHeader{
		Type:      CONNACK,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *ConnackPacket) Decode(r io.Reader, header FixedHeader) error {
	// 连接确认标志
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return err
	}
	p.SessionPresent = flags[0]&0x01 != 0

	// 原因码
	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code[0])

	// 属性
	p.Properties = NewConnackProperties()
	return p.Properties.Decode(r)
}

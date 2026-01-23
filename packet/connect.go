package packet

import (
	"bytes"
	"io"
)

// ConnectPacket CONNECT 数据包
type ConnectPacket struct {
	// 协议信息
	ProtocolName    string
	ProtocolVersion uint8

	// 客户端标识
	ClientID string

	// 连接保持
	KeepAlive uint16 // 心跳间隔（秒）

	// 会话管理
	SessionTimeout uint32 // 会话过期时间（秒）

	// 属性
	Properties *ConnectProperties

	// 遗嘱消息
	Will       bool // 遗嘱标志
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

func (p *ConnectPacket) encode(w io.Writer) error {
	// 协议名称
	if err := EncodeString(w, p.ProtocolName); err != nil {
		return err
	}

	// 协议版本
	if err := WriteByte(w, p.ProtocolVersion); err != nil {
		return err
	}

	// 客户端标识
	if err := EncodeString(w, p.ClientID); err != nil {
		return err
	}

	// KeepAlive
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
		return err
	}

	// SessionTimeout
	if err := EncodeUint32(w, p.SessionTimeout); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewConnectProperties()
	}
	if err := p.Properties.Encode(w); err != nil {
		return err
	}

	// 遗嘱消息
	if p.Will && p.WillPacket != nil {
		// 遗嘱 Flags 字节 (紧凑布局) - 与 PublishPacket 一致
		// Bit 0: QoS (0 或 1)
		// Bit 1: Retain
		// Bit 2: Dup
		// Bit 3: Will
		var willFlags uint8
		willFlags |= uint8(p.WillPacket.QoS) & 0x01 // Bit 0: QoS
		if p.WillPacket.Retain {
			willFlags |= 0x02 // Bit 1: Retain
		}
		if p.WillPacket.Dup {
			willFlags |= 0x04 // Bit 2: Dup
		}
		willFlags |= 0x08 // Bit 3: Will

		// 写入遗嘱 Flags 字节
		if err := WriteByte(w, willFlags); err != nil {
			return err
		}

		// 遗嘱主题
		if err := EncodeString(w, p.WillPacket.Topic); err != nil {
			return err
		}

		// 遗嘱属性
		if p.WillPacket.Properties == nil {
			p.WillPacket.Properties = NewPublishProperties()
		}
		if err := p.WillPacket.Properties.Encode(w); err != nil {
			return err
		}

		// 遗嘱载荷 (直接写入，与 PublishPacket 一致)
		if _, err := w.Write(p.WillPacket.Payload); err != nil {
			return err
		}
	} else {
		// 如果没有遗嘱消息,写入一个空的遗嘱标志字节
		if err := WriteByte(w, 0x00); err != nil {
			return err
		}
	}

	return nil
}

func (p *ConnectPacket) decode(r io.Reader, header FixedHeader) error {
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

	// Session Timeout
	p.SessionTimeout, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewConnectProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 读取遗嘱 Flags 字节
	var willFlags [1]byte
	if _, err := io.ReadFull(r, willFlags[:]); err != nil {
		return err
	}
	p.Will = willFlags[0]&0x08 != 0 // Bit 3: Will

	// 遗嘱消息
	if p.Will {
		p.WillPacket = &PublishPacket{}

		// 遗嘱 Flags 字节 (紧凑布局) - 与 PublishPacket 一致
		// Bit 0: QoS (0 或 1)
		// Bit 1: Retain
		// Bit 2: Dup
		// Bit 3: Will
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

func (p *ConnackPacket) encode(w io.Writer) error {
	// 会话存在标志
	var flags byte
	if p.SessionPresent {
		flags |= 0x01
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}

	// 原因码
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewConnackProperties()
	}
	return p.Properties.Encode(w)
}

func (p *ConnackPacket) decode(r io.Reader, header FixedHeader) error {
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

package packet

import (
	"io"
)

// ConnectPacket TCP连接认证请求包
type ConnectPacket struct {
	ClientID string // 客户端ID（必填）

	// 认证
	AuthMethod string // 认证方法
	AuthData   []byte // 认证数据

	KeepAlive  uint16            // 保活时间（秒）
	Properties map[string]string // 扩展属性
}

func (p *ConnectPacket) Type() PacketType {
	return CONNECT
}

func (p *ConnectPacket) encode(w io.Writer) error {
	// ClientID
	if err := EncodeString(w, p.ClientID); err != nil {
		return err
	}

	// AuthMethod
	if err := EncodeString(w, p.AuthMethod); err != nil {
		return err
	}

	// AuthData
	if err := EncodeBinary(w, p.AuthData); err != nil {
		return err
	}

	// KeepAlive
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
		return err
	}

	// Properties
	return EncodeProperties(w, p.Properties)
}

func (p *ConnectPacket) decode(r io.Reader) error {
	var err error

	p.ClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.AuthMethod, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.AuthData, err = DecodeBinary(r)
	if err != nil {
		return err
	}

	p.KeepAlive, err = DecodeUint16(r)
	if err != nil {
		return err
	}

	p.Properties, err = DecodeProperties(r)
	return err
}

// ConnackPacket TCP连接认证响应包
type ConnackPacket struct {
	ReasonCode ReasonCode        // 原因码
	ServerID   string            // 服务端ID
	Message    string            // 可选消息
	Properties map[string]string // 扩展属性
}

func (p *ConnackPacket) Type() PacketType {
	return CONNACK
}

func (p *ConnackPacket) encode(w io.Writer) error {
	// ReasonCode
	if err := EncodeUint8(w, uint8(p.ReasonCode)); err != nil {
		return err
	}

	// ServerID
	if err := EncodeString(w, p.ServerID); err != nil {
		return err
	}

	// Message
	if err := EncodeString(w, p.Message); err != nil {
		return err
	}

	// Properties
	return EncodeProperties(w, p.Properties)
}

func (p *ConnackPacket) decode(r io.Reader) error {
	code, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code)

	p.ServerID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Message, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Properties, err = DecodeProperties(r)
	return err
}

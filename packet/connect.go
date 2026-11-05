package packet

import (
	"io"
)

// ChannelDecl 客户端声明的一项 channel 兴趣。
type ChannelDecl struct {
	Channel string
}

// ConnectPacket CONNECT：建立连接、单轮认证、首次声明。
type ConnectPacket struct {
	Version        uint8
	ClientID       string
	KeepAlive      uint16 // 秒
	SessionTimeout uint32 // 秒；0 表示断开即清理
	MaxPacketSize  uint32

	HasWill     bool
	WillChannel string
	WillPayload []byte
	WillHeaders map[string]string

	AuthMethod string
	AuthData   []byte

	Channels []ChannelDecl
}

// NewConnectPacket 创建 CONNECT 包。
func NewConnectPacket(clientID string) *ConnectPacket {
	return &ConnectPacket{
		Version:   ProtocolVersion,
		ClientID:  clientID,
		KeepAlive: DefaultKeepAlive,
	}
}

func (p *ConnectPacket) Type() PacketType { return CONNECT }

func (p *ConnectPacket) encode(w io.Writer) error {
	if err := WriteByte(w, p.Version); err != nil {
		return err
	}
	if err := EncodeString(w, p.ClientID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.SessionTimeout); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.MaxPacketSize); err != nil {
		return err
	}

	var willFlags uint8
	if p.HasWill {
		willFlags |= 1
	}
	if err := WriteByte(w, willFlags); err != nil {
		return err
	}
	if err := EncodeString(w, p.WillChannel); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.WillPayload); err != nil {
		return err
	}
	if err := EncodeStringMap(w, p.WillHeaders); err != nil {
		return err
	}

	if err := EncodeString(w, p.AuthMethod); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.AuthData); err != nil {
		return err
	}

	if len(p.Channels) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.Channels))); err != nil {
		return err
	}
	for _, ch := range p.Channels {
		if err := EncodeString(w, ch.Channel); err != nil {
			return err
		}
	}
	return nil
}

func (p *ConnectPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.Version, err = ReadByte(r); err != nil {
		return err
	}
	if p.ClientID, err = DecodeString(r); err != nil {
		return err
	}
	if p.KeepAlive, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.SessionTimeout, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.MaxPacketSize, err = DecodeUint32(r); err != nil {
		return err
	}

	willFlags, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.HasWill = willFlags&1 != 0
	if p.WillChannel, err = DecodeString(r); err != nil {
		return err
	}
	if p.WillPayload, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.WillHeaders, err = DecodeStringMap(r); err != nil {
		return err
	}

	if p.AuthMethod, err = DecodeString(r); err != nil {
		return err
	}
	if p.AuthData, err = DecodeBytes32(r); err != nil {
		return err
	}

	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	if count > 0 {
		if err := checkCount(r, count, 1); err != nil {
			return err
		}
		p.Channels = make([]ChannelDecl, 0, count)
		for i := 0; i < int(count); i++ {
			ch, err := DecodeString(r)
			if err != nil {
				return err
			}
			p.Channels = append(p.Channels, ChannelDecl{Channel: ch})
		}
	}
	return nil
}

// ConnackPacket CONNACK：连接响应。
type ConnackPacket struct {
	SessionPresent      bool
	ReasonCode          ReasonCode
	AssignedClientID    string
	ServerMaxPacketSize uint32
	// KeepAlive 是服务端要求的心跳间隔（秒）；客户端应以此覆盖自己的值。
	KeepAlive         uint16
	SessionID         uint64
	Pending           [3]uint32
	Delivered         uint32
	OldestUncommitted PacketID
}

// NewConnackPacket 创建 CONNACK 包。
func NewConnackPacket(reason ReasonCode) *ConnackPacket {
	return &ConnackPacket{ReasonCode: reason}
}

func (p *ConnackPacket) Type() PacketType { return CONNACK }

func (p *ConnackPacket) encode(w io.Writer) error {
	var present uint8
	if p.SessionPresent {
		present = 1
	}
	if err := WriteByte(w, present); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}
	if err := EncodeString(w, p.AssignedClientID); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.ServerMaxPacketSize); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.SessionID); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	if err := EncodeUint32(w, p.Delivered); err != nil {
		return err
	}
	return EncodePacketID(w, p.OldestUncommitted)
}

func (p *ConnackPacket) decode(r io.Reader, _ FixedHeader) error {
	present, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.SessionPresent = present&1 != 0

	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)

	if p.AssignedClientID, err = DecodeString(r); err != nil {
		return err
	}
	if p.ServerMaxPacketSize, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.KeepAlive, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.SessionID, err = DecodeUint64(r); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	if p.Delivered, err = DecodeUint32(r); err != nil {
		return err
	}
	p.OldestUncommitted, err = DecodePacketID(r)
	return err
}

package packet

import (
	"io"
)

// ChannelDecl 客户端声明的一项 channel 兴趣。
type ChannelDecl struct {
	Channel string
}

// ConnectFlags 是 CONNECT 的会话与能力开关。
//
// 用一个字节承载全部开关，后续新增能力时无需再扩展 CONNECT 布局。
type ConnectFlags uint8

const (
	// ConnectFlagCleanSession 丢弃该 ClientID 的既有会话状态
	// （pending/delivered/声明），以全新会话建立本次连接。
	ConnectFlagCleanSession ConnectFlags = 1 << 0
)

const connectFlagsKnown = ConnectFlagCleanSession

// Unknown 返回未定义的标志位，供服务端拒绝无法理解的 CONNECT。
func (f ConnectFlags) Unknown() ConnectFlags { return f &^ connectFlagsKnown }

// HasFlag 报告某个开关是否置位。
func (p *ConnectPacket) HasFlag(flag ConnectFlags) bool { return p.Flags&flag != 0 }

// SetFlag 设置或清除某个开关。
func (p *ConnectPacket) SetFlag(flag ConnectFlags, on bool) {
	if on {
		p.Flags |= flag
	} else {
		p.Flags &^= flag
	}
}

// CleanSession 报告是否要求新建会话。
func (p *ConnectPacket) CleanSession() bool { return p.HasFlag(ConnectFlagCleanSession) }

// SetCleanSession 设置是否要求新建会话。
func (p *ConnectPacket) SetCleanSession(on bool) { p.SetFlag(ConnectFlagCleanSession, on) }

// ConnectPacket CONNECT：建立连接与单轮认证。
//
// 订阅声明不属于 CONNECT：连接建立后由 DECLARE 全量替换，因此重连时
// 既不会携带过期声明，也必须在恢复的会话上重新声明或沿用服务端保留的声明。
type ConnectPacket struct {
	Version uint8
	// Flags 承载会话与能力开关，见 ConnectFlag*。
	Flags         ConnectFlags
	ClientID      string
	KeepAlive     uint16 // 秒
	MaxPacketSize uint32

	HasWill     bool
	WillChannel string
	WillPayload []byte
	WillHeaders map[string]string

	AuthMethod string
	AuthData   []byte
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
	if err := WriteByte(w, byte(p.Flags)); err != nil {
		return err
	}
	if err := EncodeString(w, p.ClientID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
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
	return EncodeBytes32(w, p.AuthData)
}

func (p *ConnectPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.Version, err = ReadByte(r); err != nil {
		return err
	}
	flags, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Flags = ConnectFlags(flags)
	if p.ClientID, err = DecodeString(r); err != nil {
		return err
	}
	if p.KeepAlive, err = DecodeUint16(r); err != nil {
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
	p.AuthData, err = DecodeBytes32(r)
	return err
}

// ConnackFlags 是 CONNACK 的会话与能力开关。
type ConnackFlags uint8

const (
	// ConnackFlagSessionPresent 表示复用了既有会话。
	ConnackFlagSessionPresent ConnackFlags = 1 << 0
)

// HasFlag 报告某个开关是否置位。
func (p *ConnackPacket) HasFlag(flag ConnackFlags) bool { return p.Flags&flag != 0 }

// SetFlag 设置或清除某个开关。
func (p *ConnackPacket) SetFlag(flag ConnackFlags, on bool) {
	if on {
		p.Flags |= flag
	} else {
		p.Flags &^= flag
	}
}

// SessionPresent 报告是否复用了既有会话。
func (p *ConnackPacket) SessionPresent() bool { return p.HasFlag(ConnackFlagSessionPresent) }

// SetSessionPresent 设置是否复用既有会话。
func (p *ConnackPacket) SetSessionPresent(on bool) { p.SetFlag(ConnackFlagSessionPresent, on) }

// ConnackPacket CONNACK：连接响应。
type ConnackPacket struct {
	// Flags 承载会话与能力开关，见 ConnackFlag*。
	Flags               ConnackFlags
	ReasonCode          ReasonCode
	AssignedClientID    string
	ServerMaxPacketSize uint32
	// KeepAlive 是服务端要求的心跳间隔（秒）；客户端应以此覆盖自己的值。
	KeepAlive uint16
	SessionID uint64
	Pending   [3]uint32
}

// NewConnackPacket 创建 CONNACK 包。
func NewConnackPacket(reason ReasonCode) *ConnackPacket {
	return &ConnackPacket{ReasonCode: reason}
}

func (p *ConnackPacket) Type() PacketType { return CONNACK }

func (p *ConnackPacket) encode(w io.Writer) error {
	if err := WriteByte(w, byte(p.Flags)); err != nil {
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
	return nil
}

func (p *ConnackPacket) decode(r io.Reader, _ FixedHeader) error {
	flags, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Flags = ConnackFlags(flags)

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
	return nil
}

package packet

import (
	"io"
)

// ConnectFlags 是 CONNECT 的会话与能力开关。
//
// 用一个字节承载全部开关，后续新增能力时无需再扩展 CONNECT 布局。
type ConnectFlags uint8

const (
	// ConnectFlagCleanSession 丢弃该 ClientID 的既有会话状态
	// （pending/delivered/声明），以全新会话建立本次连接。
	ConnectFlagCleanSession ConnectFlags = 1 << 0
	// ConnectFlagHasProperties 表示 CONNECT 末尾携带 Properties。
	ConnectFlagHasProperties ConnectFlags = 1 << 1
)

const connectFlagsKnown = ConnectFlagCleanSession | ConnectFlagHasProperties

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

	AuthMethod string
	AuthData   []byte
	// Properties 是客户端随 CONNECT 发送给服务端的键值信息。
	// broker 本身不解释这些键；认证/连接钩子可以读取。
	Properties map[string]string
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
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= ConnectFlagHasProperties
	} else {
		flags &^= ConnectFlagHasProperties
	}
	if err := WriteByte(w, p.Version); err != nil {
		return err
	}
	if err := WriteByte(w, byte(flags)); err != nil {
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

	if err := EncodeString(w, p.AuthMethod); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.AuthData); err != nil {
		return err
	}
	if flags&ConnectFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
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
	if err := checkUnknownFlags(flags, uint8(connectFlagsKnown)); err != nil {
		return err
	}
	if p.ClientID, err = DecodeString(r); err != nil {
		return err
	}
	if p.KeepAlive, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.MaxPacketSize, err = DecodeUint32(r); err != nil {
		return err
	}

	if p.AuthMethod, err = DecodeString(r); err != nil {
		return err
	}
	if p.AuthData, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Flags&ConnectFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

// ConnackFlags 是 CONNACK 的会话与能力开关。
type ConnackFlags uint8

const (
	// ConnackFlagSessionPresent 表示复用了既有会话。
	ConnackFlagSessionPresent ConnackFlags = 1 << 0
	// ConnackFlagHasProperties 表示 CONNACK 末尾携带 Properties。
	ConnackFlagHasProperties ConnackFlags = 1 << 1
)

const connackFlagsKnown = ConnackFlagSessionPresent | ConnackFlagHasProperties

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
	Flags            ConnackFlags
	ReasonCode       ReasonCode
	AssignedClientID string
	// SessionID 是本次会话的租约标识；重连恢复同一会话时保持不变。
	SessionID uint64
	// KeepAlive 是服务端要求的心跳间隔（秒）；客户端应以此覆盖自己的值。
	KeepAlive uint16
	// ReceiveMaximum 是服务端为该会话采用的未确认可靠消息窗口。
	ReceiveMaximum uint16
	// Pending 是 Low / Medium / High 三个优先级的未投递积压。
	Pending [3]uint32
	// Limits 是服务端对连接/会话施加的主要限制，其中 MaxPacketSize
	// 也是客户端发送包的上限。
	Limits Limits
	// Properties 是服务端随 CONNACK 返回给客户端的键值信息。
	Properties map[string]string
}

// NewConnackPacket 创建 CONNACK 包。
func NewConnackPacket(reason ReasonCode) *ConnackPacket {
	return &ConnackPacket{ReasonCode: reason}
}

func (p *ConnackPacket) Type() PacketType { return CONNACK }

func (p *ConnackPacket) encode(w io.Writer) error {
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= ConnackFlagHasProperties
	} else {
		flags &^= ConnackFlagHasProperties
	}
	if err := WriteByte(w, byte(flags)); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}
	if err := EncodeString(w, p.AssignedClientID); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.SessionID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.KeepAlive); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.ReceiveMaximum); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	if err := p.Limits.encode(w); err != nil {
		return err
	}
	if flags&ConnackFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
}

func (p *ConnackPacket) decode(r io.Reader, _ FixedHeader) error {
	flags, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Flags = ConnackFlags(flags)
	if err := checkUnknownFlags(flags, uint8(connackFlagsKnown)); err != nil {
		return err
	}

	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)

	if p.AssignedClientID, err = DecodeString(r); err != nil {
		return err
	}
	if p.SessionID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.KeepAlive, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.ReceiveMaximum, err = DecodeUint16(r); err != nil {
		return err
	}
	for i := range 3 {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	if err := p.Limits.decode(r); err != nil {
		return err
	}
	if p.Flags&ConnackFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

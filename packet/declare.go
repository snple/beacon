package packet

import "io"

// ChannelDecl 客户端声明的一项 channel 兴趣。
type ChannelDecl struct {
	Channel string
	// Flags 承载声明开关，见 ChannelFlag*。
	Flags ChannelFlags
}

// ChannelFlags 是单个 channel 声明的开关。
type ChannelFlags uint8

const (
	// ChannelFlagTargetOnly 表示该过滤器只接收定向投递：广播发布不会命中它。
	// 未置位时同时接收广播与定向投递（默认）。
	ChannelFlagTargetOnly ChannelFlags = 1 << 0
)

const channelFlagsKnown = ChannelFlagTargetOnly

// Unknown 返回未定义的标志位，供服务端拒绝无法理解的声明。
func (f ChannelFlags) Unknown() ChannelFlags { return f &^ channelFlagsKnown }

// HasFlag 报告某个开关是否置位。
func (d *ChannelDecl) HasFlag(flag ChannelFlags) bool { return d.Flags&flag != 0 }

// SetFlag 设置或清除某个开关。
func (d *ChannelDecl) SetFlag(flag ChannelFlags, on bool) {
	if on {
		d.Flags |= flag
	} else {
		d.Flags &^= flag
	}
}

// TargetOnly 报告该过滤器是否只接收定向投递。
func (d *ChannelDecl) TargetOnly() bool { return d.HasFlag(ChannelFlagTargetOnly) }

// SetTargetOnly 设置该过滤器是否只接收定向投递。
func (d *ChannelDecl) SetTargetOnly(on bool) { d.SetFlag(ChannelFlagTargetOnly, on) }

// DeclarePacket DECLARE：全量替换客户端的 channel 声明。
//
// 空列表表示取消全部兴趣；希望接收所有 channel 时必须显式声明 "**"。
// 每项声明可带 ChannelFlags：默认同时接收广播与定向投递，
// ChannelFlagTargetOnly 则只接收定向投递。
type DeclarePacket struct {
	RequestID uint32
	Channels  []ChannelDecl
}

// NewDeclarePacket 创建 DECLARE 包。
func NewDeclarePacket(requestID uint32, channels []ChannelDecl) *DeclarePacket {
	return &DeclarePacket{RequestID: requestID, Channels: channels}
}

func (p *DeclarePacket) Type() PacketType { return DECLARE }

func (p *DeclarePacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
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
		if err := WriteByte(w, byte(ch.Flags)); err != nil {
			return err
		}
	}
	return nil
}

func (p *DeclarePacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	if err := checkCount(r, count, 3); err != nil {
		return err
	}
	p.Channels = make([]ChannelDecl, 0, count)
	for i := 0; i < int(count); i++ {
		ch, err := DecodeString(r)
		if err != nil {
			return err
		}
		flags, err := ReadByte(r)
		if err != nil {
			return err
		}
		p.Channels = append(p.Channels, ChannelDecl{Channel: ch, Flags: ChannelFlags(flags)})
	}
	return nil
}

// Limits 服务端对单连接/单会话施加的上限。
type Limits struct {
	MaxPacketSize        uint32
	MaxWaitMs            uint32
	MaxChannelsPerClient uint16
}

// ChannelResult 单个 channel 声明的处理结果。
type ChannelResult struct {
	Channel string
	Reason  ReasonCode
}

// DeclareRespPacket DECLARE_RESP：声明结果与服务端信息。
//
// Results 只包含被拒绝的声明项；Accepted 是被接受的项数。
// 整批因数量超限被拒绝时 Results 为空且 Accepted=0。
type DeclareRespPacket struct {
	RequestID uint32
	Accepted  uint16
	Results   []ChannelResult
	Pending   [3]uint32
	Limits    Limits
}

// NewDeclareRespPacket 创建 DECLARE_RESP 包。
func NewDeclareRespPacket(requestID uint32) *DeclareRespPacket {
	return &DeclareRespPacket{RequestID: requestID}
}

func (p *DeclareRespPacket) Type() PacketType { return DECLARE_RESP }

func (p *DeclareRespPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.Accepted); err != nil {
		return err
	}
	if len(p.Results) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.Results))); err != nil {
		return err
	}
	for _, res := range p.Results {
		if err := EncodeString(w, res.Channel); err != nil {
			return err
		}
		if err := WriteByte(w, byte(res.Reason)); err != nil {
			return err
		}
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	if err := EncodeUint32(w, p.Limits.MaxPacketSize); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Limits.MaxWaitMs); err != nil {
		return err
	}
	return EncodeUint16(w, p.Limits.MaxChannelsPerClient)
}

func (p *DeclareRespPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Accepted, err = DecodeUint16(r); err != nil {
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
		p.Results = make([]ChannelResult, 0, count)
		for i := 0; i < int(count); i++ {
			ch, err := DecodeString(r)
			if err != nil {
				return err
			}
			reason, err := ReadByte(r)
			if err != nil {
				return err
			}
			p.Results = append(p.Results, ChannelResult{Channel: ch, Reason: ReasonCode(reason)})
		}
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	if p.Limits.MaxPacketSize, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Limits.MaxWaitMs, err = DecodeUint32(r); err != nil {
		return err
	}
	p.Limits.MaxChannelsPerClient, err = DecodeUint16(r)
	return err
}

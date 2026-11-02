package packet

import "io"

// DeclarePacket DECLARE：全量替换客户端的 channel 声明。
//
// 空列表表示取消全部兴趣；希望接收所有 channel 时必须显式声明 "**"。
type DeclarePacket struct {
	RequestID uint64
	Channels  []ChannelDecl
}

// NewDeclarePacket 创建 DECLARE 包。
func NewDeclarePacket(requestID uint64, channels []ChannelDecl) *DeclarePacket {
	return &DeclarePacket{RequestID: requestID, Channels: channels}
}

func (p *DeclarePacket) Type() PacketType { return DECLARE }

func (p *DeclarePacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
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

func (p *DeclarePacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
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
	return nil
}

// Limits 服务端对单连接/单会话施加的上限。
type Limits struct {
	MaxPacketSize        uint32
	MaxRecords           uint16
	MaxWaitMs            uint32
	MaxBatch             uint32
	MaxChannelsPerClient uint16
}

// ChannelResult 单个 channel 声明的处理结果。
type ChannelResult struct {
	Channel string
	Reason  ReasonCode
}

// DeclareRespPacket DECLARE_RESP：声明结果与服务端信息。
type DeclareRespPacket struct {
	RequestID uint64
	Results   []ChannelResult
	Pending   [3]uint32
	Delivered uint32
	Limits    Limits
}

// NewDeclareRespPacket 创建 DECLARE_RESP 包。
func NewDeclareRespPacket(requestID uint64) *DeclareRespPacket {
	return &DeclareRespPacket{RequestID: requestID}
}

func (p *DeclareRespPacket) Type() PacketType { return DECLARE_RESP }

func (p *DeclareRespPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
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
	if err := EncodeUint32(w, p.Delivered); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Limits.MaxPacketSize); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.Limits.MaxRecords); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Limits.MaxWaitMs); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Limits.MaxBatch); err != nil {
		return err
	}
	return EncodeUint16(w, p.Limits.MaxChannelsPerClient)
}

func (p *DeclareRespPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
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
	if p.Delivered, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Limits.MaxPacketSize, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Limits.MaxRecords, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.Limits.MaxWaitMs, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Limits.MaxBatch, err = DecodeUint32(r); err != nil {
		return err
	}
	p.Limits.MaxChannelsPerClient, err = DecodeUint16(r)
	return err
}

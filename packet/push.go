package packet

import "io"

// PushPacket PUSH：客户端发布一条记录。
type PushPacket struct {
	RequestID uint32
	// Flags bit0 WantAck，bit1 HasProperties；其余位保留，必须为 0。
	Flags      uint8
	Record     Record
	Properties map[string]string
}

// Push 标志位
const (
	PushFlagWantAck       uint8 = 1 << 0
	PushFlagHasProperties uint8 = 1 << 1
)

const pushFlagsKnown = PushFlagWantAck | PushFlagHasProperties

// NewPushPacket 创建 PUSH 包。
func NewPushPacket(requestID uint32, record Record) *PushPacket {
	return &PushPacket{RequestID: requestID, Record: record}
}

func (p *PushPacket) Type() PacketType { return PUSH }

func (p *PushPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= PushFlagHasProperties
	} else {
		flags &^= PushFlagHasProperties
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}
	if err := encodeRecord(w, &p.Record); err != nil {
		return err
	}
	if flags&PushFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
}

func (p *PushPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	if err := checkUnknownFlags(p.Flags, pushFlagsKnown); err != nil {
		return err
	}
	rec, err := decodeRecord(r)
	if err != nil {
		return err
	}
	p.Record = *rec
	if p.Flags&PushFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

// PushResult 单条记录的处理结果。
type PushResult struct {
	Reason  ReasonCode
	ID      PacketID
	Matched uint32 // 入队成功的客户端数
	// Dropped 是未入队的客户端数：目标离线时的不可靠消息丢弃、入队导致旧消息被挤出、
	// 或新消息本身因优先级/可靠性被丢弃。
	Dropped uint32
}

// PushOkPacket PUSH_OK：发布结果。
type PushOkPacket struct {
	RequestID uint32
	// Flags bit0 = HasProperties；其余保留。
	Flags uint8
	PushResult
	Properties map[string]string
}

// PUSH_OK 标志位。
const (
	PushOkFlagHasProperties uint8 = 1 << 0
)

const pushOkFlagsKnown = PushOkFlagHasProperties

// NewPushOkPacket 创建 PUSH_OK 包。
func NewPushOkPacket(requestID uint32) *PushOkPacket {
	return &PushOkPacket{RequestID: requestID}
}

func (p *PushOkPacket) Type() PacketType { return PUSH_OK }

// Size 返回 PUSH_OK 完整编码后的大小（含固定头与可选 Properties）。
func (p *PushOkPacket) Size() int {
	size := 5 + 4 + 1 + 1 + PacketIDSize + 4 + 4
	if len(p.Properties) > 0 {
		size += StringMapSize(p.Properties)
	}
	return size
}

func (p *PushOkPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= PushOkFlagHasProperties
	} else {
		flags &^= PushOkFlagHasProperties
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.Reason)); err != nil {
		return err
	}
	if err := EncodePacketID(w, p.ID); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Matched); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.Dropped); err != nil {
		return err
	}
	if flags&PushOkFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
}

func (p *PushOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	if err := checkUnknownFlags(p.Flags, pushOkFlagsKnown); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Reason = ReasonCode(reason)
	if p.ID, err = DecodePacketID(r); err != nil {
		return err
	}
	if p.Matched, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Dropped, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags&PushOkFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

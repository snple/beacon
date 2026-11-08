package packet

import "io"

// PushPacket PUSH：客户端发布一条记录。
type PushPacket struct {
	RequestID uint32
	// Flags bit0 WantAck
	Flags  uint8
	Record Record
}

// Push 标志位
const (
	PushFlagWantAck uint8 = 1 << 0
)

// NewPushPacket 创建 PUSH 包。
func NewPushPacket(requestID uint32, record Record) *PushPacket {
	return &PushPacket{RequestID: requestID, Record: record}
}

func (p *PushPacket) Type() PacketType { return PUSH }

func (p *PushPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	if err := WriteByte(w, p.Flags); err != nil {
		return err
	}
	return encodeRecord(w, &p.Record)
}

func (p *PushPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	rec, err := decodeRecord(r)
	if err != nil {
		return err
	}
	p.Record = *rec
	return nil
}

// PushResult 单条记录的处理结果。
type PushResult struct {
	Reason  ReasonCode
	ID      PacketID
	Matched uint32 // 入队成功的客户端数
	// Dropped 是未入队的客户端数：目标离线时的 Low 丢弃、入队导致旧消息被挤出、
	// 或新消息本身因优先级被丢弃。
	Dropped uint32
}

// PushOkPacket PUSH_OK：发布结果。
type PushOkPacket struct {
	RequestID uint32
	PushResult
}

// NewPushOkPacket 创建 PUSH_OK 包。
func NewPushOkPacket(requestID uint32) *PushOkPacket {
	return &PushOkPacket{RequestID: requestID}
}

func (p *PushOkPacket) Type() PacketType { return PUSH_OK }

func (p *PushOkPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
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
	return EncodeUint32(w, p.Dropped)
}

func (p *PushOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
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
	p.Dropped, err = DecodeUint32(r)
	return err
}

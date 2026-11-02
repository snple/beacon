package packet

import "io"

// PushPacket PUSH：客户端发布一批记录。
type PushPacket struct {
	RequestID uint64
	// Flags bit0 WantAck
	Flags   uint8
	Records []Record
}

// Push 标志位
const (
	PushFlagWantAck uint8 = 1 << 0
)

// NewPushPacket 创建 PUSH 包。
func NewPushPacket(requestID uint64, records ...Record) *PushPacket {
	return &PushPacket{RequestID: requestID, Records: records}
}

func (p *PushPacket) Type() PacketType { return PUSH }

func (p *PushPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := WriteByte(w, p.Flags); err != nil {
		return err
	}
	if len(p.Records) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.Records))); err != nil {
		return err
	}
	for i := range p.Records {
		if err := encodeRecord(w, &p.Records[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *PushPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	// 最小记录长度：Flags(1)+ID(12)+Channel(2)+Target(2)+Source(2)+Headers(2)+Payload(4)
	if err := checkCount(r, count, 25); err != nil {
		return err
	}
	p.Records = make([]Record, 0, count)
	for i := 0; i < int(count); i++ {
		rec, err := decodeRecord(r)
		if err != nil {
			return err
		}
		p.Records = append(p.Records, *rec)
	}
	return nil
}

// PushResult 单条记录的处理结果。
type PushResult struct {
	Reason  ReasonCode
	ID      PacketID
	Matched uint32 // 入队成功的客户端数
	Dropped uint32 // 因入队导致旧消息被挤出的客户端数 / 新消息被丢弃的客户端数
}

// PushOkPacket PUSH_OK：发布结果。
type PushOkPacket struct {
	RequestID uint64
	Results   []PushResult
}

// NewPushOkPacket 创建 PUSH_OK 包。
func NewPushOkPacket(requestID uint64) *PushOkPacket {
	return &PushOkPacket{RequestID: requestID}
}

func (p *PushOkPacket) Type() PacketType { return PUSH_OK }

func (p *PushOkPacket) encode(w io.Writer) error {
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
		if err := WriteByte(w, byte(res.Reason)); err != nil {
			return err
		}
		if err := EncodePacketID(w, res.ID); err != nil {
			return err
		}
		if err := EncodeUint32(w, res.Matched); err != nil {
			return err
		}
		if err := EncodeUint32(w, res.Dropped); err != nil {
			return err
		}
	}
	return nil
}

func (p *PushOkPacket) decode(r io.Reader, _ FixedHeader) error {
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
	p.Results = make([]PushResult, 0, count)
	for i := 0; i < int(count); i++ {
		reason, err := ReadByte(r)
		if err != nil {
			return err
		}
		id, err := DecodePacketID(r)
		if err != nil {
			return err
		}
		matched, err := DecodeUint32(r)
		if err != nil {
			return err
		}
		dropped, err := DecodeUint32(r)
		if err != nil {
			return err
		}
		p.Results = append(p.Results, PushResult{
			Reason: ReasonCode(reason), ID: id, Matched: matched, Dropped: dropped,
		})
	}
	return nil
}

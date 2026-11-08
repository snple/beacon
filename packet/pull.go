package packet

import "io"

// PullPacket PULL：请求拉取一条记录。
type PullPacket struct {
	RequestID uint32
	WaitMs    uint32
}

// NewPullPacket 创建 PULL 包。
func NewPullPacket(requestID uint32, waitMs uint32) *PullPacket {
	return &PullPacket{RequestID: requestID, WaitMs: waitMs}
}

func (p *PullPacket) Type() PacketType { return PULL }

func (p *PullPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	return EncodeUint32(w, p.WaitMs)
}

func (p *PullPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	p.WaitMs, err = DecodeUint32(r)
	return err
}

// PullOkPacket PULL_OK：拉取结果（至多一条记录）。
type PullOkPacket struct {
	RequestID uint32
	Reason    ReasonCode
	Flags     uint8 // bit0 = HasRecord
	Pending   [3]uint32
	Record    Record // 仅 HasRecord=1 时有效
}

// PULL_OK 标志位
const (
	PullFlagHasRecord uint8 = 1 << 0
)

// HasRecord 报告响应是否携带记录。
func (p *PullOkPacket) HasRecord() bool { return p.Flags&PullFlagHasRecord != 0 }

// SetRecord 设置响应携带的记录。
func (p *PullOkPacket) SetRecord(rec Record) {
	p.Record = rec
	p.Flags |= PullFlagHasRecord
}

// NewPullOkPacket 创建 PULL_OK 包。
func NewPullOkPacket(requestID uint32) *PullOkPacket {
	return &PullOkPacket{RequestID: requestID, Reason: ReasonSuccess}
}

func (p *PullOkPacket) Type() PacketType { return PULL_OK }

func (p *PullOkPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.Reason)); err != nil {
		return err
	}
	if err := WriteByte(w, p.Flags); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	if !p.HasRecord() {
		return nil
	}
	return encodeRecord(w, &p.Record)
}

func (p *PullOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Reason = ReasonCode(reason)

	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	if !p.HasRecord() {
		return nil
	}
	rec, err := decodeRecord(r)
	if err != nil {
		return err
	}
	p.Record = *rec
	return nil
}

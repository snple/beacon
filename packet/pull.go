package packet

import "io"

// PullPacket PULL：请求拉取数据。
type PullPacket struct {
	RequestID  uint64
	MaxRecords uint16
	MaxBytes   uint32
	WaitMs     uint32
}

// NewPullPacket 创建 PULL 包。
func NewPullPacket(requestID uint64, maxRecords uint16, maxBytes, waitMs uint32) *PullPacket {
	return &PullPacket{
		RequestID:  requestID,
		MaxRecords: maxRecords,
		MaxBytes:   maxBytes,
		WaitMs:     waitMs,
	}
}

func (p *PullPacket) Type() PacketType { return PULL }

func (p *PullPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.MaxRecords); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.MaxBytes); err != nil {
		return err
	}
	return EncodeUint32(w, p.WaitMs)
}

func (p *PullPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.MaxRecords, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.MaxBytes, err = DecodeUint32(r); err != nil {
		return err
	}
	p.WaitMs, err = DecodeUint32(r)
	return err
}

// PullOkPacket PULL_OK：拉取结果。
type PullOkPacket struct {
	RequestID         uint64
	Reason            ReasonCode
	More              bool
	RetryAfterMs      uint32
	Pending           [3]uint32
	Delivered         uint32
	OldestUncommitted PacketID
	Records           []Record
}

// NewPullOkPacket 创建 PULL_OK 包。
func NewPullOkPacket(requestID uint64) *PullOkPacket {
	return &PullOkPacket{RequestID: requestID, Reason: ReasonSuccess}
}

func (p *PullOkPacket) Type() PacketType { return PULL_OK }

func (p *PullOkPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.Reason)); err != nil {
		return err
	}
	var flags uint8
	if p.More {
		flags |= 1
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.RetryAfterMs); err != nil {
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
	if err := EncodePacketID(w, p.OldestUncommitted); err != nil {
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

func (p *PullOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Reason = ReasonCode(reason)

	flags, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.More = flags&1 != 0

	if p.RetryAfterMs, err = DecodeUint32(r); err != nil {
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
	if p.OldestUncommitted, err = DecodePacketID(r); err != nil {
		return err
	}

	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
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

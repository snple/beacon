package packet

import "io"

// CommitPacket COMMIT：确认已处理的消息 ID（按 ID 列表，支持乱序确认）。
type CommitPacket struct {
	RequestID uint64
	IDs       []PacketID
}

// NewCommitPacket 创建 COMMIT 包。
func NewCommitPacket(requestID uint64, ids ...PacketID) *CommitPacket {
	return &CommitPacket{RequestID: requestID, IDs: ids}
}

func (p *CommitPacket) Type() PacketType { return COMMIT }

func (p *CommitPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if len(p.IDs) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.IDs))); err != nil {
		return err
	}
	for _, id := range p.IDs {
		if err := EncodePacketID(w, id); err != nil {
			return err
		}
	}
	return nil
}

func (p *CommitPacket) decode(r io.Reader, _ FixedHeader) error {
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
	p.IDs = make([]PacketID, 0, count)
	for i := 0; i < int(count); i++ {
		id, err := DecodePacketID(r)
		if err != nil {
			return err
		}
		p.IDs = append(p.IDs, id)
	}
	return nil
}

// CommitOkPacket COMMIT_OK：确认结果。
type CommitOkPacket struct {
	RequestID uint64
	Committed uint16
	Pending   [3]uint32
	Delivered uint32
}

// NewCommitOkPacket 创建 COMMIT_OK 包。
func NewCommitOkPacket(requestID uint64) *CommitOkPacket {
	return &CommitOkPacket{RequestID: requestID}
}

func (p *CommitOkPacket) Type() PacketType { return COMMIT_OK }

func (p *CommitOkPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.Committed); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	return EncodeUint32(w, p.Delivered)
}

func (p *CommitOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.Committed, err = DecodeUint16(r); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	p.Delivered, err = DecodeUint32(r)
	return err
}

package packet

import "io"

// CommitPacket COMMIT：确认一条已处理的消息。
type CommitPacket struct {
	RequestID uint32
	ID        PacketID
}

// NewCommitPacket 创建 COMMIT 包。
func NewCommitPacket(requestID uint32, id PacketID) *CommitPacket {
	return &CommitPacket{RequestID: requestID, ID: id}
}

func (p *CommitPacket) Type() PacketType { return COMMIT }

func (p *CommitPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	return EncodePacketID(w, p.ID)
}

func (p *CommitPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	p.ID, err = DecodePacketID(r)
	return err
}

// CommitOkPacket COMMIT_OK：确认结果。
type CommitOkPacket struct {
	RequestID uint32
	Pending   [3]uint32
}

// NewCommitOkPacket 创建 COMMIT_OK 包。
func NewCommitOkPacket(requestID uint32) *CommitOkPacket {
	return &CommitOkPacket{RequestID: requestID}
}

func (p *CommitOkPacket) Type() PacketType { return COMMIT_OK }

func (p *CommitOkPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	return nil
}

func (p *CommitOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	return nil
}

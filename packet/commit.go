package packet

import (
	"fmt"
	"io"
)

// CommitAction 是 COMMIT 中单个 ID 的处理动作。
type CommitAction uint8

const (
	// CommitActionCommit 确认消息已处理。
	CommitActionCommit CommitAction = 0
	// CommitActionNack 拒绝消息（不再重发，交由服务端丢弃/死信）。
	CommitActionNack CommitAction = 1
)

// CommitEntry 是 COMMIT 中的一条记录。
type CommitEntry struct {
	ID     PacketID
	Action CommitAction
	Reason ReasonCode // NACK 时可选的原因码
}

// CommitPacket COMMIT：批量确认或拒绝已投递记录。
type CommitPacket struct {
	RequestID uint32
	Entries   []CommitEntry
}

// NewCommitPacket 创建只确认一条记录的 COMMIT 包。
func NewCommitPacket(requestID uint32, id PacketID) *CommitPacket {
	return &CommitPacket{RequestID: requestID, Entries: []CommitEntry{{ID: id, Action: CommitActionCommit}}}
}

// NewCommitBatchPacket 创建批量 COMMIT 包。
func NewCommitBatchPacket(requestID uint32, entries []CommitEntry) *CommitPacket {
	return &CommitPacket{RequestID: requestID, Entries: entries}
}

func (p *CommitPacket) Type() PacketType { return COMMIT }

func (p *CommitPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	if len(p.Entries) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.Entries))); err != nil {
		return err
	}
	for _, e := range p.Entries {
		if err := EncodePacketID(w, e.ID); err != nil {
			return err
		}
		if err := WriteByte(w, byte(e.Action)); err != nil {
			return err
		}
		if err := WriteByte(w, byte(e.Reason)); err != nil {
			return err
		}
	}
	return nil
}

func (p *CommitPacket) decode(r io.Reader, _ FixedHeader) error {
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
	if err := checkCount(r, count, 14); err != nil {
		return err
	}
	p.Entries = make([]CommitEntry, 0, count)
	for i := 0; i < int(count); i++ {
		var e CommitEntry
		if e.ID, err = DecodePacketID(r); err != nil {
			return err
		}
		action, err := ReadByte(r)
		if err != nil {
			return err
		}
		reason, err := ReadByte(r)
		if err != nil {
			return err
		}
		e.Action = CommitAction(action)
		if e.Action != CommitActionCommit && e.Action != CommitActionNack {
			return fmt.Errorf("%w: unknown commit action %d", ErrMalformedPacket, e.Action)
		}
		e.Reason = ReasonCode(reason)
		p.Entries = append(p.Entries, e)
	}
	return nil
}

// CommitOkPacket COMMIT_OK：确认结果。
type CommitOkPacket struct {
	RequestID uint32
	Committed uint16
	Nacked    uint16
	Unknown   uint16 // 不在 delivered 中的 ID（幂等空操作）
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
	if err := EncodeUint16(w, p.Committed); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.Nacked); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.Unknown); err != nil {
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
	if p.Committed, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.Nacked, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.Unknown, err = DecodeUint16(r); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	return nil
}

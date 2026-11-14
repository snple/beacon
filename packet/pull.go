package packet

import "io"

// PullPacket PULL：请求拉取一批记录。
type PullPacket struct {
	RequestID uint32
	// Flags bit0 = HasProperties；其余保留。
	Flags  uint8
	WaitMs uint32 // 长轮询时长；0 = 立即返回
	// MaxCount 是本次最多返回的记录数；0 与 1 等价。
	MaxCount uint16
	// MaxBytes 是本次响应中记录编码大小的总和上限；0 = 不额外限制。
	MaxBytes   uint32
	Properties map[string]string
}

// PULL 标志位。
const (
	PullFlagHasProperties uint8 = 1 << 0
)

const pullFlagsKnown = PullFlagHasProperties

// NewPullPacket 创建 PULL 包。
func NewPullPacket(requestID uint32, waitMs uint32) *PullPacket {
	return &PullPacket{RequestID: requestID, WaitMs: waitMs, MaxCount: 1}
}

func (p *PullPacket) Type() PacketType { return PULL }

func (p *PullPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= PullFlagHasProperties
	} else {
		flags &^= PullFlagHasProperties
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.WaitMs); err != nil {
		return err
	}
	if err := EncodeUint16(w, p.MaxCount); err != nil {
		return err
	}
	if err := EncodeUint32(w, p.MaxBytes); err != nil {
		return err
	}
	if flags&PullFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
}

func (p *PullPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	if err := checkUnknownFlags(p.Flags, pullFlagsKnown); err != nil {
		return err
	}
	if p.WaitMs, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.MaxCount, err = DecodeUint16(r); err != nil {
		return err
	}
	if p.MaxBytes, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags&PullFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

// PullOkPacket PULL_OK：拉取结果（0..N 条记录）。
type PullOkPacket struct {
	RequestID uint32
	// Flags bit0 = HasProperties；其余保留。
	Flags      uint8
	Reason     ReasonCode
	Pending    [3]uint32
	Records    []Record
	Properties map[string]string
}

// PULL_OK 标志位。
const (
	PullOkFlagHasProperties uint8 = 1 << 0
)

const pullOkFlagsKnown = PullOkFlagHasProperties

// NewPullOkPacket 创建 PULL_OK 包。
func NewPullOkPacket(requestID uint32) *PullOkPacket {
	return &PullOkPacket{RequestID: requestID, Reason: ReasonSuccess}
}

// HasRecord 报告响应是否携带至少一条记录。
func (p *PullOkPacket) HasRecord() bool { return len(p.Records) > 0 }

// RecordCount 返回记录条数。
func (p *PullOkPacket) RecordCount() int { return len(p.Records) }

// AddRecord 追加一条记录。
func (p *PullOkPacket) AddRecord(rec Record) { p.Records = append(p.Records, rec) }

// SetRecord 用单条记录替换结果（兼容单条调用）。
func (p *PullOkPacket) SetRecord(rec Record) {
	p.Records = []Record{rec}
}

func (p *PullOkPacket) Type() PacketType { return PULL_OK }

func (p *PullOkPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.RequestID); err != nil {
		return err
	}
	flags := p.Flags
	if len(p.Properties) > 0 {
		flags |= PullOkFlagHasProperties
	} else {
		flags &^= PullOkFlagHasProperties
	}
	if err := WriteByte(w, flags); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.Reason)); err != nil {
		return err
	}
	if len(p.Records) > MaxStringLength {
		return ErrMalformedPacket
	}
	if err := EncodeUint16(w, uint16(len(p.Records))); err != nil {
		return err
	}
	for _, v := range p.Pending {
		if err := EncodeUint32(w, v); err != nil {
			return err
		}
	}
	for i := range p.Records {
		if err := encodeRecord(w, &p.Records[i]); err != nil {
			return err
		}
	}
	if flags&PullOkFlagHasProperties != 0 {
		return EncodeStringMap(w, p.Properties)
	}
	return nil
}

func (p *PullOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint32(r); err != nil {
		return err
	}
	if p.Flags, err = ReadByte(r); err != nil {
		return err
	}
	if err := checkUnknownFlags(p.Flags, pullOkFlagsKnown); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Reason = ReasonCode(reason)
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}
	for i := range 3 {
		if p.Pending[i], err = DecodeUint32(r); err != nil {
			return err
		}
	}
	if count == 0 {
		if p.Flags&PullOkFlagHasProperties != 0 {
			p.Properties, err = DecodeStringMap(r)
		}
		return err
	}
	// 每条记录至少 25 字节（Flags+ID+三个空字符串长度前缀+headers 计数+payload 长度）。
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
	if p.Flags&PullOkFlagHasProperties != 0 {
		p.Properties, err = DecodeStringMap(r)
	}
	return err
}

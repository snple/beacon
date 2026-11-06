package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Record 是 PUSH 与 PULL_OK 共用的消息记录。
//
// 一条消息由 broker 生成唯一 ID 并编码一次，之后被所有命中的客户端队列共享，
// 因此线上编码与内存中的编码可以复用同一份字节。
type Record struct {
	// Flags 位布局：bit0 Dup，bit1-2 Priority，bit3 HasHeaders，bit4 HasPriority
	Flags uint8

	ID      PacketID
	Channel string
	Target  string // 定向投递目标 clientID；空表示普通发布
	Source  string
	Headers map[string]string

	Payload []byte
}

// Record 标志位
const (
	recordFlagDup         uint8 = 1 << 0
	recordFlagPriorityLo  uint8 = 1 << 1
	recordFlagPriorityHi  uint8 = 1 << 2
	recordFlagHasHeaders  uint8 = 1 << 3
	recordFlagHasPriority uint8 = 1 << 4
)

// SetDup 设置重复投递标志。
func (r *Record) SetDup(dup bool) {
	if dup {
		r.Flags |= recordFlagDup
	} else {
		r.Flags &^= recordFlagDup
	}
}

// Dup 报告该记录是否是重复投递。
func (r *Record) Dup() bool { return r.Flags&recordFlagDup != 0 }

// SetPriority 设置优先级，并标记为显式指定。
func (r *Record) SetPriority(p Priority) {
	r.Flags &^= recordFlagPriorityLo | recordFlagPriorityHi
	r.Flags |= (uint8(p) << 1) & (recordFlagPriorityLo | recordFlagPriorityHi)
	r.Flags |= recordFlagHasPriority
}

// HasPriority 报告发送方是否显式指定了优先级。
func (r *Record) HasPriority() bool { return r.Flags&recordFlagHasPriority != 0 }

// Priority 返回优先级；未显式指定时默认 Medium。
func (r *Record) Priority() Priority {
	if r.Flags&recordFlagHasPriority == 0 {
		return PriorityMedium
	}
	return Priority((r.Flags >> 1) & 0x03)
}

// HasHeaders 报告是否携带 headers。
func (r *Record) HasHeaders() bool { return r.Flags&recordFlagHasHeaders != 0 }

// Timestamp 返回 ID 中内嵌的毫秒时间戳。
func (r *Record) Timestamp() int64 {
	return int64(binary.BigEndian.Uint16(r.ID[0:2]))<<32 |
		int64(binary.BigEndian.Uint16(r.ID[2:4]))<<16 |
		int64(binary.BigEndian.Uint16(r.ID[4:6]))
}

// Clone 深拷贝记录（payload/headers 独立）。
func (r *Record) Clone() *Record {
	if r == nil {
		return nil
	}
	c := *r
	c.Payload = append([]byte(nil), r.Payload...)
	if r.Headers != nil {
		c.Headers = make(map[string]string, len(r.Headers))
		for k, v := range r.Headers {
			c.Headers[k] = v
		}
	}
	return &c
}

// encodeRecord 把记录写入 w。
func encodeRecord(w io.Writer, r *Record) error {
	flags := r.Flags
	if len(r.Headers) > 0 {
		flags |= recordFlagHasHeaders
	} else {
		flags &^= recordFlagHasHeaders
	}
	if _, err := w.Write([]byte{flags}); err != nil {
		return err
	}
	if err := EncodePacketID(w, r.ID); err != nil {
		return err
	}
	if err := EncodeString(w, r.Channel); err != nil {
		return err
	}
	if err := EncodeString(w, r.Target); err != nil {
		return err
	}
	if err := EncodeString(w, r.Source); err != nil {
		return err
	}

	if flags&recordFlagHasHeaders != 0 {
		if err := EncodeStringMap(w, r.Headers); err != nil {
			return err
		}
	} else if err := EncodeUint16(w, 0); err != nil {
		return err
	}

	return EncodeBytes32(w, r.Payload)
}

// decodeRecord 从 r 读取一条记录。
func decodeRecord(r io.Reader) (*Record, error) {
	flags, err := ReadByte(r)
	if err != nil {
		return nil, err
	}

	rec := &Record{Flags: flags}

	if rec.ID, err = DecodePacketID(r); err != nil {
		return nil, err
	}
	if rec.Channel, err = DecodeString(r); err != nil {
		return nil, err
	}
	if rec.Target, err = DecodeString(r); err != nil {
		return nil, err
	}
	if rec.Source, err = DecodeString(r); err != nil {
		return nil, err
	}

	headers, err := DecodeStringMap(r)
	if err != nil {
		return nil, err
	}
	// HasHeaders 位必须与实际 headers 一致，否则不同实现可能对同一字节流
	// 产生不同解释（例如把 map 内容当作 payload 前缀）。
	if flags&recordFlagHasHeaders != 0 != (len(headers) > 0) {
		return nil, fmt.Errorf("%w: HasHeaders flag does not match headers", ErrMalformedPacket)
	}
	rec.Headers = headers

	if rec.Payload, err = DecodeBytes32(r); err != nil {
		return nil, err
	}
	if rec.HasPriority() && !rec.Priority().IsValid() {
		return nil, fmt.Errorf("%w: invalid priority %d", ErrMalformedPacket, rec.Priority())
	}

	return rec, nil
}

// RecordSize 返回记录编码后的字节数。
func RecordSize(r *Record) int {
	return 1 + PacketIDSize +
		StringSize(r.Channel) + StringSize(r.Target) + StringSize(r.Source) +
		StringMapSize(r.Headers) + Bytes32Size(r.Payload)
}

// EncodeRecordBody 把记录编码为独立字节切片（broker 侧一次编码、多客户端共享）。
func EncodeRecordBody(r *Record) ([]byte, error) {
	var buf bytes.Buffer
	buf.Grow(RecordSize(r))
	if err := encodeRecord(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeRecord 从字节切片解析出一条完整记录，且要求恰好消费完。
func DecodeRecord(data []byte) (*Record, error) {
	r := bytes.NewReader(data)
	rec, err := decodeRecord(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("%w: trailing bytes in record", ErrMalformedPacket)
	}
	return rec, nil
}

// EncodeBytes32 编码 uint32 长度前缀的字节串。
func EncodeBytes32(w io.Writer, data []byte) error {
	if err := EncodeUint32(w, uint32(len(data))); err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	_, err := w.Write(data)
	return err
}

// DecodeBytes32 解码 uint32 长度前缀的字节串。
//
// 长度字段来自对端，必须先确认它不超过缓冲区剩余字节数再分配，
// 否则声明 4GB 长度的小包会直接导致 OOM。
func DecodeBytes32(r io.Reader) ([]byte, error) {
	var length uint32
	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length == 0 {
		return nil, nil
	}
	if avail, ok := remainingBytes(r); ok && int64(length) > int64(avail) {
		return nil, fmt.Errorf("%w: declared length %d exceeds remaining %d bytes",
			ErrMalformedPacket, length, avail)
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

// remainingBytes 返回 reader 中尚未读取的字节数（仅对 *bytes.Reader 有效）。
func remainingBytes(r io.Reader) (int, bool) {
	if br, ok := r.(*bytes.Reader); ok {
		return br.Len(), true
	}
	return 0, false
}

// checkCount 校验「声明了 count 个元素」与实际剩余字节是否匹配，
// 避免用极小的包触发大切片预分配。
func checkCount(r io.Reader, count uint16, minElemSize int) error {
	avail, ok := remainingBytes(r)
	if !ok || count == 0 || minElemSize <= 0 {
		return nil
	}
	if int(count)*minElemSize > avail {
		return fmt.Errorf("%w: declared %d elements but only %d bytes remain",
			ErrMalformedPacket, count, avail)
	}
	return nil
}

// Bytes32Size 返回 uint32 长度前缀字节串的编码大小。
func Bytes32Size(data []byte) int { return 4 + len(data) }

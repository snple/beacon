package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
)

// Record 是 PUSH 与 PULL_OK 共用的消息记录。
//
// 一条消息由 broker 生成唯一 ID 并编码一次，之后被所有命中的客户端队列共享，
// 因此线上编码与内存中的编码可以复用同一份字节。
type Record struct {
	// Flags 位布局：bit0 Dup，bit1-2 Priority（00=Medium，01=Low，10=High），
	// bit3 HasHeaders，bit4 保留，bit5 Reliable，bit6 HasExpiry，
	// bit7 HasPublishedAt。
	Flags uint8

	ID      PacketID
	Channel string
	Target  string // 定向投递目标 clientID；空表示普通发布
	Source  string
	Headers map[string]string

	Payload []byte

	// ExpiresAt 是消息过期时间（Unix 毫秒）；0 表示永不过期。
	ExpiresAt int64
	// PublishedAt 是发布者提供的时间戳（Unix 毫秒）；0 表示未提供。
	PublishedAt int64
}

// Record 标志位
const (
	recordFlagDup            uint8 = 1 << 0
	recordFlagPriorityMask   uint8 = 0x06 // bit1-2
	recordFlagHasHeaders     uint8 = 1 << 3
	recordFlagReliable       uint8 = 1 << 5
	recordFlagHasExpiry      uint8 = 1 << 6
	recordFlagHasPublishedAt uint8 = 1 << 7
	recordFlagsKnown               = recordFlagDup | recordFlagPriorityMask |
		recordFlagHasHeaders | recordFlagReliable | recordFlagHasExpiry | recordFlagHasPublishedAt
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

// wirePriority 返回优先级在线路上的编码：00=Medium，01=Low，10=High。
//
// 除 Low/High 外的值都按默认的 Medium 编码。
func wirePriority(p Priority) uint8 {
	switch p {
	case PriorityLow:
		return 1
	case PriorityHigh:
		return 2
	default:
		return 0
	}
}

// priorityFromWire 解码线路上的优先级；11 返回无效值。
func priorityFromWire(v uint8) Priority {
	switch v {
	case 0:
		return PriorityMedium
	case 1:
		return PriorityLow
	case 2:
		return PriorityHigh
	default:
		return Priority(0xFF)
	}
}

// SetPriority 设置优先级。Medium 的线路编码是 00，因此零值 Flags 就是默认优先级。
func (r *Record) SetPriority(p Priority) {
	r.Flags &^= recordFlagPriorityMask
	r.Flags |= wirePriority(p) << 1
}

// Priority 返回优先级；默认 Medium。仅在解码损坏数据时可能返回无效值。
func (r *Record) Priority() Priority {
	return priorityFromWire((r.Flags & recordFlagPriorityMask) >> 1)
}

// HasHeaders 报告是否携带 headers。
func (r *Record) HasHeaders() bool { return r.Flags&recordFlagHasHeaders != 0 }

// SetReliable 设置该记录是否需要 COMMIT 确认。
//
// 可靠性默认关闭：未调用 SetReliable(true) 的记录一律视为不可靠。
func (r *Record) SetReliable(reliable bool) {
	if reliable {
		r.Flags |= recordFlagReliable
	} else {
		r.Flags &^= recordFlagReliable
	}
}

// Reliable 报告该记录是否需要 COMMIT 确认；默认 false。
func (r *Record) Reliable() bool { return r.Flags&recordFlagReliable != 0 }

// HasExpiry 报告记录是否携带过期时间。
func (r *Record) HasExpiry() bool { return r.Flags&recordFlagHasExpiry != 0 }

// SetExpiresAt 设置过期时间（Unix 毫秒）；<= 0 表示永不过期。
func (r *Record) SetExpiresAt(ms int64) {
	if ms > 0 {
		r.ExpiresAt = ms
		r.Flags |= recordFlagHasExpiry
		return
	}
	r.ExpiresAt = 0
	r.Flags &^= recordFlagHasExpiry
}

// Expired 报告记录是否已在 now（Unix 毫秒）之前过期。
func (r *Record) Expired(now int64) bool {
	return r.ExpiresAt > 0 && r.ExpiresAt <= now
}

// HasPublishedAt 报告记录是否携带发布者时间戳。
func (r *Record) HasPublishedAt() bool { return r.Flags&recordFlagHasPublishedAt != 0 }

// SetPublishedAt 设置发布者时间戳（Unix 毫秒）；<= 0 表示不提供。
func (r *Record) SetPublishedAt(ms int64) {
	if ms > 0 {
		r.PublishedAt = ms
		r.Flags |= recordFlagHasPublishedAt
		return
	}
	r.PublishedAt = 0
	r.Flags &^= recordFlagHasPublishedAt
}

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
		maps.Copy(c.Headers, r.Headers)
	}
	return &c
}

// encodeRecord 把记录写入 w。
func encodeRecord(w io.Writer, r *Record) error {
	if r.Flags&^recordFlagsKnown != 0 {
		return fmt.Errorf("%w: unknown record flags 0x%02x", ErrMalformedPacket, r.Flags&^recordFlagsKnown)
	}
	flags := r.Flags
	if len(r.Headers) > 0 {
		flags |= recordFlagHasHeaders
	} else {
		flags &^= recordFlagHasHeaders
	}
	payload := r.Payload
	if r.ExpiresAt > 0 {
		flags |= recordFlagHasExpiry
	} else {
		flags &^= recordFlagHasExpiry
	}
	if r.PublishedAt > 0 {
		flags |= recordFlagHasPublishedAt
	} else {
		flags &^= recordFlagHasPublishedAt
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

	if err := EncodeBytes32(w, payload); err != nil {
		return err
	}
	if flags&recordFlagHasExpiry != 0 {
		if err := EncodeUint64(w, uint64(r.ExpiresAt)); err != nil {
			return err
		}
	}
	if flags&recordFlagHasPublishedAt != 0 {
		if err := EncodeUint64(w, uint64(r.PublishedAt)); err != nil {
			return err
		}
	}
	return nil
}

// decodeRecord 从 r 读取一条记录。
func decodeRecord(r io.Reader) (*Record, error) {
	flags, err := ReadByte(r)
	if err != nil {
		return nil, err
	}

	if flags&^recordFlagsKnown != 0 {
		return nil, fmt.Errorf("%w: unknown record flags 0x%02x", ErrMalformedPacket, flags&^recordFlagsKnown)
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

	payload, err := DecodeBytes32(r)
	if err != nil {
		return nil, err
	}
	if flags&recordFlagHasExpiry != 0 {
		v, err := DecodeUint64(r)
		if err != nil {
			return nil, err
		}
		rec.ExpiresAt = int64(v)
	}
	if flags&recordFlagHasPublishedAt != 0 {
		v, err := DecodeUint64(r)
		if err != nil {
			return nil, err
		}
		rec.PublishedAt = int64(v)
	}
	rec.Payload = payload
	rec.Flags = flags
	if !rec.Priority().IsValid() {
		return nil, fmt.Errorf("%w: invalid priority", ErrMalformedPacket)
	}

	return rec, nil
}

// RecordSize 返回记录编码后的字节数。
func RecordSize(r *Record) int {
	payload := r.Payload
	size := 1 + PacketIDSize +
		StringSize(r.Channel) + StringSize(r.Target) + StringSize(r.Source) +
		StringMapSize(r.Headers) + Bytes32Size(payload)
	if r.ExpiresAt > 0 {
		size += 8
	}
	if r.PublishedAt > 0 {
		size += 8
	}
	return size
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

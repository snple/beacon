package packet

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

// PingPacket PING 数据包（增强版，携带时间戳用于 RTT 计算）
type PingPacket struct {
	Seq       uint32 // 序号，用于关联 PING/PONG
	Timestamp uint64 // 发送时间戳（Unix 纳秒），用于计算 RTT
}

// NewPingPacket 创建新的 PING 包
func NewPingPacket(seq uint32, timestamp uint64) *PingPacket {
	return &PingPacket{Seq: seq, Timestamp: timestamp}
}

func (p *PingPacket) Type() PacketType {
	return PING
}

func (p *PingPacket) encode(w io.Writer) error {
	// 格式：Seq(4) + Timestamp(8)
	if err := EncodeUint32(w, p.Seq); err != nil {
		return err
	}

	return EncodeUint64(w, p.Timestamp)
}

func (p *PingPacket) decode(r io.Reader, header FixedHeader) error {
	// 格式：Seq(4) + Timestamp(8)
	if header.Remaining != 12 {
		return ErrMalformedPacket
	}

	seq, err := DecodeUint32(r)
	if err != nil {
		return err
	}
	p.Seq = seq

	ts, err := DecodeUint64(r)
	if err != nil {
		return err
	}
	p.Timestamp = ts
	return nil
}

// PongPacket PONG 数据包（增强版，携带 RTT 信息和服务器状态）
type PongPacket struct {
	Seq  uint32 // 序号，回显 Ping 的序号
	Echo uint64 // 回显 Ping 的时间戳
	Load uint8  // 服务器负载 (0-100%)
}

// NewPongPacket 创建新的 PONG 包
func NewPongPacket(seq uint32, echo uint64, load uint8) *PongPacket {
	return &PongPacket{Seq: seq, Echo: echo, Load: load}
}

func (p *PongPacket) Type() PacketType {
	return PONG
}

func (p *PongPacket) encode(w io.Writer) error {
	// 格式：Seq(4) + Echo(8) + Load(1)
	if err := EncodeUint32(w, p.Seq); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.Echo); err != nil {
		return err
	}

	return WriteByte(w, p.Load)
}

func (p *PongPacket) decode(r io.Reader, header FixedHeader) error {
	// 格式：Seq(4) + Echo(8) + Load(1)
	if header.Remaining != 13 {
		return ErrMalformedPacket
	}

	seq, err := DecodeUint32(r)
	if err != nil {
		return err
	}
	p.Seq = seq

	echo, err := DecodeUint64(r)
	if err != nil {
		return err
	}
	p.Echo = echo

	var load [1]byte
	if _, err := io.ReadFull(r, load[:]); err != nil {
		return err
	}
	p.Load = load[0]
	return nil
}

// DisconnectPacket DISCONNECT 数据包
type DisconnectPacket struct {
	ReasonCode ReasonCode
	Properties *ReasonProperties
}

// NewDisconnectPacket 创建新的 DISCONNECT 包
func NewDisconnectPacket(code ReasonCode) *DisconnectPacket {
	return &DisconnectPacket{
		ReasonCode: code,
		Properties: NewReasonProperties(),
	}
}

func (p *DisconnectPacket) Type() PacketType {
	return DISCONNECT
}

func (p *DisconnectPacket) encode(w io.Writer) error {
	// 正常断开且无属性时使用空包体（Remaining = 0）。
	// 注意：这里只能返回 nil，不能再写一次固定头，否则包体会嵌套一个头部。
	if p.ReasonCode == ReasonNormalDisconnect && p.Properties == nil {
		return nil
	}

	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}

	props := p.Properties
	if props == nil {
		props = NewReasonProperties()
	}

	return props.Encode(w)
}

func (p *DisconnectPacket) decode(r io.Reader, header FixedHeader) error {
	if header.Remaining == 0 {
		p.ReasonCode = ReasonNormalDisconnect
		return nil
	}

	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code[0])

	if header.Remaining > 1 {
		p.Properties = NewReasonProperties()
		if err := p.Properties.Decode(r); err != nil {
			return err
		}
	}

	return nil
}

// AuthPacket AUTH 数据包
type AuthPacket struct {
	ReasonCode ReasonCode
	Properties *AuthProperties
}

// NewAuthPacket 创建新的 AUTH 包
func NewAuthPacket(code ReasonCode) *AuthPacket {
	return &AuthPacket{
		ReasonCode: code,
		Properties: NewAuthProperties(),
	}
}

func (p *AuthPacket) Type() PacketType {
	return AUTH
}

func (p *AuthPacket) encode(w io.Writer) error {
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}

	props := p.Properties
	if props == nil {
		props = NewAuthProperties()
	}

	return props.Encode(w)
}

func (p *AuthPacket) decode(r io.Reader, header FixedHeader) error {
	if header.Remaining == 0 {
		p.ReasonCode = ReasonSuccess
		return nil
	}

	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code[0])

	if header.Remaining > 1 {
		p.Properties = NewAuthProperties()
		if err := p.Properties.Decode(r); err != nil {
			return err
		}
	}

	return nil
}

// TracePacket TRACE 数据包（用于消息追踪和调试）
type TracePacket struct {
	TraceID    string            // 追踪 ID
	Event      string            // 事件类型（如 "publish", "subscribe", "deliver" 等）
	ClientID   string            // 客户端 ID
	Topic      string            // 相关主题（可选）
	Timestamp  uint64            // 时间戳（Unix 毫秒）
	Details    map[string]string // 额外详情
	Properties *ReasonProperties // 额外属性
}

// NewTracePacket 创建新的 TRACE 包
func NewTracePacket(traceID, event, clientID string) *TracePacket {
	return &TracePacket{
		TraceID:    traceID,
		Event:      event,
		ClientID:   clientID,
		Details:    make(map[string]string),
		Properties: NewReasonProperties(),
	}
}

func (p *TracePacket) Type() PacketType {
	return TRACE
}

func (p *TracePacket) encode(w io.Writer) error {
	// 编码字段
	if err := EncodeString(w, p.TraceID); err != nil {
		return err
	}
	if err := EncodeString(w, p.Event); err != nil {
		return err
	}
	if err := EncodeString(w, p.ClientID); err != nil {
		return err
	}
	if err := EncodeString(w, p.Topic); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.Timestamp); err != nil {
		return err
	}

	// 编码 Details（按 key 排序，保证编码结果确定）
	if len(p.Details) > maxUserProperties {
		return fmt.Errorf("too many trace details: %d (max %d)", len(p.Details), maxUserProperties)
	}
	if err := EncodeUint16(w, uint16(len(p.Details))); err != nil {
		return err
	}
	detailKeys := make([]string, 0, len(p.Details))
	for k := range p.Details {
		detailKeys = append(detailKeys, k)
	}
	sort.Strings(detailKeys)
	for _, k := range detailKeys {
		if err := EncodeString(w, k); err != nil {
			return err
		}
		if err := EncodeString(w, p.Details[k]); err != nil {
			return err
		}
	}

	// 编码属性
	props := p.Properties
	if props == nil {
		props = NewReasonProperties()
	}

	return props.Encode(w)
}

func (p *TracePacket) decode(r io.Reader, header FixedHeader) error {
	var err error

	p.TraceID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Event, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.ClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Topic, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Timestamp, err = DecodeUint64(r)
	if err != nil {
		return err
	}

	// 解码 Details
	detailsCount, err := DecodeUint16(r)
	if err != nil {
		return err
	}

	// 每个条目至少 4 字节（两个空字符串的长度前缀）。先按剩余字节校验，
	// 避免畸形包用极小的报文触发数 MB 的 map 预分配。
	if br, ok := r.(*bytes.Reader); ok && uint64(detailsCount)*4 > uint64(br.Len()) {
		return ErrMalformedPacket
	}

	p.Details = make(map[string]string, detailsCount)
	for range detailsCount {
		key, err := DecodeString(r)
		if err != nil {
			return err
		}
		value, err := DecodeString(r)
		if err != nil {
			return err
		}
		p.Details[key] = value
	}

	// 解码属性
	p.Properties = NewReasonProperties()

	return p.Properties.Decode(r)
}

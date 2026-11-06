package packet

import "io"

// stream 控制面：只负责在已认证的主连接上协商与转发。
// 真正的字节流走独立的 stream 端口（见 stream/packet），不占用主连接。
//
// 能力声明复用 CONNECT/CONNACK 的 spare bit，不再有独立的 HELLO 包。

// StreamOpenPacket STREAM_OPEN：发起方申请与目标客户端建立流。
type StreamOpenPacket struct {
	RequestID  uint64
	Target     string
	Topic      string
	Metadata   []byte
	Properties map[string]string
}

func (p *StreamOpenPacket) Type() PacketType { return STREAM_OPEN }

func (p *StreamOpenPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := EncodeString(w, p.Target); err != nil {
		return err
	}
	if err := EncodeString(w, p.Topic); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Metadata); err != nil {
		return err
	}
	return EncodeStringMap(w, p.Properties)
}

func (p *StreamOpenPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.Target, err = DecodeString(r); err != nil {
		return err
	}
	if p.Topic, err = DecodeString(r); err != nil {
		return err
	}
	if p.Metadata, err = DecodeBytes32(r); err != nil {
		return err
	}
	p.Properties, err = DecodeStringMap(r)
	return err
}

// StreamOpenOkPacket STREAM_OPEN_OK：服务端返回申请结果。
//
// 成功时携带 streamID、数据面地址、一次性票据（ticket）以及接受方生成的 challenge。
// ticket 与 challenge 都必须在 BIND 中回显，服务端才允许绑定。
type StreamOpenOkPacket struct {
	RequestID     uint64
	ReasonCode    ReasonCode
	Message       string
	StreamID      uint64
	StreamAddress string
	TLS           bool
	Ticket        []byte
	Challenge     []byte
	Metadata      []byte // 接受方返回的元数据
	Properties    map[string]string
	BindTimeoutMs uint32
}

func (p *StreamOpenOkPacket) Type() PacketType { return STREAM_OPEN_OK }

func (p *StreamOpenOkPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.RequestID); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}
	if err := EncodeString(w, p.Message); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	if err := EncodeString(w, p.StreamAddress); err != nil {
		return err
	}
	var tls uint8
	if p.TLS {
		tls = 1
	}
	if err := WriteByte(w, tls); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Ticket); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Challenge); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Metadata); err != nil {
		return err
	}
	if err := EncodeStringMap(w, p.Properties); err != nil {
		return err
	}
	return EncodeUint32(w, p.BindTimeoutMs)
}

func (p *StreamOpenOkPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.RequestID, err = DecodeUint64(r); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)
	if p.Message, err = DecodeString(r); err != nil {
		return err
	}
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.StreamAddress, err = DecodeString(r); err != nil {
		return err
	}
	tls, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.TLS = tls&1 != 0
	if p.Ticket, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Challenge, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Metadata, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Properties, err = DecodeStringMap(r); err != nil {
		return err
	}
	p.BindTimeoutMs, err = DecodeUint32(r)
	return err
}

// StreamRequestPacket STREAM_REQUEST：服务端把建流请求转发给目标客户端。
//
// 携带服务端票据与数据面地址：目标接受后即可直接连接数据面，无需额外往返。
type StreamRequestPacket struct {
	StreamID      uint64
	Source        string
	Topic         string
	Metadata      []byte
	Properties    map[string]string
	Ticket        []byte
	StreamAddress string
	TLS           bool
	BindTimeoutMs uint32
}

func (p *StreamRequestPacket) Type() PacketType { return STREAM_REQUEST }

func (p *StreamRequestPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	if err := EncodeString(w, p.Source); err != nil {
		return err
	}
	if err := EncodeString(w, p.Topic); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Metadata); err != nil {
		return err
	}
	if err := EncodeStringMap(w, p.Properties); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Ticket); err != nil {
		return err
	}
	if err := EncodeString(w, p.StreamAddress); err != nil {
		return err
	}
	var tls uint8
	if p.TLS {
		tls = 1
	}
	if err := WriteByte(w, tls); err != nil {
		return err
	}
	return EncodeUint32(w, p.BindTimeoutMs)
}

func (p *StreamRequestPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	if p.Source, err = DecodeString(r); err != nil {
		return err
	}
	if p.Topic, err = DecodeString(r); err != nil {
		return err
	}
	if p.Metadata, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Properties, err = DecodeStringMap(r); err != nil {
		return err
	}
	if p.Ticket, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.StreamAddress, err = DecodeString(r); err != nil {
		return err
	}
	tls, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.TLS = tls&1 != 0
	p.BindTimeoutMs, err = DecodeUint32(r)
	return err
}

// StreamAcceptPacket STREAM_ACCEPT：目标客户端接受或拒绝建流请求。
//
// 接受时必须携带由目标客户端生成的 challenge（建议 32 字节随机数）。
// 使用 StreamID 关联，不再需要额外的 RequestID。
type StreamAcceptPacket struct {
	StreamID   uint64
	Accepted   bool
	ReasonCode ReasonCode
	Message    string
	Challenge  []byte
	Metadata   []byte
	Properties map[string]string
}

func (p *StreamAcceptPacket) Type() PacketType { return STREAM_ACCEPT }

func (p *StreamAcceptPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	var accepted uint8
	if p.Accepted {
		accepted = 1
	}
	if err := WriteByte(w, accepted); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}
	if err := EncodeString(w, p.Message); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Challenge); err != nil {
		return err
	}
	if err := EncodeBytes32(w, p.Metadata); err != nil {
		return err
	}
	return EncodeStringMap(w, p.Properties)
}

func (p *StreamAcceptPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	accepted, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.Accepted = accepted&1 != 0
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)
	if p.Message, err = DecodeString(r); err != nil {
		return err
	}
	if p.Challenge, err = DecodeBytes32(r); err != nil {
		return err
	}
	if p.Metadata, err = DecodeBytes32(r); err != nil {
		return err
	}
	p.Properties, err = DecodeStringMap(r)
	return err
}

// StreamAbortPacket STREAM_ABORT：服务端通知某一端流已中止（对端断开、超时等）。
type StreamAbortPacket struct {
	StreamID   uint64
	ReasonCode ReasonCode
	Message    string
}

func (p *StreamAbortPacket) Type() PacketType { return STREAM_ABORT }

func (p *StreamAbortPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	if err := WriteByte(w, byte(p.ReasonCode)); err != nil {
		return err
	}
	return EncodeString(w, p.Message)
}

func (p *StreamAbortPacket) decode(r io.Reader, _ FixedHeader) error {
	var err error
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)
	p.Message, err = DecodeString(r)
	return err
}

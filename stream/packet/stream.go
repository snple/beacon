package packet

import (
	"bytes"
	"io"
)

// StreamOpenPacket 打开流请求包
// 客户端A -> Core -> 客户端B
type StreamOpenPacket struct {
	StreamID       uint32            // 流ID（由Core生成）
	Topic          string            // 流主题/用途
	SourceClientID string            // 发起方客户端ID（Core填充）
	TargetClientID string            // 目标方客户端ID
	Metadata       []byte            // 业务元数据
	Properties     map[string]string // 扩展属性
}

func (p *StreamOpenPacket) Type() PacketType {
	return STREAM_OPEN
}

func (p *StreamOpenPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// StreamID
	if err := EncodeUint32(&buf, p.StreamID); err != nil {
		return err
	}

	// Topic
	if err := EncodeString(&buf, p.Topic); err != nil {
		return err
	}

	// SourceClientID
	if err := EncodeString(&buf, p.SourceClientID); err != nil {
		return err
	}

	// TargetClientID
	if err := EncodeString(&buf, p.TargetClientID); err != nil {
		return err
	}

	// Metadata
	if err := EncodeBinary(&buf, p.Metadata); err != nil {
		return err
	}

	// Properties
	if err := EncodeProperties(&buf, p.Properties); err != nil {
		return err
	}

	// 写入固定头部和包体
	header := FixedHeader{
		Type:   STREAM_OPEN,
		Length: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *StreamOpenPacket) Decode(r io.Reader) error {
	var err error

	p.StreamID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	p.Topic, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.SourceClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.TargetClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Metadata, err = DecodeBinary(r)
	if err != nil {
		return err
	}

	p.Properties, err = DecodeProperties(r)
	return err
}

// StreamAckPacket 流确认响应包
// 客户端B -> Core -> 客户端A
type StreamAckPacket struct {
	StreamID   uint32            // 流ID
	ReasonCode ReasonCode        // 原因码（Success表示接受，其他表示拒绝）
	Message    string            // 可选消息
	Metadata   []byte            // 响应元数据
	Properties map[string]string // 扩展属性
}

func (p *StreamAckPacket) Type() PacketType {
	return STREAM_ACK
}

func (p *StreamAckPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// StreamID
	if err := EncodeUint32(&buf, p.StreamID); err != nil {
		return err
	}

	// ReasonCode
	if err := EncodeUint8(&buf, uint8(p.ReasonCode)); err != nil {
		return err
	}

	// Message
	if err := EncodeString(&buf, p.Message); err != nil {
		return err
	}

	// Metadata
	if err := EncodeBinary(&buf, p.Metadata); err != nil {
		return err
	}

	// Properties
	if err := EncodeProperties(&buf, p.Properties); err != nil {
		return err
	}

	// 写入固定头部和包体
	header := FixedHeader{
		Type:   STREAM_ACK,
		Length: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *StreamAckPacket) Decode(r io.Reader) error {
	var err error

	p.StreamID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	code, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code)

	p.Message, err = DecodeString(r)
	if err != nil {
		return err
	}

	p.Metadata, err = DecodeBinary(r)
	if err != nil {
		return err
	}

	p.Properties, err = DecodeProperties(r)
	return err
}

// StreamClosePacket 关闭流通知包
type StreamClosePacket struct {
	StreamID   uint32     // 流ID
	ReasonCode ReasonCode // 关闭原因
	Message    string     // 可选消息
}

func (p *StreamClosePacket) Type() PacketType {
	return STREAM_CLOSE
}

func (p *StreamClosePacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// StreamID
	if err := EncodeUint32(&buf, p.StreamID); err != nil {
		return err
	}

	// ReasonCode
	if err := EncodeUint8(&buf, uint8(p.ReasonCode)); err != nil {
		return err
	}

	// Message
	if err := EncodeString(&buf, p.Message); err != nil {
		return err
	}

	// 写入固定头部和包体
	header := FixedHeader{
		Type:   STREAM_CLOSE,
		Length: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *StreamClosePacket) Decode(r io.Reader) error {
	var err error

	p.StreamID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	code, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code)

	p.Message, err = DecodeString(r)
	return err
}

// StreamReadyPacket 客户端准备接收流的信号包
// 客户端 -> Core，表示已准备好等待流连接
type StreamReadyPacket struct {
	Properties map[string]string // 扩展属性（可选）
}

func (p *StreamReadyPacket) Type() PacketType {
	return STREAM_READY
}

func (p *StreamReadyPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// Properties
	if err := EncodeProperties(&buf, p.Properties); err != nil {
		return err
	}

	// 写入固定头部和包体
	header := FixedHeader{
		Type:   STREAM_READY,
		Length: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *StreamReadyPacket) Decode(r io.Reader) error {
	var err error
	p.Properties, err = DecodeProperties(r)
	return err
}

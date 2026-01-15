package packet

import (
	"bytes"
	"io"
)

// StreamDataPacket 流数据传输包
// 用于在建立的流上传输实际数据
type StreamDataPacket struct {
	StreamID   uint32            // 流ID
	SequenceID uint32            // 序列号
	Flags      uint8             // 标志位（保留，可用于标识数据类型等）
	Data       []byte            // 实际数据
	Properties map[string]string // 可选属性（用于携带元数据）
}

func (p *StreamDataPacket) Type() PacketType {
	return STREAM_DATA
}

func (p *StreamDataPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// StreamID
	if err := EncodeUint32(&buf, p.StreamID); err != nil {
		return err
	}

	// SequenceID
	if err := EncodeUint32(&buf, p.SequenceID); err != nil {
		return err
	}

	// Flags
	if err := EncodeUint8(&buf, p.Flags); err != nil {
		return err
	}

	// Data
	if err := EncodeBinary(&buf, p.Data); err != nil {
		return err
	}

	// Properties
	if err := EncodeProperties(&buf, p.Properties); err != nil {
		return err
	}

	// 写入固定头部和包体
	header := FixedHeader{
		Type:   STREAM_DATA,
		Length: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *StreamDataPacket) Decode(r io.Reader) error {
	var err error

	p.StreamID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	p.SequenceID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	p.Flags, err = DecodeUint8(r)
	if err != nil {
		return err
	}

	p.Data, err = DecodeBinary(r)
	if err != nil {
		return err
	}

	p.Properties, err = DecodeProperties(r)
	return err
}

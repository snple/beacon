package packet

import (
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

func (p *StreamDataPacket) encode(w io.Writer) error {
	// StreamID
	if err := EncodeUint32(w, p.StreamID); err != nil {
		return err
	}

	// SequenceID
	if err := EncodeUint32(w, p.SequenceID); err != nil {
		return err
	}

	// Flags
	if err := EncodeUint8(w, p.Flags); err != nil {
		return err
	}

	// Data
	if err := EncodeBinary(w, p.Data); err != nil {
		return err
	}

	// Properties
	return EncodeProperties(w, p.Properties)

}

func (p *StreamDataPacket) decode(r io.Reader) error {
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

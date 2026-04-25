package packet

import (
	"bytes"
	"fmt"
	"io"
)

// 安全上限：防止恶意 length 导致 OOM。
// 如需更大负载，建议做分片/应用层流式传输。
const MaxPacketSize uint32 = 16 * 1024 * 1024 // 16MiB

// Packet 数据包接口
type Packet interface {
	Type() PacketType
	encode(w io.Writer) error
	decode(r io.Reader) error
}

// FixedHeader 固定头部（所有数据包通用）
// 格式: [PacketType:1字节][Length:4字节]
type FixedHeader struct {
	Type   PacketType // 数据包类型
	Length uint32     // 剩余长度（不包含固定头部）
}

// Encode 编码固定头部
func (h *FixedHeader) Encode(w io.Writer) error {
	if err := EncodeUint8(w, uint8(h.Type)); err != nil {
		return err
	}
	return EncodeUint32(w, h.Length)
}

// Decode 解码固定头部
func (h *FixedHeader) Decode(r io.Reader) error {
	typ, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	h.Type = PacketType(typ)

	h.Length, err = DecodeUint32(r)
	return err
}

// ReadPacket 从连接读取数据包
func ReadPacket(r io.Reader) (Packet, error) {
	// 读取固定头部
	var header FixedHeader
	if err := header.Decode(r); err != nil {
		return nil, err
	}
	if header.Length > MaxPacketSize {
		return nil, &StreamProtocolError{Code: MalformedPacket, Message: fmt.Sprintf("packet too large: %d", header.Length)}
	}

	bodyLen := int(header.Length)

	// 读取包体
	body := make([]byte, bodyLen)
	if bodyLen > 0 {
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}
	}

	// 根据类型创建对应的包
	var pkt Packet
	switch header.Type {
	case CONNECT:
		pkt = &ConnectPacket{}
	case CONNACK:
		pkt = &ConnackPacket{}
	case STREAM_OPEN:
		pkt = &StreamOpenPacket{}
	case STREAM_ACK:
		pkt = &StreamAckPacket{}
	case STREAM_DATA:
		pkt = &StreamDataPacket{}
	case STREAM_CLOSE:
		pkt = &StreamClosePacket{}
	case STREAM_READY:
		pkt = &StreamReadyPacket{}
	case PING:
		pkt = &PingPacket{}
	case PONG:
		pkt = &PongPacket{}
	default:
		return nil, &StreamProtocolError{Code: MalformedPacket, Message: "unknown packet type"}
	}

	// 解码包体
	if err := pkt.decode(bytes.NewReader(body)); err != nil {
		return nil, fmt.Errorf("failed to decode %v packet body: %w", header.Type, err)
	}

	return pkt, nil
}

// WritePacket 写入数据包到连接
func WritePacket(w io.Writer, pkt Packet) error {
	// 先编码包体到缓冲区
	var buf bytes.Buffer
	if err := pkt.encode(&buf); err != nil {
		return fmt.Errorf("failed to encode %v packet: %w", pkt.Type(), err)
	}

	// 检查包大小是否超过限制
	if buf.Len() > int(MaxPacketSize) {
		return &StreamProtocolError{Code: MalformedPacket, Message: fmt.Sprintf("packet too large: %d", buf.Len())}
	}

	// 构造固定头部
	header := FixedHeader{
		Type:   pkt.Type(),
		Length: uint32(buf.Len()),
	}

	// 先写入固定头部
	if err := header.Encode(w); err != nil {
		return err
	}

	// 再写入包体
	_, err := w.Write(buf.Bytes())
	return err
}

// StreamProtocolError 流协议错误
type StreamProtocolError struct {
	Code    ReasonCode
	Message string
}

func (e *StreamProtocolError) Error() string {
	return e.Message
}

// PingPacket 心跳请求
type PingPacket struct{}

func (p *PingPacket) Type() PacketType {
	return PING
}

func (p *PingPacket) encode(w io.Writer) error {
	return nil
}

func (p *PingPacket) decode(r io.Reader) error {
	return nil
}

// PongPacket 心跳响应
type PongPacket struct{}

func (p *PongPacket) Type() PacketType {
	return PONG
}

func (p *PongPacket) encode(w io.Writer) error {
	return nil
}

func (p *PongPacket) decode(r io.Reader) error {
	return nil
}

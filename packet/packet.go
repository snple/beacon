package packet

import (
	"bytes"
	"fmt"
	"io"
)

// minRemainingLength 各包类型所需的最小 Remaining Length
// 只列出有明确最小值的类型，未列出的类型（如 DISCONNECT, AUTH）允许 Remaining = 0
var minRemainingLength = map[PacketType]uint32{
	CONNECT:     16, // ProtocolName(2+0) + Version(1) + ClientID(2+0) + KeepAlive(2) + SessionTimeout(4) + Properties(4+0) + WillFlags(1)
	CONNACK:     6,  // SessionPresent(1) + ReasonCode(1) + Properties(4+0)
	PUBLISH:     19, // Flags(1) + Topic(2+0) + PacketID(12) + Properties(4+0)
	PUBACK:      12, // PacketID(12)
	SUBSCRIBE:   19, // PacketID(12) + Properties(4+0) + Topic(2+0) + Options(1)
	SUBACK:      17, // PacketID(12) + Properties(4+0) + ReasonCode(1)
	UNSUBSCRIBE: 18, // PacketID(12) + Properties(4+0) + Topic(2+0)
	UNSUBACK:    17, // PacketID(12) + Properties(4+0) + ReasonCode(1)
	PING:        12, // Seq(4) + Timestamp(8)
	PONG:        13, // Seq(4) + Echo(8) + Load(1)
}

// Packet 数据包接口
type Packet interface {
	// Type 返回数据包类型
	Type() PacketType

	// encode 编码数据包到 Writer
	encode(w io.Writer) error

	// Decode 从 Reader 解码数据包
	decode(r io.Reader, header FixedHeader) error
}

// ReadPacket 从 Reader 读取并解析数据包
// maxPacketSize: 最大允许的包大小（0 表示无限制）
func ReadPacket(r io.Reader, maxPacketSize uint32) (Packet, error) {
	var header FixedHeader
	if err := header.Decode(r); err != nil {
		return nil, err
	}

	totalSize := header.Remaining + 5

	// 检查包大小是否超过限制
	if maxPacketSize > 0 && totalSize > maxPacketSize {
		return nil, NewPacketTooLargeError(
			totalSize,
			maxPacketSize,
		)
	}

	// 即使 maxPacketSize=0（无限制），也不允许超过协议硬上限
	if totalSize > MaxPacketSize {
		return nil, NewPacketTooLargeError(
			totalSize,
			MaxPacketSize,
		)
	}

	// 检查最小 Remaining Length（防止明显畸形的包分配内存）
	if minLen, ok := minRemainingLength[header.Type]; ok && header.Remaining < minLen {
		return nil, fmt.Errorf("%w: %v requires at least %d bytes, got %d",
			ErrMalformedPacket, header.Type, minLen, header.Remaining)
	}

	// 读取剩余数据
	data := make([]byte, header.Remaining)
	if header.Remaining > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
	}

	// 创建对应类型的包
	var pkt Packet
	switch header.Type {
	case CONNECT:
		pkt = &ConnectPacket{}
	case CONNACK:
		pkt = &ConnackPacket{}
	case PUBLISH:
		pkt = &PublishPacket{}
	case PUBACK:
		pkt = &PubackPacket{}
	case SUBSCRIBE:
		pkt = &SubscribePacket{}
	case SUBACK:
		pkt = &SubackPacket{}
	case UNSUBSCRIBE:
		pkt = &UnsubscribePacket{}
	case UNSUBACK:
		pkt = &UnsubackPacket{}
	case PING:
		pkt = &PingPacket{}
	case PONG:
		pkt = &PongPacket{}
	case DISCONNECT:
		pkt = &DisconnectPacket{}
	case AUTH:
		pkt = &AuthPacket{}
	case TRACE:
		pkt = &TracePacket{}
	default:
		return nil, ErrInvalidPacketType
	}

	// 解码包体
	buf := bytes.NewReader(data)
	if err := pkt.decode(buf, header); err != nil {
		return nil, fmt.Errorf("failed to decode %v packet: %w", header.Type, err)
	}
	if buf.Len() > 0 {
		return nil, fmt.Errorf("failed to decode %v packet: %w", header.Type, ErrMalformedPacket)
	}

	return pkt, nil
}

// WritePacket 将数据包写入 Writer
func WritePacket(w io.Writer, pkt Packet, maxPacketSize uint32) error {
	// 先编码包体到缓冲区
	var buf bytes.Buffer
	if err := pkt.encode(&buf); err != nil {
		return fmt.Errorf("failed to encode %v packet: %w", pkt.Type(), err)
	}

	// 检查包大小是否超过限制
	totalSize := uint32(buf.Len() + 5) // 包体大小 + 固定头部大小
	if maxPacketSize > 0 && totalSize > maxPacketSize {
		return NewPacketTooLargeError(
			totalSize,
			maxPacketSize,
		)
	}

	// 构造固定头部
	header := FixedHeader{
		Type:      pkt.Type(),
		Remaining: uint32(buf.Len()),
	}

	// 先写入固定头部
	if err := header.Encode(w); err != nil {
		return err
	}

	// 再写入包体
	_, err := w.Write(buf.Bytes())
	return err
}

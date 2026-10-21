package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// minRemainingLength 各包类型所需的最小 Remaining Length
// 只列出有明确最小值的类型，未列出的类型（如 DISCONNECT, AUTH）允许 Remaining = 0
var minRemainingLength = map[PacketType]uint32{
	CONNECT:     17, // ProtocolName(2+0) + Version(1) + ClientID(2+0) + KeepAlive(2) + SessionTimeout(4) + SessionFlags(1) + Properties(4+0) + WillFlags(1)
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

// ReadPacketWithSize 从 Reader 读取并解析数据包，并返回读取的完整线格式字节数
// （固定头 + 包体）。maxPacketSize: 最大允许的包大小（0 表示无限制）
func ReadPacketWithSize(r io.Reader, maxPacketSize uint32) (Packet, uint32, error) {
	var header FixedHeader
	if err := header.Decode(r); err != nil {
		return nil, 0, err
	}

	// 注意：header.Remaining 直接来自对端（4 字节无符号，无上限校验），
	// 可能为 0xFFFFFFFB ~ 0xFFFFFFFF。此时若直接计算 totalSize = Remaining + 5，
	// uint32 会回绕成 0~4 的极小值，从而绕过下方所有大小检查，最终执行
	// make([]byte, Remaining) 分配约 4GB 内存导致进程 OOM（远程可触发的安全漏洞）。
	// 因此必须用减法形式（Remaining > Max-5）先对 Remaining 本身做溢出安全校验，
	// 再安全地计算 totalSize。

	// 检查包大小是否超过限制（maxPacketSize=0 表示无限制）
	if maxPacketSize > 0 && (maxPacketSize < 5 || header.Remaining > maxPacketSize-5) {
		return nil, 0, NewPacketTooLargeError(
			header.Remaining+5,
			maxPacketSize,
		)
	}

	// 即使 maxPacketSize=0（无限制），也不允许超过协议硬上限
	if header.Remaining > MaxPacketSize-5 {
		return nil, 0, NewPacketTooLargeError(
			header.Remaining+5,
			MaxPacketSize,
		)
	}

	// 检查最小 Remaining Length（防止明显畸形的包分配内存）
	if minLen, ok := minRemainingLength[header.Type]; ok && header.Remaining < minLen {
		return nil, 0, fmt.Errorf("%w: %v requires at least %d bytes, got %d",
			ErrMalformedPacket, header.Type, minLen, header.Remaining)
	}

	// 读取剩余数据
	data := make([]byte, header.Remaining)
	if header.Remaining > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, 0, err
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
		return nil, 0, ErrInvalidPacketType
	}

	// 解码包体
	buf := bytes.NewReader(data)
	if err := pkt.decode(buf, header); err != nil {
		return nil, 0, fmt.Errorf("failed to decode %v packet: %w", header.Type, err)
	}
	if buf.Len() > 0 {
		return nil, 0, fmt.Errorf("failed to decode %v packet: %w", header.Type, ErrMalformedPacket)
	}

	return pkt, header.Remaining + 5, nil
}

// ReadPacket 从 Reader 读取并解析数据包。
// maxPacketSize: 最大允许的包大小（0 表示无限制）
func ReadPacket(r io.Reader, maxPacketSize uint32) (Packet, error) {
	pkt, _, err := ReadPacketWithSize(r, maxPacketSize)
	return pkt, err
}

// WriteError 表示数据包写入底层 Writer 时发生的错误（区别于编码错误）。
// 出现该错误时流中可能已经写入了部分字节，调用方应把连接视为不可用。
type WriteError struct {
	Err error
}

func (e *WriteError) Error() string { return fmt.Sprintf("write packet: %v", e.Err) }

func (e *WriteError) Unwrap() error { return e.Err }

// EncodePacket 将数据包编码为完整的线格式（固定头 + 包体）。
// 编码失败发生在任何字节写出之前，调用方可以据此区分
// “消息无法编码（不应断开连接）”与“写入失败（连接不可信）”。
func EncodePacket(pkt Packet, maxPacketSize uint32) ([]byte, error) {
	var body bytes.Buffer
	if err := pkt.encode(&body); err != nil {
		return nil, fmt.Errorf("failed to encode %v packet: %w", pkt.Type(), err)
	}

	totalSize := uint32(body.Len() + 5) // 包体大小 + 固定头部大小
	if maxPacketSize > 0 && totalSize > maxPacketSize {
		return nil, NewPacketTooLargeError(
			totalSize,
			maxPacketSize,
		)
	}

	frame := make([]byte, 5+body.Len())
	frame[0] = byte(pkt.Type())
	binary.BigEndian.PutUint32(frame[1:5], uint32(body.Len()))
	copy(frame[5:], body.Bytes())

	return frame, nil
}

// WritePacketWithSize 将数据包写入 Writer，并返回实际写入的字节数。
// 固定头与包体一次性写出，避免头/体之间出现部分写入的中间状态。
func WritePacketWithSize(w io.Writer, pkt Packet, maxPacketSize uint32) (int, error) {
	frame, err := EncodePacket(pkt, maxPacketSize)
	if err != nil {
		return 0, err
	}

	n, err := w.Write(frame)
	if err != nil {
		return n, &WriteError{Err: err}
	}
	if n != len(frame) {
		return n, &WriteError{Err: io.ErrShortWrite}
	}

	return n, nil
}

// WritePacket 将数据包写入 Writer。
func WritePacket(w io.Writer, pkt Packet, maxPacketSize uint32) error {
	_, err := WritePacketWithSize(w, pkt, maxPacketSize)
	return err
}

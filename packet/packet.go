package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// minRemainingLength 各包类型所需的最小 Remaining Length。
var minRemainingLength = map[PacketType]uint32{
	CONNECT:      16, // Version(1)+Flags(1)+ClientID(2)+KeepAlive(2)+MaxPacketSize(4)+AuthMethod(2)+AuthData(4)
	CONNACK:      54, // Flags(1)+ReasonCode(1)+ClientID(2)+SessionID(8)+KeepAlive(2)+ReceiveMaximum(2)+Pending(12)+Limits(26)
	PUSH:         30, // RequestID(4)+Flags(1)+Record(25)
	PUSH_OK:      26, // RequestID(4)+Flags(1)+Reason(1)+ID(12)+Matched(4)+Dropped(4)
	PULL:         15, // RequestID(4)+Flags(1)+WaitMs(4)+MaxCount(2)+MaxBytes(4)
	PULL_OK:      20, // RequestID(4)+Flags(1)+Reason(1)+Count(2)+Pending(12)
	COMMIT:       6,  // RequestID(4)+Count(2)；Entries 可选
	COMMIT_OK:    22, // RequestID(4)+Committed(2)+Nacked(2)+Unknown(2)+Pending(12)
	PING:         12, // Seq(4)+Timestamp(8)
	PONG:         12, // Seq(4)+Echo(8)
	DISCONNECT:   1,  // ReasonCode(1)
	DECLARE:      7,  // RequestID(4)+Flags(1)+Count(2)
	DECLARE_RESP: 47, // RequestID(4)+Flags(1)+Accepted(2)+Count(2)+Pending(12)+Limits(26)
}

// Packet 数据包接口
type Packet interface {
	Type() PacketType
	encode(w io.Writer) error
	decode(r io.Reader, header FixedHeader) error
}

// ReadPacketWithSize 从 Reader 读取并解析数据包，并返回读取的完整线格式字节数
// （固定头 + 包体）。maxPacketSize: 最大允许的包大小（0 表示无限制）
func ReadPacketWithSize(r io.Reader, maxPacketSize uint32) (Packet, uint32, error) {
	var header FixedHeader
	if err := header.Decode(r); err != nil {
		return nil, 0, err
	}

	// 必须用减法形式先对 Remaining 本身做溢出安全校验，再计算总长度，
	// 否则 Remaining 接近 uint32 上限时 totalSize 会回绕，绕过大小检查。
	if maxPacketSize > 0 && (maxPacketSize < 5 || header.Remaining > maxPacketSize-5) {
		return nil, 0, NewPacketTooLargeError(header.Remaining+5, maxPacketSize)
	}
	if header.Remaining > MaxPacketSize-5 {
		return nil, 0, NewPacketTooLargeError(header.Remaining+5, MaxPacketSize)
	}

	if minLen, ok := minRemainingLength[header.Type]; ok && header.Remaining < minLen {
		return nil, 0, fmt.Errorf("%w: %v requires at least %d bytes, got %d",
			ErrMalformedPacket, header.Type, minLen, header.Remaining)
	}

	// 先确认包类型已知，再按声明长度分配包体，避免未知类型也能触发大分配。
	var pkt Packet
	switch header.Type {
	case CONNECT:
		pkt = &ConnectPacket{}
	case CONNACK:
		pkt = &ConnackPacket{}
	case PUSH:
		pkt = &PushPacket{}
	case PUSH_OK:
		pkt = &PushOkPacket{}
	case PULL:
		pkt = &PullPacket{}
	case PULL_OK:
		pkt = &PullOkPacket{}
	case COMMIT:
		pkt = &CommitPacket{}
	case COMMIT_OK:
		pkt = &CommitOkPacket{}
	case PING:
		pkt = &PingPacket{}
	case PONG:
		pkt = &PongPacket{}
	case DISCONNECT:
		pkt = &DisconnectPacket{}
	case DECLARE:
		pkt = &DeclarePacket{}
	case DECLARE_RESP:
		pkt = &DeclareRespPacket{}
	default:
		return nil, 0, ErrInvalidPacketType
	}

	data := make([]byte, header.Remaining)
	if header.Remaining > 0 {
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, 0, err
		}
	}

	buf := bytes.NewReader(data)
	if err := pkt.decode(buf, header); err != nil {
		return nil, 0, fmt.Errorf("failed to decode %v packet: %w", header.Type, err)
	}
	if buf.Len() > 0 {
		return nil, 0, fmt.Errorf("failed to decode %v packet: %w", header.Type, ErrMalformedPacket)
	}

	return pkt, header.Remaining + 5, nil
}

// ReadPacket 从 Reader 读取并解析数据包。maxPacketSize: 最大允许的包大小（0 表示无限制）
func ReadPacket(r io.Reader, maxPacketSize uint32) (Packet, error) {
	pkt, _, err := ReadPacketWithSize(r, maxPacketSize)
	return pkt, err
}

// WriteError 表示数据包写入底层 Writer 时发生的错误（区别于编码错误）。
type WriteError struct{ Err error }

func (e *WriteError) Error() string { return fmt.Sprintf("write packet: %v", e.Err) }

func (e *WriteError) Unwrap() error { return e.Err }

// EncodePacket 将数据包编码为完整的线格式（固定头 + 包体）。
func EncodePacket(pkt Packet, maxPacketSize uint32) ([]byte, error) {
	var body bytes.Buffer
	if err := pkt.encode(&body); err != nil {
		return nil, fmt.Errorf("failed to encode %v packet: %w", pkt.Type(), err)
	}

	totalSize := uint32(body.Len() + 5)
	if maxPacketSize > 0 && totalSize > maxPacketSize {
		return nil, NewPacketTooLargeError(totalSize, maxPacketSize)
	}
	if totalSize > MaxPacketSize {
		return nil, NewPacketTooLargeError(totalSize, MaxPacketSize)
	}

	frame := make([]byte, 5+body.Len())
	frame[0] = byte(pkt.Type())
	binary.BigEndian.PutUint32(frame[1:5], uint32(body.Len()))
	copy(frame[5:], body.Bytes())
	return frame, nil
}

// WritePacketWithSize 将数据包写入 Writer，并返回实际写入的字节数。
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

package packet

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Packet 是数据面握手包。
type Packet interface {
	Type() PacketType
	encode(w io.Writer) error
	decode(r io.Reader) error
}

// FixedHeader 固定头：Type(1) + Length(4，大端)。
type FixedHeader struct {
	Type   PacketType
	Length uint32
}

var minRemainingLength = map[PacketType]uint32{
	BIND:    18, // Version(1)+StreamID(8)+Role(1)+Ticket(4)+Challenge(4)
	BIND_OK: 12, // StreamID(8)+Role(1)+Reason(1)+Message(2)
}

// ReadPacket 从连接读取一个握手包。
func ReadPacket(r io.Reader) (Packet, error) {
	var head [5]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return nil, err
	}

	typ := PacketType(head[0])
	length := binary.BigEndian.Uint32(head[1:])
	if length > MaxPacketSize {
		return nil, &HandshakeError{Code: MalformedPacket, Message: fmt.Sprintf("packet too large: %d", length)}
	}
	if min, ok := minRemainingLength[typ]; !ok {
		return nil, &HandshakeError{Code: MalformedPacket, Message: "unknown packet type"}
	} else if length < min {
		return nil, &HandshakeError{Code: MalformedPacket,
			Message: fmt.Sprintf("%v requires at least %d bytes, got %d", typ, min, length)}
	}

	body := make([]byte, length)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	var pkt Packet
	switch typ {
	case BIND:
		pkt = &BindPacket{}
	case BIND_OK:
		pkt = &BindOkPacket{}
	default:
		return nil, &HandshakeError{Code: MalformedPacket, Message: "unknown packet type"}
	}

	br := bytes.NewReader(body)
	if err := pkt.decode(br); err != nil {
		return nil, fmt.Errorf("failed to decode %v packet: %w", typ, err)
	}
	if br.Len() != 0 {
		return nil, &HandshakeError{Code: MalformedPacket, Message: "trailing bytes in packet body"}
	}
	return pkt, nil
}

// WritePacket 将握手包写入连接。头与体一次写出，避免中间状态。
func WritePacket(w io.Writer, pkt Packet) error {
	var body bytes.Buffer
	if err := pkt.encode(&body); err != nil {
		return fmt.Errorf("failed to encode %v packet: %w", pkt.Type(), err)
	}
	if body.Len() > int(MaxPacketSize) {
		return &HandshakeError{Code: MalformedPacket, Message: fmt.Sprintf("packet too large: %d", body.Len())}
	}

	frame := make([]byte, 5+body.Len())
	frame[0] = byte(pkt.Type())
	binary.BigEndian.PutUint32(frame[1:5], uint32(body.Len()))
	copy(frame[5:], body.Bytes())

	n, err := w.Write(frame)
	if err != nil {
		return err
	}
	if n != len(frame) {
		return io.ErrShortWrite
	}
	return nil
}

// HandshakeError 是 stream 数据面协议错误。
type HandshakeError struct {
	Code    ReasonCode
	Message string
}

func (e *HandshakeError) Error() string { return e.Message }

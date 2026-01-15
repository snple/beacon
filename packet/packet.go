package packet

import (
	"bytes"
	"fmt"
	"io"
)

// Packet 数据包接口
type Packet interface {
	// Type 返回数据包类型
	Type() PacketType

	// Encode 编码数据包到 Writer
	Encode(w io.Writer) error

	// Decode 从 Reader 解码数据包
	Decode(r io.Reader, header FixedHeader) error
}

// ReadPacket 从 Reader 读取并解析数据包
func ReadPacket(r io.Reader) (Packet, error) {
	var header FixedHeader
	if err := header.Decode(r); err != nil {
		return nil, err
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
	case REQUEST:
		pkt = &RequestPacket{}
	case RESPONSE:
		pkt = &ResponsePacket{}
	case REGISTER:
		pkt = &RegisterPacket{}
	case REGACK:
		pkt = &RegackPacket{}
	case UNREGISTER:
		pkt = &UnregisterPacket{}
	case UNREGACK:
		pkt = &UnregackPacket{}
	default:
		return nil, ErrInvalidPacketType
	}

	// 解码包体
	buf := bytes.NewReader(data)
	if err := pkt.Decode(buf, header); err != nil {
		return nil, fmt.Errorf("failed to decode %v packet: %w", header.Type, err)
	}

	return pkt, nil
}

// WritePacket 将数据包写入 Writer
func WritePacket(w io.Writer, pkt Packet) error {
	return pkt.Encode(w)
}

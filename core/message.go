package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
)

// 消息持久化格式：
//
//	[version(1)][flags(1)][PacketID(12)][Timestamp(8)][PUBLISH 线格式]
//
// flags: bit0=Dup, bit1=QoS, bit2=Retain。
// 内容层直接复用 packet 的线格式编解码，不再经过反射式序列化。
const (
	messageFormatVersion byte = 1
	messageHeaderSize         = 1 + 1 + 12 + 8
)

const (
	messageFlagDup    byte = 1 << 0
	messageFlagQoS    byte = 1 << 1
	messageFlagRetain byte = 1 << 2
)

var (
	_ mq.BinaryMarshaler   = Message{}
	_ mq.BinaryUnmarshaler = (*Message)(nil)
)

// Message 内部消息表示
//
// 设计原则：
// - 内容层：直接引用原始 *packet.PublishPacket（视为只读、可共享）
// - 投递层（PacketID/QoS/Dup）由发送队列/持久化层单独管理
type Message struct {
	// 原始 PUBLISH 包（视为只读）
	Packet *packet.PublishPacket

	// 固定头部标志
	Dup    bool       // 重发标志
	QoS    packet.QoS // 服务质量
	Retain bool       // 保留标志

	// 发送相关
	// 分配的包 ID（QoS 0/1 均分配，用于队列 key 与 FIFO 排序；仅 QoS1 用于 ACK 匹配）
	PacketID packet.PacketID

	// 时间相关（非协议字段，仅用于调试/观察）
	Timestamp int64
}

// MarshalBinary 将消息编码为固定二进制格式。
// PUBLISH 内容复用 packet 的线格式，避免反射开销，并保持与协议编码完全一致。
func (m Message) MarshalBinary() ([]byte, error) {
	if m.Packet == nil {
		return nil, ErrInvalidMessage
	}

	frame, err := packet.EncodePacket(m.Packet, 0)
	if err != nil {
		return nil, err
	}

	data := make([]byte, messageHeaderSize, messageHeaderSize+len(frame))
	data[0] = messageFormatVersion

	var flags byte
	if m.Dup {
		flags |= messageFlagDup
	}
	if m.QoS == packet.QoS1 {
		flags |= messageFlagQoS
	}
	if m.Retain {
		flags |= messageFlagRetain
	}
	data[1] = flags

	copy(data[2:14], m.PacketID[:])
	binary.BigEndian.PutUint64(data[14:22], uint64(m.Timestamp))

	return append(data, frame...), nil
}

// UnmarshalBinary 从固定二进制格式解码消息。
func (m *Message) UnmarshalBinary(data []byte) error {
	if len(data) < messageHeaderSize {
		return fmt.Errorf("%w: message too short (%d bytes)", ErrInvalidMessage, len(data))
	}
	if data[0] != messageFormatVersion {
		return fmt.Errorf("%w: unsupported message format version %d", ErrInvalidMessage, data[0])
	}

	flags := data[1]
	m.Dup = flags&messageFlagDup != 0
	m.QoS = packet.QoS((flags & messageFlagQoS) >> 1)
	m.Retain = flags&messageFlagRetain != 0
	copy(m.PacketID[:], data[2:14])
	m.Timestamp = int64(binary.BigEndian.Uint64(data[14:22]))

	r := bytes.NewReader(data[messageHeaderSize:])
	// 以剩余字节数作为包上限：损坏数据里伪造的 Remaining 会在分配缓冲前被拒绝。
	pkt, _, err := packet.ReadPacketWithSize(r, uint32(r.Len()))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}
	if r.Len() != 0 {
		return fmt.Errorf("%w: trailing bytes after packet", ErrInvalidMessage)
	}

	pub, ok := pkt.(*packet.PublishPacket)
	if !ok {
		return fmt.Errorf("%w: unexpected packet type %T", ErrInvalidMessage, pkt)
	}

	m.Packet = pub
	return nil
}

// IsExpired 检查消息是否已过期
func (m *Message) IsExpired() bool {
	if m.Packet == nil || m.Packet.Properties == nil || m.Packet.Properties.ExpiryTime == 0 {
		return false
	}
	return time.Now().Unix() > m.Packet.Properties.ExpiryTime
}

// Copy 复制消息结构体。
// Packet 内容层在 Message 生命周期内视为只读，真正出站时会在 sendMessage 中构造一次性 packet，
// 因此这里共享 Packet 指针即可，避免为每个订阅者做不必要的深拷贝。
func (m *Message) Copy() Message {
	if m == nil {
		return Message{}
	}

	return Message{
		Packet:    m.Packet,
		Dup:       m.Dup,
		QoS:       m.QoS,
		Retain:    m.Retain,
		PacketID:  m.PacketID,
		Timestamp: m.Timestamp,
	}
}

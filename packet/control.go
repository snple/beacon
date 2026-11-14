package packet

import "io"

// PingPacket PING 数据包（携带时间戳用于 RTT 计算）
type PingPacket struct {
	Seq       uint32 // 序号，用于关联 PING/PONG
	Timestamp uint64 // 发送时间戳（Unix 纳秒），用于计算 RTT
}

// NewPingPacket 创建新的 PING 包
func NewPingPacket(seq uint32, timestamp uint64) *PingPacket {
	return &PingPacket{Seq: seq, Timestamp: timestamp}
}

func (p *PingPacket) Type() PacketType { return PING }

func (p *PingPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.Seq); err != nil {
		return err
	}
	return EncodeUint64(w, p.Timestamp)
}

func (p *PingPacket) decode(r io.Reader, header FixedHeader) error {
	if header.Remaining != 12 {
		return ErrMalformedPacket
	}
	var err error
	if p.Seq, err = DecodeUint32(r); err != nil {
		return err
	}
	p.Timestamp, err = DecodeUint64(r)
	return err
}

// PongPacket PONG 数据包
type PongPacket struct {
	Seq  uint32 // 序号，回显 Ping 的序号
	Echo uint64 // 回显 Ping 的时间戳
}

// NewPongPacket 创建新的 PONG 包
func NewPongPacket(seq uint32, echo uint64) *PongPacket {
	return &PongPacket{Seq: seq, Echo: echo}
}

func (p *PongPacket) Type() PacketType { return PONG }

func (p *PongPacket) encode(w io.Writer) error {
	if err := EncodeUint32(w, p.Seq); err != nil {
		return err
	}
	return EncodeUint64(w, p.Echo)
}

func (p *PongPacket) decode(r io.Reader, header FixedHeader) error {
	if header.Remaining != 12 {
		return ErrMalformedPacket
	}
	var err error
	if p.Seq, err = DecodeUint32(r); err != nil {
		return err
	}
	p.Echo, err = DecodeUint64(r)
	return err
}

// DisconnectPacket DISCONNECT：双向的断开通知。
//
//	客户端 -> 服务端：优雅关闭
//	服务端 -> 客户端：终止通知（会话被接管、停机、被踢下线）
type DisconnectPacket struct {
	ReasonCode ReasonCode
}

// NewDisconnectPacket 创建 DISCONNECT 包
func NewDisconnectPacket(reason ReasonCode) *DisconnectPacket {
	return &DisconnectPacket{ReasonCode: reason}
}

func (p *DisconnectPacket) Type() PacketType { return DISCONNECT }

func (p *DisconnectPacket) encode(w io.Writer) error {
	return WriteByte(w, byte(p.ReasonCode))
}

func (p *DisconnectPacket) decode(r io.Reader, _ FixedHeader) error {
	reason, err := ReadByte(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)
	return nil
}

package packet

import "io"

// BindPacket 是数据面连接的首包：绑定到控制面协商好的流。
//
// Ticket 由服务端生成，Challenge 由接受方生成；两端都必须同时回显两者，
// 服务端才能完成配对。它们都是一次性的，且只在 bind deadline 内有效。
type BindPacket struct {
	Version   uint8
	StreamID  uint64
	Role      Role
	Ticket    []byte
	Challenge []byte
}

func (p *BindPacket) Type() PacketType { return BIND }

func (p *BindPacket) encode(w io.Writer) error {
	if err := EncodeUint8(w, p.Version); err != nil {
		return err
	}
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	if err := EncodeUint8(w, uint8(p.Role)); err != nil {
		return err
	}
	if err := EncodeBinary(w, p.Ticket); err != nil {
		return err
	}
	return EncodeBinary(w, p.Challenge)
}

func (p *BindPacket) decode(r io.Reader) error {
	var err error
	if p.Version, err = DecodeUint8(r); err != nil {
		return err
	}
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	role, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.Role = Role(role)
	if p.Ticket, err = DecodeBinary(r); err != nil {
		return err
	}
	p.Challenge, err = DecodeBinary(r)
	return err
}

// BindOkPacket 是服务端对 BIND 的确认。
type BindOkPacket struct {
	StreamID   uint64
	Role       Role
	ReasonCode ReasonCode
	Message    string
}

func (p *BindOkPacket) Type() PacketType { return BIND_OK }

func (p *BindOkPacket) encode(w io.Writer) error {
	if err := EncodeUint64(w, p.StreamID); err != nil {
		return err
	}
	if err := EncodeUint8(w, uint8(p.Role)); err != nil {
		return err
	}
	if err := EncodeUint8(w, uint8(p.ReasonCode)); err != nil {
		return err
	}
	return EncodeString(w, p.Message)
}

func (p *BindOkPacket) decode(r io.Reader) error {
	var err error
	if p.StreamID, err = DecodeUint64(r); err != nil {
		return err
	}
	role, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.Role = Role(role)
	reason, err := DecodeUint8(r)
	if err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(reason)
	p.Message, err = DecodeString(r)
	return err
}

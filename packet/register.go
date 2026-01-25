package packet

import (
	"bytes"
	"io"
)

// RegisterPacket REGISTER 数据包
// 用于客户端向 core 注册可处理的 actions
type RegisterPacket struct {
	// 注册的 actions（精确匹配，不支持通配符）
	Actions []string

	// 属性
	Properties *RegisterProperties
}

// NewRegisterPacket 创建新的 REGISTER 包
func NewRegisterPacket(actions []string) *RegisterPacket {
	return &RegisterPacket{
		Actions:    actions,
		Properties: NewRegisterProperties(),
	}
}

func (p *RegisterPacket) Type() PacketType {
	return REGISTER
}

func (p *RegisterPacket) encode(w io.Writer) error {
	// Actions 数量 (2 bytes)
	if err := EncodeUint16(w, uint16(len(p.Actions))); err != nil {
		return err
	}

	// Actions 列表
	for _, action := range p.Actions {
		if err := EncodeString(w, action); err != nil {
			return err
		}
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewRegisterProperties()
	}

	return p.Properties.Encode(w)
}

func (p *RegisterPacket) decode(r io.Reader, header FixedHeader) error {
	// Actions 数量
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}

	// Actions 列表
	p.Actions = make([]string, count)
	for i := uint16(0); i < count; i++ {
		p.Actions[i], err = DecodeString(r)
		if err != nil {
			return err
		}
	}

	// 属性
	p.Properties = NewRegisterProperties()
	return p.Properties.Decode(r)
}

// RegackPacket REGACK 数据包
// core 对 REGISTER 的确认响应
type RegackPacket struct {
	// 注册结果（对应每个 action 的结果）
	Results []RegisterResult

	// 属性
	Properties *ReasonProperties
}

// RegisterResult 单个 action 的注册结果
type RegisterResult struct {
	Action     string
	ReasonCode ReasonCode
}

// NewRegackPacket 创建新的 REGACK 包
func NewRegackPacket(results []RegisterResult) *RegackPacket {
	return &RegackPacket{
		Results:    results,
		Properties: NewReasonProperties(),
	}
}

func (p *RegackPacket) Type() PacketType {
	return REGACK
}

func (p *RegackPacket) encode(w io.Writer) error {
	// 结果数量 (2 bytes)
	if err := EncodeUint16(w, uint16(len(p.Results))); err != nil {
		return err
	}

	// 结果列表
	for _, result := range p.Results {
		if err := EncodeString(w, result.Action); err != nil {
			return err
		}
		if err := WriteByte(w, byte(result.ReasonCode)); err != nil {
			return err
		}
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}

	return p.Properties.Encode(w)
}

func (p *RegackPacket) decode(r io.Reader, header FixedHeader) error {
	// 结果数量
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}

	// 结果列表
	p.Results = make([]RegisterResult, count)
	for i := uint16(0); i < count; i++ {
		p.Results[i].Action, err = DecodeString(r)
		if err != nil {
			return err
		}

		var code [1]byte
		if _, err := io.ReadFull(r, code[:]); err != nil {
			return err
		}
		p.Results[i].ReasonCode = ReasonCode(code[0])
	}

	// 属性
	p.Properties = NewReasonProperties()
	return p.Properties.Decode(r)
}

// UnregisterPacket UNREGISTER 数据包
// 用于客户端向 core 注销 actions
type UnregisterPacket struct {
	// 注销的 actions
	Actions []string

	// 属性
	Properties *ReasonProperties
}

// NewUnregisterPacket 创建新的 UNREGISTER 包
func NewUnregisterPacket(actions []string) *UnregisterPacket {
	return &UnregisterPacket{
		Actions:    actions,
		Properties: NewReasonProperties(),
	}
}

func (p *UnregisterPacket) Type() PacketType {
	return UNREGISTER
}

func (p *UnregisterPacket) encode(w io.Writer) error {
	// Actions 数量 (2 bytes)
	if err := EncodeUint16(w, uint16(len(p.Actions))); err != nil {
		return err
	}

	// Actions 列表
	for _, action := range p.Actions {
		if err := EncodeString(w, action); err != nil {
			return err
		}
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}

	return p.Properties.Encode(w)
}

func (p *UnregisterPacket) decode(r io.Reader, header FixedHeader) error {
	// Actions 数量
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}

	// Actions 列表
	p.Actions = make([]string, count)
	for i := uint16(0); i < count; i++ {
		p.Actions[i], err = DecodeString(r)
		if err != nil {
			return err
		}
	}

	// 属性
	p.Properties = NewReasonProperties()
	return p.Properties.Decode(r)
}

// UnregackPacket UNREGACK 数据包
// core 对 UNREGISTER 的确认响应
type UnregackPacket struct {
	// 注销结果（对应每个 action 的结果）
	Results []RegisterResult

	// 属性
	Properties *ReasonProperties
}

// NewUnregackPacket 创建新的 UNREGACK 包
func NewUnregackPacket(results []RegisterResult) *UnregackPacket {
	return &UnregackPacket{
		Results:    results,
		Properties: NewReasonProperties(),
	}
}

func (p *UnregackPacket) Type() PacketType {
	return UNREGACK
}

func (p *UnregackPacket) encode(w io.Writer) error {
	// 结果数量 (2 bytes)
	if err := EncodeUint16(w, uint16(len(p.Results))); err != nil {
		return err
	}

	// 结果列表
	for _, result := range p.Results {
		if err := EncodeString(w, result.Action); err != nil {
			return err
		}
		if err := WriteByte(w, byte(result.ReasonCode)); err != nil {
			return err
		}
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewReasonProperties()
	}

	return p.Properties.Encode(w)
}

func (p *UnregackPacket) decode(r io.Reader, header FixedHeader) error {
	// 结果数量
	count, err := DecodeUint16(r)
	if err != nil {
		return err
	}

	// 结果列表
	p.Results = make([]RegisterResult, count)
	for i := uint16(0); i < count; i++ {
		p.Results[i].Action, err = DecodeString(r)
		if err != nil {
			return err
		}

		var code [1]byte
		if _, err := io.ReadFull(r, code[:]); err != nil {
			return err
		}
		p.Results[i].ReasonCode = ReasonCode(code[0])
	}

	// 属性
	p.Properties = NewReasonProperties()
	return p.Properties.Decode(r)
}

// RegisterProperties REGISTER 专用属性
// 使用位掩码优化编码，减少字节开销
type RegisterProperties struct {
	// 并发处理能力（建议值）
	Concurrency *uint16

	// 用户自定义属性
	UserProperties map[string]string
}

// RegisterProperties 位掩码常量
const (
	regPropConcurrency    uint16 = 1 << 0 // bit 0
	regPropUserProperties uint16 = 1 << 1 // bit 1
)

// NewRegisterProperties 创建新的注册属性
func NewRegisterProperties() *RegisterProperties {
	return &RegisterProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码注册属性（使用位掩码优化）
func (p *RegisterProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.Concurrency != nil {
		flags |= regPropConcurrency
	}
	if len(p.UserProperties) > 0 {
		flags |= regPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	EncodeUint16(&buf, flags)

	// 按顺序写入存在的属性
	if flags&regPropConcurrency != 0 {
		EncodeUint16(&buf, *p.Concurrency)
	}
	if flags&regPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	// 写入属性长度和数据
	propLen := uint32(buf.Len())
	if propLen > MaxPacketSize {
		return ErrMalformedPacket
	}

	if err := EncodeUint32(w, propLen); err != nil {
		return err
	}
	if propLen > 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	return nil
}

// Decode 解码注册属性（使用位掩码优化）
func (p *RegisterProperties) Decode(r io.Reader) error {
	propLen, err := DecodeUint32(r)
	if err != nil {
		return err
	}

	if propLen == 0 {
		return nil
	}

	if propLen > MaxPacketSize {
		return ErrMalformedPacket
	}

	data := make([]byte, propLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}

	buf := bytes.NewReader(data)

	// 读取位掩码
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags&regPropConcurrency != 0 {
		v, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		p.Concurrency = &v
	}
	if flags&regPropUserProperties != 0 {
		count, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		if p.UserProperties == nil {
			p.UserProperties = make(map[string]string)
		}
		for range count {
			key, err := DecodeString(buf)
			if err != nil {
				return err
			}
			value, err := DecodeString(buf)
			if err != nil {
				return err
			}
			p.UserProperties[key] = value
		}
	}

	return nil
}

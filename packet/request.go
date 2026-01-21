package packet

import (
	"bytes"
	"io"
)

// RequestPacket REQUEST 数据包
type RequestPacket struct {
	// 请求ID（由调用端分配，用于匹配响应）
	RequestID uint32

	// Action/Method 名称（精确匹配，不支持通配符）
	Action string

	// 目标客户端ID（可选，为空时由core选择）
	TargetClientID string

	// 来源客户端ID（由core填充）
	SourceClientID string

	// 属性
	Properties *RequestProperties

	// 载荷（如果 Compression != None，则为压缩后的数据）
	Payload []byte
}

// NewRequestPacket 创建新的 REQUEST 包
func NewRequestPacket(requestID uint32, action string, payload []byte) *RequestPacket {
	return &RequestPacket{
		RequestID:  requestID,
		Action:     action,
		Payload:    payload,
		Properties: NewRequestProperties(),
	}
}

func (p *RequestPacket) Type() PacketType {
	return REQUEST
}

func (p *RequestPacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 请求ID (8 bytes)
	if err := EncodeUint32(&buf, p.RequestID); err != nil {
		return err
	}

	// Action 名称
	if err := EncodeString(&buf, p.Action); err != nil {
		return err
	}

	// 目标客户端ID
	if err := EncodeString(&buf, p.TargetClientID); err != nil {
		return err
	}

	// 来源客户端ID（由core填充）
	if err := EncodeString(&buf, p.SourceClientID); err != nil {
		return err
	}

	// 属性
	if p.Properties == nil {
		p.Properties = NewRequestProperties()
	}
	if err := p.Properties.Encode(&buf); err != nil {
		return err
	}

	// 载荷
	buf.Write(p.Payload)

	// 固定头部
	header := FixedHeader{
		Type:      REQUEST,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *RequestPacket) Decode(r io.Reader, header FixedHeader) error {
	// 请求ID
	var err error
	p.RequestID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	// Action 名称
	p.Action, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 目标客户端ID
	p.TargetClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 来源客户端ID
	p.SourceClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 属性
	p.Properties = NewRequestProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 载荷（读取剩余所有数据）
	if br, ok := r.(*bytes.Reader); ok {
		p.Payload = make([]byte, br.Len())
		_, err = io.ReadFull(r, p.Payload)
	} else {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		if err != nil && err != io.EOF {
			return err
		}
		p.Payload = buf.Bytes()
	}

	return err
}

// ResponsePacket RESPONSE 数据包
type ResponsePacket struct {
	// 对应的请求ID
	RequestID uint32

	// 目标客户端ID（core 根据此字段转发响应）
	TargetClientID string

	// 原因码（成功或各种错误）
	ReasonCode ReasonCode

	// 属性
	Properties *ResponseProperties

	// 载荷（响应数据，如果 Compression != None，则为压缩后的数据）
	Payload []byte
}

// NewResponsePacket 创建新的 RESPONSE 包
func NewResponsePacket(requestID uint32, targetClientID string, code ReasonCode, payload []byte) *ResponsePacket {
	return &ResponsePacket{
		RequestID:      requestID,
		TargetClientID: targetClientID,
		ReasonCode:     code,
		Payload:        payload,
		Properties:     NewResponseProperties(),
	}
}

func (p *ResponsePacket) Type() PacketType {
	return RESPONSE
}

func (p *ResponsePacket) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 请求ID (8 bytes)
	if err := EncodeUint32(&buf, p.RequestID); err != nil {
		return err
	}

	// 目标客户端ID
	if err := EncodeString(&buf, p.TargetClientID); err != nil {
		return err
	}

	// 原因码 (1 byte)
	buf.WriteByte(byte(p.ReasonCode))

	// 属性
	if p.Properties == nil {
		p.Properties = NewResponseProperties()
	}
	if err := p.Properties.Encode(&buf); err != nil {
		return err
	}

	// 载荷
	buf.Write(p.Payload)

	// 固定头部
	header := FixedHeader{
		Type:      RESPONSE,
		Remaining: uint32(buf.Len()),
	}
	if err := header.Encode(w); err != nil {
		return err
	}

	_, err := w.Write(buf.Bytes())
	return err
}

func (p *ResponsePacket) Decode(r io.Reader, header FixedHeader) error {
	// 请求ID
	var err error
	p.RequestID, err = DecodeUint32(r)
	if err != nil {
		return err
	}

	// 目标客户端ID
	p.TargetClientID, err = DecodeString(r)
	if err != nil {
		return err
	}

	// 原因码 (1 byte)
	var code [1]byte
	if _, err := io.ReadFull(r, code[:]); err != nil {
		return err
	}
	p.ReasonCode = ReasonCode(code[0])

	// 属性
	p.Properties = NewResponseProperties()
	if err := p.Properties.Decode(r); err != nil {
		return err
	}

	// 载荷（读取剩余所有数据）
	if br, ok := r.(*bytes.Reader); ok {
		p.Payload = make([]byte, br.Len())
		_, err = io.ReadFull(r, p.Payload)
	} else {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		if err != nil && err != io.EOF {
			return err
		}
		p.Payload = buf.Bytes()
	}

	return err
}

// RequestProperties REQUEST 专用属性
// 使用位掩码优化编码，减少字节开销
type RequestProperties struct {
	// 超时时间,相对时间，由调用方决定如何使用
	Timeout uint32

	// 追踪 ID
	TraceID string

	// 内容类型
	ContentType string

	// 用户自定义属性
	UserProperties map[string]string
}

// RequestProperties 位掩码常量
const (
	reqPropTimeout        uint16 = 1 << 0 // bit 0
	reqPropTraceID        uint16 = 1 << 1 // bit 1
	reqPropContentType    uint16 = 1 << 2 // bit 2
	reqPropUserProperties uint16 = 1 << 3 // bit 3
)

// NewRequestProperties 创建新的请求属性
func NewRequestProperties() *RequestProperties {
	return &RequestProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码请求属性（使用位掩码优化）
func (p *RequestProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.Timeout > 0 {
		flags |= reqPropTimeout
	}
	if p.TraceID != "" {
		flags |= reqPropTraceID
	}
	if p.ContentType != "" {
		flags |= reqPropContentType
	}
	if len(p.UserProperties) > 0 {
		flags |= reqPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&reqPropTimeout != 0 {
		EncodeUint32(&buf, p.Timeout)
	}
	if flags&reqPropTraceID != 0 {
		EncodeString(&buf, p.TraceID)
	}
	if flags&reqPropContentType != 0 {
		EncodeString(&buf, p.ContentType)
	}
	if flags&reqPropUserProperties != 0 {
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
		return ErrPacketTooLarge
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

// Decode 解码请求属性（使用位掩码优化）
func (p *RequestProperties) Decode(r io.Reader) error {
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
	if flags&reqPropTimeout != 0 {
		p.Timeout, err = DecodeUint32(buf)
		if err != nil {
			return err
		}
	}
	if flags&reqPropTraceID != 0 {
		p.TraceID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&reqPropContentType != 0 {
		p.ContentType, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&reqPropUserProperties != 0 {
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

// ResponseProperties RESPONSE 专用属性
// 使用位掩码优化编码，减少字节开销
type ResponseProperties struct {
	// 原因字符串
	ReasonString string

	// 追踪 ID
	TraceID string

	// 内容类型
	ContentType string

	// 用户自定义属性
	UserProperties map[string]string
}

// ResponseProperties 位掩码常量
const (
	respPropReasonString   uint16 = 1 << 0 // bit 0
	respPropTraceID        uint16 = 1 << 1 // bit 1
	respPropContentType    uint16 = 1 << 2 // bit 2
	respPropUserProperties uint16 = 1 << 3 // bit 3
)

// NewResponseProperties 创建新的响应属性
func NewResponseProperties() *ResponseProperties {
	return &ResponseProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码响应属性（使用位掩码优化）
func (p *ResponseProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.ReasonString != "" {
		flags |= respPropReasonString
	}
	if p.TraceID != "" {
		flags |= respPropTraceID
	}
	if p.ContentType != "" {
		flags |= respPropContentType
	}
	if len(p.UserProperties) > 0 {
		flags |= respPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&respPropReasonString != 0 {
		EncodeString(&buf, p.ReasonString)
	}
	if flags&respPropTraceID != 0 {
		EncodeString(&buf, p.TraceID)
	}
	if flags&respPropContentType != 0 {
		EncodeString(&buf, p.ContentType)
	}
	if flags&respPropUserProperties != 0 {
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
		return ErrPacketTooLarge
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

// Decode 解码响应属性（使用位掩码优化）
func (p *ResponseProperties) Decode(r io.Reader) error {
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
	if flags&respPropReasonString != 0 {
		p.ReasonString, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&respPropTraceID != 0 {
		p.TraceID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&respPropContentType != 0 {
		p.ContentType, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&respPropUserProperties != 0 {
		count, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		if p.UserProperties == nil {
			p.UserProperties = make(map[string]string)
		}
		for i := uint16(0); i < count; i++ {
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

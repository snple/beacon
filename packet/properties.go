package packet

import (
	"bytes"
	"io"
)

// ConnectProperties CONNECT 专用属性（客户端发送）
// 使用位掩码优化编码，减少字节开销
type ConnectProperties struct {
	// 会话管理
	SessionExpiry *uint32 // 会话过期时间（秒）

	// 认证
	AuthMethod string // 认证方法
	AuthData   []byte // 认证数据

	// 流量控制
	MaxPacketSize *uint32 // 最大包大小
	ReceiveWindow *uint16 // 初始接收窗口（消息数量）

	TraceID string // 追踪 ID

	// 用户属性
	UserProperties map[string]string
}

// ConnectProperties 位掩码常量
const (
	connPropSessionExpiry  uint16 = 1 << 0 // bit 0
	connPropAuthMethod     uint16 = 1 << 1 // bit 1
	connPropAuthData       uint16 = 1 << 2 // bit 2
	connPropMaxPacketSize  uint16 = 1 << 3 // bit 3
	connPropReceiveWindow  uint16 = 1 << 4 // bit 4
	connPropTraceID        uint16 = 1 << 5 // bit 5
	connPropUserProperties uint16 = 1 << 6 // bit 6
)

// NewConnectProperties 创建新的连接属性
func NewConnectProperties() *ConnectProperties {
	return &ConnectProperties{
		UserProperties: make(map[string]string),
	}
}

// ConnackProperties CONNACK 专用属性（服务器响应）
// 使用位掩码优化编码，减少字节开销
type ConnackProperties struct {
	// 会话管理
	SessionExpiry *uint32 // 会话过期时间（秒）

	// 服务器分配
	ClientID  string  // 服务器分配的 Client ID
	KeepAlive *uint16 // 服务器要求的心跳间隔

	// 流量控制
	MaxPacketSize *uint32 // 最大包大小
	ReceiveWindow *uint16 // 初始接收窗口（消息数量）

	TraceID string // 追踪 ID

	// 用户属性
	UserProperties map[string]string
}

// ConnackProperties 位掩码常量
const (
	connackPropSessionExpiry  uint16 = 1 << 0 // bit 0
	connackPropClientID       uint16 = 1 << 1 // bit 1
	connackPropKeepAlive      uint16 = 1 << 2 // bit 2
	connackPropMaxPacketSize  uint16 = 1 << 3 // bit 3
	connackPropReceiveWindow  uint16 = 1 << 4 // bit 4
	connackPropTraceID        uint16 = 1 << 5 // bit 5
	connackPropUserProperties uint16 = 1 << 6 // bit 6
)

// NewConnackProperties 创建新的连接响应属性
func NewConnackProperties() *ConnackProperties {
	return &ConnackProperties{
		UserProperties: make(map[string]string),
	}
}

// AuthProperties AUTH 专用属性
// 简化的属性结构，只包含认证相关字段
type AuthProperties struct {
	AuthMethod     string            // 认证方法
	AuthData       []byte            // 认证数据
	UserProperties map[string]string // 用户属性
}

// AuthProperties 位掩码常量
const (
	authPropAuthMethod     uint8 = 1 << 0 // bit 0
	authPropAuthData       uint8 = 1 << 1 // bit 1
	authPropUserProperties uint8 = 1 << 2 // bit 2
)

// NewAuthProperties 创建新的认证属性
func NewAuthProperties() *AuthProperties {
	return &AuthProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码认证属性
func (p *AuthProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint8
	if p.AuthMethod != "" {
		flags |= authPropAuthMethod
	}
	if len(p.AuthData) > 0 {
		flags |= authPropAuthData
	}
	if len(p.UserProperties) > 0 {
		flags |= authPropUserProperties
	}

	// 写入位掩码 (1 byte)
	buf.WriteByte(flags)

	// 按顺序写入存在的属性
	if flags&authPropAuthMethod != 0 {
		EncodeString(&buf, p.AuthMethod)
	}
	if flags&authPropAuthData != 0 {
		EncodeBinary(&buf, p.AuthData)
	}
	if flags&authPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	_, err := w.Write(buf.Bytes())
	return err
}

// Decode 解码认证属性
func (p *AuthProperties) Decode(r io.Reader) error {
	// 读取位掩码
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags[0]&authPropAuthMethod != 0 {
		var err error
		p.AuthMethod, err = DecodeString(r)
		if err != nil {
			return err
		}
	}

	if flags[0]&authPropAuthData != 0 {
		var err error
		p.AuthData, err = DecodeBinary(r)
		if err != nil {
			return err
		}
	}

	if flags[0]&authPropUserProperties != 0 {
		count, err := DecodeUint16(r)
		if err != nil {
			return err
		}

		p.UserProperties = make(map[string]string, count)
		for range count {
			key, err := DecodeString(r)
			if err != nil {
				return err
			}
			value, err := DecodeString(r)
			if err != nil {
				return err
			}
			p.UserProperties[key] = value
		}
	}

	return nil
}

// Encode 编码连接属性（使用位掩码优化）
func (p *ConnectProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.SessionExpiry != nil {
		flags |= connPropSessionExpiry
	}
	if p.AuthMethod != "" {
		flags |= connPropAuthMethod
	}
	if len(p.AuthData) > 0 {
		flags |= connPropAuthData
	}
	if p.MaxPacketSize != nil {
		flags |= connPropMaxPacketSize
	}
	if p.ReceiveWindow != nil {
		flags |= connPropReceiveWindow
	}
	if p.TraceID != "" {
		flags |= connPropTraceID
	}
	if len(p.UserProperties) > 0 {
		flags |= connPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&connPropSessionExpiry != 0 {
		EncodeUint32(&buf, *p.SessionExpiry)
	}
	if flags&connPropAuthMethod != 0 {
		EncodeString(&buf, p.AuthMethod)
	}
	if flags&connPropAuthData != 0 {
		EncodeBinary(&buf, p.AuthData)
	}
	if flags&connPropMaxPacketSize != 0 {
		EncodeUint32(&buf, *p.MaxPacketSize)
	}
	if flags&connPropReceiveWindow != 0 {
		EncodeUint16(&buf, *p.ReceiveWindow)
	}
	if flags&connPropTraceID != 0 {
		EncodeString(&buf, p.TraceID)
	}
	if flags&connPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	// 写入属性长度和数据
	propLen := uint32(buf.Len())
	if err := EncodeVariableInt(w, propLen); err != nil {
		return err
	}
	if propLen > 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	return nil
}

// Decode 解码连接属性（使用位掩码优化）
func (p *ConnectProperties) Decode(r io.Reader) error {
	propLen, err := DecodeVariableInt(r)
	if err != nil {
		return err
	}

	if propLen == 0 {
		return nil
	}

	data := make([]byte, propLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}

	buf := bytes.NewReader(data)

	// 读取位掩码 (2 bytes)
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags&connPropSessionExpiry != 0 {
		v, err := DecodeUint32(buf)
		if err != nil {
			return err
		}
		p.SessionExpiry = &v
	}
	if flags&connPropAuthMethod != 0 {
		p.AuthMethod, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&connPropAuthData != 0 {
		p.AuthData, err = DecodeBinary(buf)
		if err != nil {
			return err
		}
	}
	if flags&connPropMaxPacketSize != 0 {
		v, err := DecodeUint32(buf)
		if err != nil {
			return err
		}
		p.MaxPacketSize = &v
	}
	if flags&connPropReceiveWindow != 0 {
		v, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		p.ReceiveWindow = &v
	}
	if flags&connPropTraceID != 0 {
		p.TraceID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&connPropUserProperties != 0 {
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

// Encode 编码连接响应属性（使用位掩码优化）
func (p *ConnackProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.SessionExpiry != nil {
		flags |= connackPropSessionExpiry
	}
	if p.ClientID != "" {
		flags |= connackPropClientID
	}
	if p.KeepAlive != nil {
		flags |= connackPropKeepAlive
	}
	if p.MaxPacketSize != nil {
		flags |= connackPropMaxPacketSize
	}
	if p.ReceiveWindow != nil {
		flags |= connackPropReceiveWindow
	}
	if p.TraceID != "" {
		flags |= connackPropTraceID
	}
	if len(p.UserProperties) > 0 {
		flags |= connackPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&connackPropSessionExpiry != 0 {
		EncodeUint32(&buf, *p.SessionExpiry)
	}
	if flags&connackPropClientID != 0 {
		EncodeString(&buf, p.ClientID)
	}
	if flags&connackPropKeepAlive != 0 {
		EncodeUint16(&buf, *p.KeepAlive)
	}
	if flags&connackPropMaxPacketSize != 0 {
		EncodeUint32(&buf, *p.MaxPacketSize)
	}
	if flags&connackPropReceiveWindow != 0 {
		EncodeUint16(&buf, *p.ReceiveWindow)
	}
	if flags&connackPropTraceID != 0 {
		EncodeString(&buf, p.TraceID)
	}
	if flags&connackPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	// 写入属性长度和数据
	propLen := uint32(buf.Len())
	if err := EncodeVariableInt(w, propLen); err != nil {
		return err
	}
	if propLen > 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	return nil
}

// Decode 解码连接响应属性（使用位掩码优化）
func (p *ConnackProperties) Decode(r io.Reader) error {
	propLen, err := DecodeVariableInt(r)
	if err != nil {
		return err
	}

	if propLen == 0 {
		return nil
	}

	data := make([]byte, propLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}

	buf := bytes.NewReader(data)

	// 读取位掩码 (2 bytes)
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags&connackPropSessionExpiry != 0 {
		v, err := DecodeUint32(buf)
		if err != nil {
			return err
		}
		p.SessionExpiry = &v
	}
	if flags&connackPropClientID != 0 {
		p.ClientID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&connackPropKeepAlive != 0 {
		v, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		p.KeepAlive = &v
	}
	if flags&connackPropMaxPacketSize != 0 {
		v, err := DecodeUint32(buf)
		if err != nil {
			return err
		}
		p.MaxPacketSize = &v
	}
	if flags&connackPropReceiveWindow != 0 {
		v, err := DecodeUint16(buf)
		if err != nil {
			return err
		}
		p.ReceiveWindow = &v
	}
	if flags&connackPropTraceID != 0 {
		p.TraceID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&connackPropUserProperties != 0 {
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

// PublishProperties PUBLISH 消息专用属性
// 使用位掩码优化编码，减少字节开销
type PublishProperties struct {
	// 过期时间 (Unix 秒级时间戳，0 会使用 core 默认值)
	ExpiryTime int64 `nson:"exp"` // 过期时间

	// 消息格式
	ContentType string `nson:"ctype"` // MIME 类型

	Priority *Priority `nson:"prio"`  // 消息优先级
	TraceID  string    `nson:"trace"` // 分布式追踪 ID

	// 请求-响应模式
	TargetClientID  string `nson:"tgt"`  // 目标客户端ID，用于点对点消息
	SourceClientID  string `nson:"src"`  // 来源客户端ID，标识发送者
	ResponseTopic   string `nson:"resp"` // 响应主题，接收方可往此主题发回响应
	CorrelationData []byte `nson:"corr"` // 关联数据，用于匹配请求和响应

	// 用户属性
	UserProperties map[string]string `nson:"uprops"` // 用户属性
}

// PublishProperties 位掩码常量
const (
	pubPropExpiryTime      uint16 = 1 << 0 // bit 0
	pubPropContentType     uint16 = 1 << 1 // bit 1
	pubPropPriority        uint16 = 1 << 2 // bit 2
	pubPropTraceID         uint16 = 1 << 3 // bit 3
	pubPropTargetClientID  uint16 = 1 << 4 // bit 4
	pubPropSourceClientID  uint16 = 1 << 5 // bit 5
	pubPropResponseTopic   uint16 = 1 << 6 // bit 6
	pubPropCorrelationData uint16 = 1 << 7 // bit 7
	pubPropUserProperties  uint16 = 1 << 8 // bit 8
)

// NewPublishProperties 创建新的发布属性
func NewPublishProperties() *PublishProperties {
	return &PublishProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码发布属性（使用位掩码优化）
func (p *PublishProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.ExpiryTime > 0 {
		flags |= pubPropExpiryTime
	}
	if p.ContentType != "" {
		flags |= pubPropContentType
	}
	if p.Priority != nil {
		flags |= pubPropPriority
	}
	if p.TraceID != "" {
		flags |= pubPropTraceID
	}
	if p.TargetClientID != "" {
		flags |= pubPropTargetClientID
	}
	if p.SourceClientID != "" {
		flags |= pubPropSourceClientID
	}
	if p.ResponseTopic != "" {
		flags |= pubPropResponseTopic
	}
	if len(p.CorrelationData) > 0 {
		flags |= pubPropCorrelationData
	}
	if len(p.UserProperties) > 0 {
		flags |= pubPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&pubPropExpiryTime != 0 {
		EncodeInt64(&buf, p.ExpiryTime)
	}
	if flags&pubPropContentType != 0 {
		EncodeString(&buf, p.ContentType)
	}
	if flags&pubPropPriority != 0 {
		buf.WriteByte(byte(*p.Priority))
	}
	if flags&pubPropTraceID != 0 {
		EncodeString(&buf, p.TraceID)
	}
	if flags&pubPropTargetClientID != 0 {
		EncodeString(&buf, p.TargetClientID)
	}
	if flags&pubPropSourceClientID != 0 {
		EncodeString(&buf, p.SourceClientID)
	}
	if flags&pubPropResponseTopic != 0 {
		EncodeString(&buf, p.ResponseTopic)
	}
	if flags&pubPropCorrelationData != 0 {
		EncodeBinary(&buf, p.CorrelationData)
	}
	if flags&pubPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	// 写入属性长度和数据
	propLen := uint32(buf.Len())
	if err := EncodeVariableInt(w, propLen); err != nil {
		return err
	}
	if propLen > 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	return nil
}

// Decode 解码发布属性（使用位掩码优化）
func (p *PublishProperties) Decode(r io.Reader) error {
	propLen, err := DecodeVariableInt(r)
	if err != nil {
		return err
	}

	if propLen == 0 {
		return nil
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
	if flags&pubPropExpiryTime != 0 {
		p.ExpiryTime, err = DecodeInt64(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropContentType != 0 {
		p.ContentType, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropPriority != 0 {
		b, err := buf.ReadByte()
		if err != nil {
			return err
		}
		pri := Priority(b)
		p.Priority = &pri
	}
	if flags&pubPropTraceID != 0 {
		p.TraceID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropTargetClientID != 0 {
		p.TargetClientID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropSourceClientID != 0 {
		p.SourceClientID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropResponseTopic != 0 {
		p.ResponseTopic, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropCorrelationData != 0 {
		p.CorrelationData, err = DecodeBinary(buf)
		if err != nil {
			return err
		}
	}
	if flags&pubPropUserProperties != 0 {
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

// ReasonProperties 通用响应属性（用于 PUBACK、DISCONNECT 等）
// 使用位掩码优化编码，减少字节开销
type ReasonProperties struct {
	ReasonString   string
	UserProperties map[string]string
}

// ReasonProperties 位掩码常量 (使用 uint16 便于未来扩展)
const (
	reasonPropReasonString   uint16 = 1 << 0 // bit 0
	reasonPropUserProperties uint16 = 1 << 1 // bit 1
)

// NewReasonProperties 创建新的响应属性
func NewReasonProperties() *ReasonProperties {
	return &ReasonProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码响应属性（使用位掩码优化）
func (p *ReasonProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.ReasonString != "" {
		flags |= reasonPropReasonString
	}
	if len(p.UserProperties) > 0 {
		flags |= reasonPropUserProperties
	}

	// 写入位掩码 (2 bytes)
	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	// 按顺序写入存在的属性
	if flags&reasonPropReasonString != 0 {
		EncodeString(&buf, p.ReasonString)
	}
	if flags&reasonPropUserProperties != 0 {
		// 写入用户属性数量
		EncodeUint16(&buf, uint16(len(p.UserProperties)))
		for k, v := range p.UserProperties {
			EncodeString(&buf, k)
			EncodeString(&buf, v)
		}
	}

	// 写入属性长度和数据
	propLen := uint32(buf.Len())
	if err := EncodeVariableInt(w, propLen); err != nil {
		return err
	}
	if propLen > 0 {
		_, err := w.Write(buf.Bytes())
		return err
	}
	return nil
}

// Decode 解码响应属性（使用位掩码优化）
func (p *ReasonProperties) Decode(r io.Reader) error {
	propLen, err := DecodeVariableInt(r)
	if err != nil {
		return err
	}

	if propLen == 0 {
		return nil
	}

	data := make([]byte, propLen)
	if _, err := io.ReadFull(r, data); err != nil {
		return err
	}

	buf := bytes.NewReader(data)

	// 读取位掩码 (2 bytes)
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags&reasonPropReasonString != 0 {
		p.ReasonString, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&reasonPropUserProperties != 0 {
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

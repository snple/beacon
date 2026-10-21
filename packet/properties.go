package packet

import (
	"bytes"
	"fmt"
	"io"
	"sort"
)

// maxUserProperties 用户属性条数上限（数量以 uint16 编码）。
const maxUserProperties = 65535

// encodeUserProperties 按 key 排序后写入用户属性，保证相同内容产生确定的字节序列
// （map 遍历顺序随机，会影响签名、去重与测试复现）。同时传播编码错误，
// 避免超长字符串导致静默产出畸形包。
func encodeUserProperties(buf *bytes.Buffer, props map[string]string) error {
	if len(props) > maxUserProperties {
		return fmt.Errorf("too many user properties: %d (max %d)", len(props), maxUserProperties)
	}
	if err := EncodeUint16(buf, uint16(len(props))); err != nil {
		return err
	}

	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if err := EncodeString(buf, k); err != nil {
			return fmt.Errorf("user property key: %w", err)
		}
		if err := EncodeString(buf, props[k]); err != nil {
			return fmt.Errorf("user property value: %w", err)
		}
	}
	return nil
}

// writePropertyBlob 写入「长度前缀 + 属性字节」。
func writePropertyBlob(w io.Writer, buf *bytes.Buffer) error {
	propLen := uint32(buf.Len())
	if propLen > MaxPacketSize {
		return ErrMalformedPacket
	}
	if err := EncodeUint32(w, propLen); err != nil {
		return err
	}
	if propLen == 0 {
		return nil
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// ConnectProperties CONNECT 专用属性（客户端发送）
// 使用位掩码优化编码，减少字节开销
type ConnectProperties struct {
	// 认证
	AuthMethod string // 认证方法
	AuthData   []byte // 认证数据

	// 流量控制
	MaxPacketSize uint32 // 最大包大小
	ReceiveWindow uint16 // 初始接收窗口（消息数量）

	TraceID string // 追踪 ID

	// 用户属性
	UserProperties map[string]string
}

// ConnectProperties 位掩码常量
const (
	connPropAuthMethod     uint16 = 1 << 0 // bit 0
	connPropAuthData       uint16 = 1 << 1 // bit 1
	connPropMaxPacketSize  uint16 = 1 << 2 // bit 2
	connPropReceiveWindow  uint16 = 1 << 3 // bit 3
	connPropTraceID        uint16 = 1 << 4 // bit 4
	connPropUserProperties uint16 = 1 << 5 // bit 5
)

// NewConnectProperties 创建新的连接属性
func NewConnectProperties() *ConnectProperties {
	return &ConnectProperties{
		UserProperties: make(map[string]string),
	}
}

// Encode 编码连接属性（使用位掩码优化）
func (p *ConnectProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.AuthMethod != "" {
		flags |= connPropAuthMethod
	}
	if len(p.AuthData) > 0 {
		flags |= connPropAuthData
	}
	if p.MaxPacketSize > 0 {
		flags |= connPropMaxPacketSize
	}
	if p.ReceiveWindow > 0 {
		flags |= connPropReceiveWindow
	}
	if p.TraceID != "" {
		flags |= connPropTraceID
	}
	if len(p.UserProperties) > 0 {
		flags |= connPropUserProperties
	}

	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	if flags&connPropAuthMethod != 0 {
		if err := EncodeString(&buf, p.AuthMethod); err != nil {
			return err
		}
	}
	if flags&connPropAuthData != 0 {
		if err := EncodeBinary(&buf, p.AuthData); err != nil {
			return err
		}
	}
	if flags&connPropMaxPacketSize != 0 {
		if err := EncodeUint32(&buf, p.MaxPacketSize); err != nil {
			return err
		}
	}
	if flags&connPropReceiveWindow != 0 {
		if err := EncodeUint16(&buf, p.ReceiveWindow); err != nil {
			return err
		}
	}
	if flags&connPropTraceID != 0 {
		if err := EncodeString(&buf, p.TraceID); err != nil {
			return err
		}
	}
	if flags&connPropUserProperties != 0 {
		if err := encodeUserProperties(&buf, p.UserProperties); err != nil {
			return err
		}
	}

	return writePropertyBlob(w, &buf)
}

// Decode 解码连接属性（使用位掩码优化）
func (p *ConnectProperties) Decode(r io.Reader) error {
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

	// 读取位掩码 (2 bytes)
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
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
		p.MaxPacketSize, err = DecodeUint32(buf)
		if err != nil {
			return err
		}
	}
	if flags&connPropReceiveWindow != 0 {
		p.ReceiveWindow, err = DecodeUint16(buf)
		if err != nil {
			return err
		}
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

	if buf.Len() > 0 {
		return ErrMalformedPacket
	}

	return nil
}

// ConnackProperties CONNACK 专用属性（服务器响应）
// 使用位掩码优化编码，减少字节开销
type ConnackProperties struct {
	// 服务器分配
	ClientID  string // 服务器分配的 Client ID
	KeepAlive uint16 // 服务器要求的心跳间隔

	// 会话管理
	SessionTimeout uint32 // 会话过期时间（秒）

	// 流量控制
	MaxPacketSize uint32 // 最大包大小
	ReceiveWindow uint16 // 初始接收窗口（消息数量）

	TraceID string // 追踪 ID

	// 用户属性
	UserProperties map[string]string
}

// ConnackProperties 位掩码常量
const (
	connackPropClientID       uint16 = 1 << 0 // bit 0
	connackPropKeepAlive      uint16 = 1 << 1 // bit 1
	connackPropSessionTimeout uint16 = 1 << 2 // bit 2
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

// Encode 编码连接响应属性（使用位掩码优化）
func (p *ConnackProperties) Encode(w io.Writer) error {
	var buf bytes.Buffer

	// 计算位掩码
	var flags uint16
	if p.ClientID != "" {
		flags |= connackPropClientID
	}
	if p.KeepAlive > 0 {
		flags |= connackPropKeepAlive
	}
	if p.SessionTimeout > 0 {
		flags |= connackPropSessionTimeout
	}
	if p.MaxPacketSize > 0 {
		flags |= connackPropMaxPacketSize
	}
	if p.ReceiveWindow > 0 {
		flags |= connackPropReceiveWindow
	}
	if p.TraceID != "" {
		flags |= connackPropTraceID
	}
	if len(p.UserProperties) > 0 {
		flags |= connackPropUserProperties
	}

	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	if flags&connackPropClientID != 0 {
		if err := EncodeString(&buf, p.ClientID); err != nil {
			return err
		}
	}
	if flags&connackPropKeepAlive != 0 {
		if err := EncodeUint16(&buf, p.KeepAlive); err != nil {
			return err
		}
	}
	if flags&connackPropSessionTimeout != 0 {
		if err := EncodeUint32(&buf, p.SessionTimeout); err != nil {
			return err
		}
	}
	if flags&connackPropMaxPacketSize != 0 {
		if err := EncodeUint32(&buf, p.MaxPacketSize); err != nil {
			return err
		}
	}
	if flags&connackPropReceiveWindow != 0 {
		if err := EncodeUint16(&buf, p.ReceiveWindow); err != nil {
			return err
		}
	}
	if flags&connackPropTraceID != 0 {
		if err := EncodeString(&buf, p.TraceID); err != nil {
			return err
		}
	}
	if flags&connackPropUserProperties != 0 {
		if err := encodeUserProperties(&buf, p.UserProperties); err != nil {
			return err
		}
	}

	return writePropertyBlob(w, &buf)
}

// Decode 解码连接响应属性（使用位掩码优化）
func (p *ConnackProperties) Decode(r io.Reader) error {
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

	// 读取位掩码 (2 bytes)
	flags, err := DecodeUint16(buf)
	if err != nil {
		return err
	}

	// 按顺序读取存在的属性
	if flags&connackPropClientID != 0 {
		p.ClientID, err = DecodeString(buf)
		if err != nil {
			return err
		}
	}
	if flags&connackPropKeepAlive != 0 {
		p.KeepAlive, err = DecodeUint16(buf)
		if err != nil {
			return err
		}
	}
	if flags&connackPropSessionTimeout != 0 {
		p.SessionTimeout, err = DecodeUint32(buf)
		if err != nil {
			return err
		}
	}

	if flags&connackPropMaxPacketSize != 0 {
		p.MaxPacketSize, err = DecodeUint32(buf)
		if err != nil {
			return err
		}
	}
	if flags&connackPropReceiveWindow != 0 {
		p.ReceiveWindow, err = DecodeUint16(buf)
		if err != nil {
			return err
		}
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

	if buf.Len() > 0 {
		return ErrMalformedPacket
	}

	return nil
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

	if err := WriteByte(&buf, flags); err != nil {
		return err
	}

	if flags&authPropAuthMethod != 0 {
		if err := EncodeString(&buf, p.AuthMethod); err != nil {
			return err
		}
	}
	if flags&authPropAuthData != 0 {
		if err := EncodeBinary(&buf, p.AuthData); err != nil {
			return err
		}
	}
	if flags&authPropUserProperties != 0 {
		if err := encodeUserProperties(&buf, p.UserProperties); err != nil {
			return err
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

// PublishProperties PUBLISH 消息专用属性
// 使用位掩码优化编码，减少字节开销
type PublishProperties struct {
	// 过期时间 (Unix 秒级时间戳，0 会使用 core 默认值)
	ExpiryTime int64 `nson:"exp"` // 过期时间

	// 消息格式
	ContentType string `nson:"ctype"` // MIME 类型

	TraceID string `nson:"trace"` // 分布式追踪 ID

	// 消息总线 / 请求-响应元数据
	TargetClientID  string `nson:"tgt"`  // 目标客户端ID，用于点对点消息
	SourceClientID  string `nson:"src"`  // 来源客户端ID，标识发送者
	ResponseTopic   string `nson:"resp"` // 响应主题，用于基于 PUBLISH 的请求-响应或异步回执
	CorrelationData []byte `nson:"corr"` // 关联数据，用于匹配请求与响应或任务回执

	// 用户属性
	UserProperties map[string]string `nson:"uprops"` // 用户属性
}

// PublishProperties 位掩码常量
const (
	pubPropExpiryTime      uint16 = 1 << 0 // bit 0
	pubPropContentType     uint16 = 1 << 1 // bit 1
	pubPropTraceID         uint16 = 1 << 2 // bit 2
	pubPropTargetClientID  uint16 = 1 << 3 // bit 3
	pubPropSourceClientID  uint16 = 1 << 4 // bit 4
	pubPropResponseTopic   uint16 = 1 << 5 // bit 5
	pubPropCorrelationData uint16 = 1 << 6 // bit 6
	pubPropUserProperties  uint16 = 1 << 7 // bit 7
)

// NewPublishProperties 创建新的发布属性
func NewPublishProperties() *PublishProperties {
	return &PublishProperties{
		UserProperties: make(map[string]string),
	}
}

// DeepCopy 深拷贝发布属性
func (p *PublishProperties) DeepCopy() *PublishProperties {
	if p == nil {
		return nil
	}

	cp := &PublishProperties{
		ExpiryTime:     p.ExpiryTime,
		ContentType:    p.ContentType,
		TraceID:        p.TraceID,
		TargetClientID: p.TargetClientID,
		SourceClientID: p.SourceClientID,
		ResponseTopic:  p.ResponseTopic,
	}

	if p.CorrelationData != nil {
		cp.CorrelationData = make([]byte, len(p.CorrelationData))
		copy(cp.CorrelationData, p.CorrelationData)
	}

	if p.UserProperties != nil {
		cp.UserProperties = make(map[string]string, len(p.UserProperties))
		for k, v := range p.UserProperties {
			cp.UserProperties[k] = v
		}
	}

	return cp
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

	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	if flags&pubPropExpiryTime != 0 {
		if err := EncodeInt64(&buf, p.ExpiryTime); err != nil {
			return err
		}
	}
	if flags&pubPropContentType != 0 {
		if err := EncodeString(&buf, p.ContentType); err != nil {
			return err
		}
	}
	if flags&pubPropTraceID != 0 {
		if err := EncodeString(&buf, p.TraceID); err != nil {
			return err
		}
	}
	if flags&pubPropTargetClientID != 0 {
		if err := EncodeString(&buf, p.TargetClientID); err != nil {
			return err
		}
	}
	if flags&pubPropSourceClientID != 0 {
		if err := EncodeString(&buf, p.SourceClientID); err != nil {
			return err
		}
	}
	if flags&pubPropResponseTopic != 0 {
		if err := EncodeString(&buf, p.ResponseTopic); err != nil {
			return err
		}
	}
	if flags&pubPropCorrelationData != 0 {
		if err := EncodeBinary(&buf, p.CorrelationData); err != nil {
			return err
		}
	}
	if flags&pubPropUserProperties != 0 {
		if err := encodeUserProperties(&buf, p.UserProperties); err != nil {
			return err
		}
	}

	return writePropertyBlob(w, &buf)
}

// Decode 解码发布属性（使用位掩码优化）
func (p *PublishProperties) Decode(r io.Reader) error {
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

	if buf.Len() > 0 {
		return ErrMalformedPacket
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

	if err := EncodeUint16(&buf, flags); err != nil {
		return err
	}

	if flags&reasonPropReasonString != 0 {
		if err := EncodeString(&buf, p.ReasonString); err != nil {
			return err
		}
	}
	if flags&reasonPropUserProperties != 0 {
		if err := encodeUserProperties(&buf, p.UserProperties); err != nil {
			return err
		}
	}

	return writePropertyBlob(w, &buf)
}

// Decode 解码响应属性（使用位掩码优化）
func (p *ReasonProperties) Decode(r io.Reader) error {
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

	if buf.Len() > 0 {
		return ErrMalformedPacket
	}

	return nil
}

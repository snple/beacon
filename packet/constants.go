package packet

// ProtocolVersion 协议版本
const ProtocolVersion uint8 = 1

// ProtocolName 协议名称
const ProtocolName = "MXTP"

// PacketType 数据包类型
type PacketType uint8

const (
	RESERVED    PacketType = 0
	CONNECT     PacketType = 1  // 客户端请求连接
	CONNACK     PacketType = 2  // 连接确认
	PUBLISH     PacketType = 3  // 发布消息
	PUBACK      PacketType = 4  // 发布确认 (QoS 1)
	SUBSCRIBE   PacketType = 5  // 订阅请求
	SUBACK      PacketType = 6  // 订阅确认
	UNSUBSCRIBE PacketType = 7  // 取消订阅
	UNSUBACK    PacketType = 8  // 取消订阅确认
	PING        PacketType = 9  // 心跳请求
	PONG        PacketType = 10 // 心跳响应
	DISCONNECT  PacketType = 11 // 断开连接
	AUTH        PacketType = 12 // 认证交换
	TRACE       PacketType = 13 // 消息追踪
	REQUEST     PacketType = 14 // 请求消息
	RESPONSE    PacketType = 15 // 响应消息
	REGISTER    PacketType = 16 // 注册 action/method
	REGACK      PacketType = 17 // 注册确认
	UNREGISTER  PacketType = 18 // 注销 action/method
	UNREGACK    PacketType = 19 // 注销确认
)

func (t PacketType) String() string {
	switch t {
	case CONNECT:
		return "CONNECT"
	case CONNACK:
		return "CONNACK"
	case PUBLISH:
		return "PUBLISH"
	case PUBACK:
		return "PUBACK"
	case SUBSCRIBE:
		return "SUBSCRIBE"
	case SUBACK:
		return "SUBACK"
	case UNSUBSCRIBE:
		return "UNSUBSCRIBE"
	case UNSUBACK:
		return "UNSUBACK"
	case PING:
		return "PING"
	case PONG:
		return "PONG"
	case DISCONNECT:
		return "DISCONNECT"
	case AUTH:
		return "AUTH"
	case TRACE:
		return "TRACE"
	case REQUEST:
		return "REQUEST"
	case RESPONSE:
		return "RESPONSE"
	case REGISTER:
		return "REGISTER"
	case REGACK:
		return "REGACK"
	case UNREGISTER:
		return "UNREGISTER"
	case UNREGACK:
		return "UNREGACK"
	default:
		return "UNKNOWN"
	}
}

// QoS 服务质量等级
// Pull 模式下只需要 QoS 0 和 QoS 1:
//   - QoS 0: 发送即忘 - Core 发送成功后立即清理
//   - QoS 1: 确认送达 - 客户端处理完成后发送 ACK，Core 收到 ACK 后清理
type QoS uint8

const (
	QoS0 QoS = 0 // 发送即忘 (fire and forget) - Core 发送后即清理
	QoS1 QoS = 1 // 确认送达 (acknowledged) - 客户端 ACK 后 Core 清理
)

func (q QoS) IsValid() bool {
	return q <= QoS1
}

func (q QoS) String() string {
	switch q {
	case QoS0:
		return "QoS 0"
	case QoS1:
		return "QoS 1"
	default:
		return "Unknown QoS"
	}
}

// Priority 消息优先级
type Priority uint8

const (
	PriorityLow      Priority = 0 // 低优先级
	PriorityNormal   Priority = 1 // 普通优先级
	PriorityHigh     Priority = 2 // 高优先级
	PriorityCritical Priority = 3 // 紧急优先级
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// ReasonCode 原因码
type ReasonCode uint8

const (
	// ========== 成功类 (0x00-0x0F) ==========
	ReasonSuccess            ReasonCode = 0x00 // 操作成功
	ReasonNormalDisconnect   ReasonCode = 0x01 // 正常断开连接
	ReasonGrantedQoS0        ReasonCode = 0x02 // 授予 QoS 0
	ReasonGrantedQoS1        ReasonCode = 0x03 // 授予 QoS 1
	ReasonDisconnectWithWill ReasonCode = 0x04 // 带遗嘱消息断开

	// ========== 订阅相关 (0x10-0x17) ==========
	ReasonNoMatchingSubscribers ReasonCode = 0x10 // 没有匹配的订阅者
	ReasonNoSubscriptionExisted ReasonCode = 0x11 // 订阅不存在

	// ========== 认证相关 (0x18-0x1F) ==========
	ReasonContinueAuth ReasonCode = 0x18 // 继续认证
	ReasonReAuth       ReasonCode = 0x19 // 重新认证

	// ========== 通用错误 (0x80-0x83) ==========
	ReasonUnspecifiedError    ReasonCode = 0x80 // 未指定错误
	ReasonMalformedPacket     ReasonCode = 0x81 // 格式错误的数据包
	ReasonProtocolError       ReasonCode = 0x82 // 协议错误
	ReasonImplementationError ReasonCode = 0x83 // 实现错误

	// ========== 连接/认证错误 (0x84-0x87) ==========
	ReasonUnsupportedProtocol     ReasonCode = 0x84 // 不支持的协议版本
	ReasonClientIDNotValid        ReasonCode = 0x85 // 客户端 ID 无效
	ReasonBadAuthMethodOrAuthData ReasonCode = 0x86 // 错误的认证方法或数据
	ReasonNotAuthorized           ReasonCode = 0x87 // 未授权

	// ========== 服务器状态错误 (0x88-0x8E) ==========
	ReasonServerUnavailable  ReasonCode = 0x88 // 服务器不可用
	ReasonServerBusy         ReasonCode = 0x89 // 服务器繁忙
	ReasonBanned             ReasonCode = 0x8A // 被禁止访问
	ReasonServerShuttingDown ReasonCode = 0x8B // 服务器关闭中
	ReasonKeepAliveTimeout   ReasonCode = 0x8D // 心跳超时
	ReasonSessionTakenOver   ReasonCode = 0x8E // 会话被接管

	// ========== 主题相关错误 (0x8F-0x90) ==========
	ReasonTopicFilterInvalid ReasonCode = 0x8F // 主题过滤器无效
	ReasonTopicNameInvalid   ReasonCode = 0x90 // 主题名称无效

	// ========== 数据包 ID 相关错误 (0x91-0x92) ==========
	ReasonPacketIDInUse    ReasonCode = 0x91 // 数据包 ID 使用中
	ReasonPacketIDNotFound ReasonCode = 0x92 // 数据包 ID 未找到

	// ========== 限制/配额相关错误 (0x93-0x99) ==========
	ReasonReceiveMaxExceeded     ReasonCode = 0x93 // 接收最大值超限
	ReasonMessageExpired         ReasonCode = 0x94 // 消息已过期
	ReasonPacketTooLarge         ReasonCode = 0x95 // 数据包过大
	ReasonMessageRateTooHigh     ReasonCode = 0x96 // 消息速率过高
	ReasonQuotaExceeded          ReasonCode = 0x97 // 配额超限
	ReasonConnectionRateExceeded ReasonCode = 0x98 // 连接速率超限
	ReasonMaxConnectTime         ReasonCode = 0x99 // 超过最大连接时间

	// ========== 功能支持相关错误 (0x9A-0x9E) ==========
	ReasonPayloadFormatInvalid  ReasonCode = 0x9A // 负载格式无效
	ReasonRetainNotSupported    ReasonCode = 0x9B // 不支持保留消息
	ReasonQoSNotSupported       ReasonCode = 0x9C // 不支持的 QoS 等级
	ReasonSharedSubNotSupported ReasonCode = 0x9D // 不支持共享订阅
	ReasonPriorityNotSupported  ReasonCode = 0x9E // 不支持优先级

	// ========== 请求/响应相关错误 (0xC0-0xCF) ==========
	ReasonActionNotFound     ReasonCode = 0xC0 // Action 不存在
	ReasonClientNotFound     ReasonCode = 0xC1 // 目标客户端不存在
	ReasonRequestTimeout     ReasonCode = 0xC2 // 请求超时
	ReasonNoAvailableHandler ReasonCode = 0xC3 // 没有可用的处理器
	ReasonRequestCancelled   ReasonCode = 0xC4 // 请求被取消
	ReasonDuplicateAction    ReasonCode = 0xC5 // Action 已被注册
	ReasonActionInvalid      ReasonCode = 0xC6 // Action 名称无效
	ReasonReceiveWindowFull  ReasonCode = 0xC7 // 接收窗口已满
)

func (r ReasonCode) String() string {
	switch r {
	// ========== 成功类 ==========
	case ReasonSuccess:
		return "Success"
	case ReasonNormalDisconnect:
		return "Normal disconnect"
	case ReasonGrantedQoS0:
		return "Granted QoS 0"
	case ReasonGrantedQoS1:
		return "Granted QoS 1"
	case ReasonDisconnectWithWill:
		return "Disconnect with will"

	// ========== 订阅相关 ==========
	case ReasonNoMatchingSubscribers:
		return "No matching subscribers"
	case ReasonNoSubscriptionExisted:
		return "No subscription existed"

	// ========== 认证相关 ==========
	case ReasonContinueAuth:
		return "Continue authentication"
	case ReasonReAuth:
		return "Re-authenticate"

	// ========== 通用错误 ==========
	case ReasonUnspecifiedError:
		return "Unspecified error"
	case ReasonMalformedPacket:
		return "Malformed packet"
	case ReasonProtocolError:
		return "Protocol error"
	case ReasonImplementationError:
		return "Implementation error"

	// ========== 连接/认证错误 ==========
	case ReasonUnsupportedProtocol:
		return "Unsupported protocol"
	case ReasonClientIDNotValid:
		return "Client ID not valid"
	case ReasonBadAuthMethodOrAuthData:
		return "Bad authentication method or data"
	case ReasonNotAuthorized:
		return "Not authorized"

	// ========== 服务器状态错误 ==========
	case ReasonServerUnavailable:
		return "Server unavailable"
	case ReasonServerBusy:
		return "Server busy"
	case ReasonBanned:
		return "Banned"
	case ReasonServerShuttingDown:
		return "Server shutting down"
	case ReasonKeepAliveTimeout:
		return "Keep alive timeout"
	case ReasonSessionTakenOver:
		return "Session taken over"

	// ========== 主题相关错误 ==========
	case ReasonTopicFilterInvalid:
		return "Topic filter invalid"
	case ReasonTopicNameInvalid:
		return "Topic name invalid"

	// ========== 数据包 ID 相关错误 ==========
	case ReasonPacketIDInUse:
		return "Packet ID in use"
	case ReasonPacketIDNotFound:
		return "Packet ID not found"

	// ========== 限制/配额相关错误 ==========
	case ReasonReceiveMaxExceeded:
		return "Receive maximum exceeded"
	case ReasonMessageExpired:
		return "Message expired"
	case ReasonPacketTooLarge:
		return "Packet too large"
	case ReasonMessageRateTooHigh:
		return "Message rate too high"
	case ReasonQuotaExceeded:
		return "Quota exceeded"
	case ReasonConnectionRateExceeded:
		return "Connection rate exceeded"
	case ReasonMaxConnectTime:
		return "Maximum connect time"

	// ========== 功能支持相关错误 ==========
	case ReasonPayloadFormatInvalid:
		return "Payload format invalid"
	case ReasonRetainNotSupported:
		return "Retain not supported"
	case ReasonQoSNotSupported:
		return "QoS not supported"
	case ReasonSharedSubNotSupported:
		return "Shared subscription not supported"
	case ReasonPriorityNotSupported:
		return "Priority not supported"

	// ========== 请求/响应相关错误 ==========
	case ReasonActionNotFound:
		return "Action not found"
	case ReasonClientNotFound:
		return "Client not found"
	case ReasonRequestTimeout:
		return "Request timeout"
	case ReasonNoAvailableHandler:
		return "No available handler"
	case ReasonRequestCancelled:
		return "Request cancelled"
	case ReasonDuplicateAction:
		return "Duplicate action"
	case ReasonActionInvalid:
		return "Action invalid"
	case ReasonReceiveWindowFull:
		return "Receive window full"

	default:
		return "Unknown reason"
	}
}

func (r ReasonCode) IsError() bool {
	return r >= 0x80
}

// 默认值
const (
	DefaultKeepAlive     = 60        // 默认心跳间隔（秒）
	DefaultMaxPacketSize = 268435456 // 256 MB
)

// 最大值限制
const (
	MaxClientIDLength   = 65535
	MaxTopicLength      = 65535
	MaxPacketSize       = 268435456 // 256 MB
	MaxVariableIntValue = 268435455 // 可变长度整数最大值
)

// Topic 通配符常量
const (
	TopicWildcardSingle = "*"  // 单层通配符 (替代 MQTT 的 +)
	TopicWildcardMulti  = "**" // 多层通配符 (替代 MQTT 的 #)
)

// Target to core, 用于 client 向 core 发布消息
const TargetToCore = "core"

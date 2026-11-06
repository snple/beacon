package packet

// ProtocolVersion 协议版本。
//
// v4 是 pull 模型：客户端 PUSH 入队、PULL 拉取、COMMIT 确认。
// 与 v2/v3 完全二进制不兼容。
const ProtocolVersion uint8 = 4

// PacketType 数据包类型
type PacketType uint8

const (
	RESERVED     PacketType = 0
	CONNECT      PacketType = 1  // 建立连接
	CONNACK      PacketType = 2  // 连接响应
	PUSH         PacketType = 3  // 客户端发布数据
	PUSH_OK      PacketType = 4  // 发布结果
	PULL         PacketType = 5  // 客户端拉取数据
	PULL_OK      PacketType = 6  // 拉取结果
	COMMIT       PacketType = 7  // 确认已处理
	COMMIT_OK    PacketType = 8  // 确认结果
	PING         PacketType = 9  // 心跳请求
	PONG         PacketType = 10 // 心跳响应
	DISCONNECT   PacketType = 11 // 断开（双向：客户端优雅关闭 / 服务端终止通知）
	DECLARE      PacketType = 12 // 全量替换 channel 声明
	DECLARE_RESP PacketType = 13 // 声明结果

	// stream 控制面（数据面走独立的 stream 端口，见 stream/packet）。
	// 14/15 曾用于已移除的 STREAM_HELLO/STREAM_HELLO_OK；能力改由
	// CONNECT/CONNACK 的 spare bit 协商。
	STREAM_OPEN    PacketType = 16 // 发起方向服务端申请建立流
	STREAM_OPEN_OK PacketType = 17 // 申请结果（含票据与 stream 端口）
	STREAM_REQUEST PacketType = 18 // 服务端转发给目标客户端
	STREAM_ACCEPT  PacketType = 19 // 目标客户端接受/拒绝
	STREAM_ABORT   PacketType = 21 // 服务端通知流已中止
	// 20 曾用于已移除的 STREAM_ACCEPT_OK：接受结果直接由数据面 BIND 结果表达。
)

func (t PacketType) String() string {
	switch t {
	case CONNECT:
		return "CONNECT"
	case CONNACK:
		return "CONNACK"
	case PUSH:
		return "PUSH"
	case PUSH_OK:
		return "PUSH_OK"
	case PULL:
		return "PULL"
	case PULL_OK:
		return "PULL_OK"
	case COMMIT:
		return "COMMIT"
	case COMMIT_OK:
		return "COMMIT_OK"
	case PING:
		return "PING"
	case PONG:
		return "PONG"
	case DISCONNECT:
		return "DISCONNECT"
	case DECLARE:
		return "DECLARE"
	case DECLARE_RESP:
		return "DECLARE_RESP"
	case STREAM_OPEN:
		return "STREAM_OPEN"
	case STREAM_OPEN_OK:
		return "STREAM_OPEN_OK"
	case STREAM_REQUEST:
		return "STREAM_REQUEST"
	case STREAM_ACCEPT:
		return "STREAM_ACCEPT"
	case STREAM_ABORT:
		return "STREAM_ABORT"
	default:
		return "UNKNOWN"
	}
}

// IsRequest 报告该类型是否是客户端发起的请求（需要响应）。
func (t PacketType) IsRequest() bool {
	switch t {
	case CONNECT, PUSH, PULL, COMMIT, DECLARE, PING,
		STREAM_OPEN, STREAM_ACCEPT:
		return true
	default:
		return false
	}
}

// Priority 消息优先级。
//
// v4 中优先级同时决定可靠性：
//   - High / Medium：必须 COMMIT 确认，未确认不会从队列删除
//   - Low：尽力而为，写出成功后立即从队列删除，无需 COMMIT
type Priority uint8

const (
	PriorityLow    Priority = 0
	PriorityMedium Priority = 1 // 默认
	PriorityHigh   Priority = 2
)

func (p Priority) IsValid() bool {
	return p <= PriorityHigh
}

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityMedium:
		return "medium"
	case PriorityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// RequiresCommit 报告该优先级是否需要客户端确认。
func (p Priority) RequiresCommit() bool {
	return p != PriorityLow
}

// ReasonCode 原因码
type ReasonCode uint8

const (
	// ========== 成功类 (0x00-0x0F) ==========
	ReasonSuccess            ReasonCode = 0x00 // 操作成功
	ReasonNormalDisconnect   ReasonCode = 0x01 // 正常断开连接
	ReasonDisconnectWithWill ReasonCode = 0x02 // 带遗嘱消息断开

	// ========== 路由类 (0x10-0x1F) ==========
	ReasonNoMatch ReasonCode = 0x10 // 没有匹配的客户端（含定向目标未订阅该 channel）

	// ========== 通用错误 (0x80-0x83) ==========
	ReasonUnspecifiedError    ReasonCode = 0x80 // 未指定错误
	ReasonMalformedPacket     ReasonCode = 0x81 // 格式错误的数据包
	ReasonProtocolError       ReasonCode = 0x82 // 协议错误
	ReasonImplementationError ReasonCode = 0x83 // 实现错误

	// ========== 连接/认证错误 (0x84-0x87) ==========
	// ReasonUnsupportedProtocol 仅为保留码：当前实现遇到非 v4 版本直接静默关闭。
	ReasonUnsupportedProtocol     ReasonCode = 0x84 // 不支持的协议版本（保留）
	ReasonClientIDNotValid        ReasonCode = 0x85 // 客户端 ID 无效
	ReasonBadAuthMethodOrAuthData ReasonCode = 0x86 // 错误的认证方法或数据
	ReasonNotAuthorized           ReasonCode = 0x87 // 未授权

	// ========== 服务器状态 (0x88-0x8E) ==========
	ReasonServerUnavailable  ReasonCode = 0x88 // 服务器不可用
	ReasonServerBusy         ReasonCode = 0x89 // 服务器繁忙
	ReasonBanned             ReasonCode = 0x8A // 被禁止访问
	ReasonServerShuttingDown ReasonCode = 0x8B // 服务器关闭中
	ReasonKeepAliveTimeout   ReasonCode = 0x8D // 心跳超时
	ReasonSessionTakenOver   ReasonCode = 0x8E // 会话被接管

	// ========== channel 相关 (0x8F-0x90) ==========
	ReasonChannelFilterInvalid ReasonCode = 0x8F // channel 过滤器无效
	ReasonChannelNameInvalid   ReasonCode = 0x90 // channel 名称无效

	// ========== 限制/配额 (0x95-0x97) ==========
	ReasonPacketTooLarge ReasonCode = 0x95 // 数据包过大
	ReasonRateLimited    ReasonCode = 0x96 // 速率超限
	ReasonQuotaExceeded  ReasonCode = 0x97 // 配额超限

	// ========== 投递语义 (0xA0-0xA4) ==========
	ReasonTargetNotFound  ReasonCode = 0xA0 // 定向目标没有会话
	ReasonCommitRequired  ReasonCode = 0xA1 // 需先 COMMIT 才能继续拉取
	ReasonKicked          ReasonCode = 0xA2 // 被管理面踢下线
	ReasonConsumerBacklog ReasonCode = 0xA3 // 消费端积压导致发布被拒
	ReasonChannelInvalid  ReasonCode = 0xA4 // 消息中的 channel 非法（含保留前缀）

	// ========== stream (0xB0-0xB7) ==========
	ReasonStreamDisabled          ReasonCode = 0xB0 // 服务端未开启 stream
	ReasonStreamRejected          ReasonCode = 0xB1 // 目标客户端拒绝
	ReasonStreamTargetUnavailable ReasonCode = 0xB2 // 目标不在线或不支持 stream
	ReasonStreamNotFound          ReasonCode = 0xB3 // 流不存在或已过期
	ReasonStreamLimitExceeded     ReasonCode = 0xB4 // 达到流配额
	ReasonStreamBindFailed        ReasonCode = 0xB5 // 数据面绑定/握手失败
	ReasonStreamUnauthorized      ReasonCode = 0xB6 // 未授权建立该流
	ReasonStreamTimeout           ReasonCode = 0xB7 // 建流协商或绑定超时
)

func (r ReasonCode) String() string {
	switch r {
	case ReasonSuccess:
		return "Success"
	case ReasonNormalDisconnect:
		return "Normal disconnect"
	case ReasonDisconnectWithWill:
		return "Disconnect with will"
	case ReasonNoMatch:
		return "No match"
	case ReasonUnspecifiedError:
		return "Unspecified error"
	case ReasonMalformedPacket:
		return "Malformed packet"
	case ReasonProtocolError:
		return "Protocol error"
	case ReasonImplementationError:
		return "Implementation error"
	case ReasonUnsupportedProtocol:
		return "Unsupported protocol"
	case ReasonClientIDNotValid:
		return "Client ID not valid"
	case ReasonBadAuthMethodOrAuthData:
		return "Bad authentication method or data"
	case ReasonNotAuthorized:
		return "Not authorized"
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
	case ReasonChannelFilterInvalid:
		return "Channel filter invalid"
	case ReasonChannelNameInvalid:
		return "Channel name invalid"
	case ReasonPacketTooLarge:
		return "Packet too large"
	case ReasonRateLimited:
		return "Rate limited"
	case ReasonQuotaExceeded:
		return "Quota exceeded"
	case ReasonTargetNotFound:
		return "Target not found"
	case ReasonCommitRequired:
		return "Commit required"
	case ReasonKicked:
		return "Kicked"
	case ReasonConsumerBacklog:
		return "Consumer backlog"
	case ReasonChannelInvalid:
		return "Channel invalid"
	case ReasonStreamDisabled:
		return "Stream disabled"
	case ReasonStreamRejected:
		return "Stream rejected"
	case ReasonStreamTargetUnavailable:
		return "Stream target unavailable"
	case ReasonStreamNotFound:
		return "Stream not found"
	case ReasonStreamLimitExceeded:
		return "Stream limit exceeded"
	case ReasonStreamBindFailed:
		return "Stream bind failed"
	case ReasonStreamUnauthorized:
		return "Stream unauthorized"
	case ReasonStreamTimeout:
		return "Stream timeout"
	default:
		return "Unknown reason"
	}
}

// IsError 报告该原因码是否表示失败。
func (r ReasonCode) IsError() bool {
	return r >= 0x80
}

// 默认值
const (
	DefaultKeepAlive     = 60       // 默认心跳间隔（秒）
	DefaultMaxPacketSize = 16777216 // 16 MB
)

// 最大值限制
const (
	// MaxClientIDLength 客户端 ID 的最大字节数。
	MaxClientIDLength = 256
	// MinChannelLength 服务端可配置的 channel 名/过滤器长度上限的最小值。
	MinChannelLength = 256
	// MaxChannelLength 协议允许的 channel 名/过滤器长度上限（硬上限）。
	// 服务端实际生效的上限由 CoreOptions.MaxChannelLength 配置，取值必须落在
	// [MinChannelLength, MaxChannelLength] 区间内。
	MaxChannelLength = 1024
	// MaxPushBatch 单次 PUSH 允许的最大记录数（协议硬上限）。
	// 服务端可通过 MaxBatch 配置更小的值，默认 16。
	MaxPushBatch = 128
	// MaxPacketSize 协议硬上限。
	MaxPacketSize = 268435456 // 256 MB
)

// channel 通配符常量
const (
	ChannelWildcardSingle = "*"  // 单层通配符
	ChannelWildcardMulti  = "**" // 多层通配符（只能出现在末尾）
)

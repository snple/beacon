package packet

// PacketType stream协议数据包类型
type PacketType uint8

const (
	// 连接管理
	CONNECT PacketType = 0x01 // 客户端连接请求
	CONNACK PacketType = 0x02 // 服务端连接确认

	// 流管理
	STREAM_OPEN  PacketType = 0x10 // 打开流请求
	STREAM_ACK   PacketType = 0x11 // 流确认响应
	STREAM_DATA  PacketType = 0x12 // 流数据传输
	STREAM_CLOSE PacketType = 0x13 // 关闭流通知
	STREAM_READY PacketType = 0x14 // 客户端准备接收流

	// 心跳
	PING PacketType = 0x20 // 心跳请求
	PONG PacketType = 0x21 // 心跳响应
)

// ReasonCode 原因码
type ReasonCode uint8

const (
	Success               ReasonCode = 0x00 // 成功
	UnspecifiedError      ReasonCode = 0x80 // 未指定错误
	MalformedPacket       ReasonCode = 0x81 // 数据包格式错误
	ProtocolErr           ReasonCode = 0x82 // 协议错误
	NotAuthorized         ReasonCode = 0x87 // 未授权
	ServerUnavailable     ReasonCode = 0x88 // 服务不可用
	ClientNotConnected    ReasonCode = 0x90 // 客户端未连接
	ClientNotFound        ReasonCode = 0x91 // 目标客户端不存在
	NoConnectionAvailable ReasonCode = 0x92 // 无可用连接
	QuotaExceeded         ReasonCode = 0x97 // 超出配额
	StreamNotFound        ReasonCode = 0xA0 // 流不存在
	StreamAlreadyExists   ReasonCode = 0xA1 // 流已存在
	StreamRejected        ReasonCode = 0xA2 // 流被拒绝
)

// String 返回原因码描述
func (r ReasonCode) String() string {
	switch r {
	case Success:
		return "Success"
	case UnspecifiedError:
		return "Unspecified error"
	case MalformedPacket:
		return "Malformed packet"
	case ProtocolErr:
		return "Protocol error"
	case NotAuthorized:
		return "Not authorized"
	case ServerUnavailable:
		return "Server unavailable"
	case ClientNotConnected:
		return "Client not connected"
	case ClientNotFound:
		return "Client not found"
	case NoConnectionAvailable:
		return "No connection available"
	case QuotaExceeded:
		return "Quota exceeded"
	case StreamNotFound:
		return "Stream not found"
	case StreamAlreadyExists:
		return "Stream already exists"
	case StreamRejected:
		return "Stream rejected"
	default:
		return "Unknown"
	}
}

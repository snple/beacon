// Package packet 定义 stream 数据面握手协议。
//
// 该协议只用于 core 的独立 stream 端口：客户端先通过主连接（packet 包）拿到
// 一次性票据，再在这里用 BIND 绑定，绑定成功后连接转为裸 TCP 字节流。
package packet

// Version 是 stream 数据面握手协议版本。
const Version uint8 = 1

// MaxPacketSize 是握手包的大小上限。握手包只包含票据和小段元数据。
const MaxPacketSize uint32 = 64 * 1024

// PacketType 是 stream 端口握手包类型。
type PacketType uint8

const (
	BIND    PacketType = 0x01 // 客户端绑定到某个待建流
	BIND_OK PacketType = 0x02 // 绑定结果：成功即表示流已建立
)

func (t PacketType) String() string {
	switch t {
	case BIND:
		return "BIND"
	case BIND_OK:
		return "BIND_OK"
	default:
		return "UNKNOWN"
	}
}

// Role 表示数据面连接在流中的角色。
type Role uint8

const (
	RoleInitiator Role = 0 // 发起方（A）
	RoleAcceptor  Role = 1 // 接受方（B）
)

func (r Role) String() string {
	switch r {
	case RoleInitiator:
		return "initiator"
	case RoleAcceptor:
		return "acceptor"
	default:
		return "unknown"
	}
}

// ReasonCode 是 stream 数据面握手结果。
type ReasonCode uint8

const (
	Success             ReasonCode = 0x00
	StreamRejected      ReasonCode = 0xB1
	StreamNotFound      ReasonCode = 0xB3
	StreamLimitExceeded ReasonCode = 0xB4
	BindFailed          ReasonCode = 0xB5
	UnspecifiedError    ReasonCode = 0x80
	MalformedPacket     ReasonCode = 0x81
	ProtocolError       ReasonCode = 0x82
	NotAuthorized       ReasonCode = 0x87
	ServerUnavailable   ReasonCode = 0x88
)

func (r ReasonCode) String() string {
	switch r {
	case Success:
		return "Success"
	case StreamRejected:
		return "Stream rejected"
	case StreamNotFound:
		return "Stream not found"
	case StreamLimitExceeded:
		return "Stream limit exceeded"
	case BindFailed:
		return "Bind failed"
	case UnspecifiedError:
		return "Unspecified error"
	case MalformedPacket:
		return "Malformed packet"
	case ProtocolError:
		return "Protocol error"
	case NotAuthorized:
		return "Not authorized"
	case ServerUnavailable:
		return "Server unavailable"
	default:
		return "Unknown"
	}
}

package core

import (
	"sync"
	"sync/atomic"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
)

// session 表示客户端的会话状态
// 负责管理跨连接持久化的状态：订阅、PacketID 种子、会话配置等
type session struct {
	// 会话标识
	id string

	// 会话配置
	// keep    bool   // 是否保持会话
	timeout uint32 // 会话过期时间（秒），0 表示断开即清理

	authMethod string // 认证方法
	authData   []byte // 认证数据

	// 遗嘱消息
	willPacket *packet.PublishPacket

	// 订阅状态（会话级别，跨连接持久化）
	subscriptions map[string]packet.SubscribeOptions
	subsMu        sync.RWMutex

	// QoS 状态 (只需要 QoS 0/1)
	pendingAck   map[nson.Id]pendingMessage // QoS 1: 等待客户端 ACK 的消息
	pendingAckMu sync.Mutex

	// 重传标志
	retransmitting atomic.Bool // true: 正在重传消息

	core *Core
}

// newSession 创建新的会话
func newSession(clientID string, connect *packet.ConnectPacket, core *Core) *session {
	s := &session{
		id:            clientID,
		subscriptions: make(map[string]packet.SubscribeOptions),
		pendingAck:    make(map[nson.Id]pendingMessage),
		core:          core,
	}

	// 提取认证信息
	if connect.Properties != nil {
		s.authMethod = connect.Properties.AuthMethod
		s.authData = connect.Properties.AuthData
	}

	// 处理会话过期时间
	// 规则：
	// 1. SessionTimeout=0: 会话无效，断开即清理
	// 2. SessionTimeout>0:
	//   - 服务器未设置 MaxSessionTimeout: 取客户端值作为会话过期时间
	//   - 客户端设置了值: 取 min(客户端值, MaxSessionTimeout) 作为会话过期时间
	if connect.SessionTimeout > 0 {
		if core.options.MaxSessionTimeout > 0 {
			s.timeout = min(connect.SessionTimeout, core.options.MaxSessionTimeout)
		} else {
			s.timeout = connect.SessionTimeout
		}
	} else {
		s.timeout = 0
	}

	return s
}

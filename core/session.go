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
	// timeout 会话过期时间（秒），0 表示断开即清理。
	// 使用原子读写：会话恢复时会在新连接的协程中更新，而其它协程可能同时读取。
	timeout atomic.Uint32

	authMethod string // 认证方法
	authData   []byte // 认证数据

	// 遗嘱消息（公开 API SetWillPacket/WillPacket 与断开清理可能并发访问）
	willPacket *packet.PublishPacket
	willMu     sync.Mutex

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

	s.timeout.Store(sessionTimeoutFromConnect(connect, core))

	return s
}

// sessionTimeoutFromConnect 计算会话过期时间（秒）。
// 规则：
//  1. SessionTimeout=0: 会话无效，断开即清理
//  2. SessionTimeout>0:
//     - 服务器未设置 MaxSessionTimeout: 取客户端值
//     - 服务器设置了 MaxSessionTimeout: 取 min(客户端值, MaxSessionTimeout)
func sessionTimeoutFromConnect(connect *packet.ConnectPacket, core *Core) uint32 {
	if connect.SessionTimeout == 0 {
		return 0
	}
	if core.options.MaxSessionTimeout > 0 {
		return min(connect.SessionTimeout, core.options.MaxSessionTimeout)
	}
	return connect.SessionTimeout
}

// applyConnect 用最新的 CONNECT 更新会话参数（会话恢复时调用）。
func (s *session) applyConnect(connect *packet.ConnectPacket, core *Core) {
	s.timeout.Store(sessionTimeoutFromConnect(connect, core))

	if connect.Properties != nil {
		s.authMethod = connect.Properties.AuthMethod
		s.authData = connect.Properties.AuthData
	}
}

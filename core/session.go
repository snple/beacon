package core

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

// willMessage 是一条遗嘱消息。
type willMessage struct {
	Channel  string
	Payload  []byte
	Headers  map[string]string
	Priority packet.Priority
}

// Client 表示一个客户端会话（可能在线，也可能离线保留）。
type Client struct {
	ID string

	// sessionID 是会话租约标识：仅在会话创建或接管时变化。
	// 管理面可能并发读取，因此使用原子读写。
	sessionID atomic.Uint64

	core *Core
	mb   *mailbox

	chMu     sync.RWMutex
	channels []packet.ChannelDecl

	identity atomic.Pointer[Identity]

	timeout   atomic.Uint32
	expiresAt atomic.Int64 // Unix 毫秒；0 表示在线

	connMu sync.Mutex
	conn   *conn

	createdAt   time.Time
	connectedAt atomic.Int64
	lastSeen    atomic.Int64
	// deliveredSince 记录 delivered 从空变为非空的时间（Unix 毫秒），0 表示当前为空。
	deliveredSince atomic.Int64

	pushLimiter *tokenBucket

	dropped  atomic.Int64
	sent     atomic.Int64
	rejected atomic.Int64
}

func newClient(core *Core, clientID string, connect *packet.ConnectPacket) *Client {
	cl := &Client{
		ID:        clientID,
		core:      core,
		createdAt: time.Now(),
	}
	cl.sessionID.Store(newSessionID())
	cl.mb = newMailbox(core.options.QueueCap, core.releaseMessage)
	cl.timeout.Store(effectiveSessionTimeout(core.options, connect.SessionTimeout))
	cl.channels = append([]packet.ChannelDecl(nil), connect.Channels...)

	if core.options.PushRate > 0 {
		cl.pushLimiter = newTokenBucket(core.options.PushRate, core.options.PushBurst)
	}

	return cl
}

func effectiveSessionTimeout(opts *CoreOptions, requested uint32) uint32 {
	if opts.MaxSessionTimeout > 0 && requested > opts.MaxSessionTimeout {
		return opts.MaxSessionTimeout
	}
	return requested
}

func newSessionID() uint64 {
	id := rand.Uint64()
	if id == 0 {
		id = 1
	}
	return id
}

// currentConn 返回当前绑定的连接（可能为 nil）。
func (cl *Client) currentConn() *conn {
	cl.connMu.Lock()
	defer cl.connMu.Unlock()
	return cl.conn
}

// swapConn 把会话绑定到新连接，返回被替换的旧连接。
func (cl *Client) swapConn(cn *conn) *conn {
	cl.connMu.Lock()
	old := cl.conn
	cl.conn = cn
	cl.connMu.Unlock()
	return old
}

// owns 报告给定连接是否仍是该会话的当前连接。
func (cl *Client) owns(cn *conn) bool {
	cl.connMu.Lock()
	defer cl.connMu.Unlock()
	return cl.conn == cn
}

// releaseConn 在连接断开时解绑；仅当它仍是当前连接时才生效。
func (cl *Client) releaseConn(cn *conn) bool {
	cl.connMu.Lock()
	defer cl.connMu.Unlock()
	if cl.conn != cn {
		return false
	}
	cl.conn = nil
	return true
}

// notify 唤醒该会话当前的连接（用于长轮询）。
func (cl *Client) notify() {
	if cn := cl.currentConn(); cn != nil {
		cn.notify()
	}
}

func (cl *Client) setIdentity(id *Identity) { cl.identity.Store(id.Clone()) }

func (cl *Client) getIdentity() *Identity { return cl.identity.Load() }

// replaceChannels 全量替换声明并同步到 matcher。
func (cl *Client) replaceChannels(decls []packet.ChannelDecl) {
	cl.chMu.Lock()
	cl.channels = append([]packet.ChannelDecl(nil), decls...)
	cl.chMu.Unlock()

	cl.core.matcher.declare(cl.ID, decls)
}

// sessionTimeout 返回会话保留时长（秒）。
func (cl *Client) sessionTimeout() uint32 { return cl.timeout.Load() }

// markExpiry 记录离线过期时间（timeout=0 表示立即过期）。
func (cl *Client) markExpiry(now time.Time) {
	t := cl.timeout.Load()
	if t == 0 {
		cl.expiresAt.Store(now.UnixMilli())
		return
	}
	cl.expiresAt.Store(now.Add(time.Duration(t) * time.Second).UnixMilli())
}

// expiry 返回离线过期时间；在线时为零值。
func (cl *Client) expiry() time.Time {
	at := cl.expiresAt.Load()
	if at <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(at)
}

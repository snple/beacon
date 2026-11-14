package core

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
)

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

	// recvMax 是该客户端最近一次声明的接收上限（CONNECT.MaxPacketSize，
	// 0 表示按服务端上限）。用于在入队时拒绝目标根本无法接收的记录。
	recvMax atomic.Uint32

	// recvWindow 是本会话允许的最大未确认可靠消息数（ReceiveMaximum）。
	recvWindow atomic.Uint32

	timeout   atomic.Uint32
	expiresAt atomic.Int64 // Unix 毫秒；0 表示在线

	connMu sync.Mutex
	conn   *conn

	// resetEpoch 在 CleanSession 重置时递增，用于让旧连接的 in-flight PULL
	// 不再把消息写回已被清理的会话。
	resetEpoch atomic.Uint64

	createdAt   time.Time
	connectedAt atomic.Int64
	lastSeen    atomic.Int64
	// deliveredSince 记录 delivered 从空变为非空的时间（Unix 毫秒），0 表示当前为空。
	deliveredSince atomic.Int64

	pushLimiter *tokenBucket

	dropped  atomic.Int64
	sent     atomic.Int64
	rejected atomic.Int64
	nacked   atomic.Int64
	expired  atomic.Int64
}

func newClient(core *Core, clientID string) *Client {
	cl := &Client{
		ID:        clientID,
		core:      core,
		createdAt: time.Now(),
	}
	cl.sessionID.Store(newSessionID())
	cl.mb = newMailbox(core.options.QueueCap, core.options.MaxPendingBytesPerClient, core.releaseMessage)
	cl.timeout.Store(core.options.SessionTimeout)
	cl.recvWindow.Store(uint32(core.options.ReceiveMaximum))

	if core.options.PushRate > 0 {
		cl.pushLimiter = newTokenBucket(core.options.PushRate, core.options.PushBurst)
	}

	return cl
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

// enqueue 把消息放入会话队列。
//
// 离线会话只保留可靠消息：不可靠消息直接丢弃（skipped=true），
// 避免可丢失的数据占用离线队列。队列满时仍由 mailbox.enqueue 按
// (reliability, priority) 淘汰。
func (cl *Client) enqueue(msg *Message) (enqueued, evicted, skipped bool) {
	cl.connMu.Lock()
	defer cl.connMu.Unlock()

	if !msg.Reliable && cl.conn == nil {
		return false, false, true
	}
	enqueued, evicted = cl.mb.enqueue(msg)
	return enqueued, evicted, false
}

// reset 清空会话状态并更换 SessionID（CONNECT 的 CleanSession=true）。
func (cl *Client) reset() {
	cl.resetEpoch.Add(1)
	if n := cl.mb.clearAll(); n > 0 {
		cl.core.stats.MessagesDropped.Add(int64(n))
	}
	cl.replaceChannels(nil)
	cl.deliveredSince.Store(0)
	cl.dropped.Store(0)
	cl.sent.Store(0)
	cl.rejected.Store(0)
	cl.nacked.Store(0)
	cl.expired.Store(0)
	cl.sessionID.Store(newSessionID())
}

func (cl *Client) setIdentity(id *Identity) { cl.identity.Store(id.Clone()) }

func (cl *Client) getIdentity() *Identity { return cl.identity.Load() }

// replaceChannels 全量替换声明并同步到 matcher。
//
// matcher 只登记参与广播匹配的声明；只接收定向投递的声明不登记，
// 定向投递直接查会话声明（matchesChannel），因此仍可命中。
func (cl *Client) replaceChannels(decls []packet.ChannelDecl) {
	cl.chMu.Lock()
	cl.channels = append([]packet.ChannelDecl(nil), decls...)
	cl.chMu.Unlock()

	broadcast := make([]packet.ChannelDecl, 0, len(decls))
	for _, d := range decls {
		if !d.TargetOnly() {
			broadcast = append(broadcast, d)
		}
	}
	cl.core.matcher.declare(cl.ID, broadcast)
}

// declaredChannels 返回当前声明的快照。
func (cl *Client) declaredChannels() []packet.ChannelDecl {
	cl.chMu.RLock()
	defer cl.chMu.RUnlock()
	if len(cl.channels) == 0 {
		return nil
	}
	return append([]packet.ChannelDecl(nil), cl.channels...)
}

// channelInfos 返回当前声明的管理面视图。
func (cl *Client) channelInfos() []ChannelInfo {
	decls := cl.declaredChannels()
	if len(decls) == 0 {
		return nil
	}
	out := make([]ChannelInfo, len(decls))
	for i, d := range decls {
		out[i] = ChannelInfo{Filter: d.Channel, TargetOnly: d.TargetOnly()}
	}
	return out
}

// matchesChannel 报告该会话是否声明了能匹配 channel 的过滤器。
//
// 用于定向投递：目标必须主动声明过该 channel（含只接收定向投递的声明），
// 否则定向投递就是绕过订阅/读权限的后门。
func (cl *Client) matchesChannel(channel string) bool {
	cl.chMu.RLock()
	defer cl.chMu.RUnlock()
	for _, d := range cl.channels {
		if packet.MatchChannel(d.Channel, channel) {
			return true
		}
	}
	return false
}

// sessionTimeout 返回会话保留时长（秒）。
func (cl *Client) sessionTimeout() uint32 { return cl.timeout.Load() }

// receiveLimit 返回该会话最近一次声明的接收上限（0 表示不限制）。
func (cl *Client) receiveLimit() uint32 { return cl.recvMax.Load() }

// receiveWindow 返回该会话允许的最大未确认可靠消息数。
func (cl *Client) receiveWindow() uint32 {
	n := cl.recvWindow.Load()
	if n == 0 {
		return 1
	}
	return n
}

// setReceiveParamsIfOwner 仅在 cn 仍是会话当前连接时记录接收上限与窗口。
// 被接管的旧连接不得再修改会话状态（fencing）。
func (cl *Client) setReceiveParamsIfOwner(cn *conn, limit, window uint32) bool {
	cl.connMu.Lock()
	defer cl.connMu.Unlock()
	if cl.conn != cn {
		return false
	}
	cl.recvMax.Store(limit)
	if window > 0 {
		cl.recvWindow.Store(window)
	}
	return true
}

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

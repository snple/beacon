package core

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// HandleConn 处理一个已建立的连接。
//
// 支持 TCP、TLS、WebSocket 等任何实现了 net.Conn 的连接。
func (c *Core) HandleConn(nc net.Conn) error {
	if !c.running.Load() {
		_ = nc.Close()
		return fmt.Errorf("core is not running")
	}

	if c.connLimiter != nil && !c.connLimiter.Allow() {
		c.rejectConn(nc, packet.ReasonRateLimited, "connection rate exceeded")
		return fmt.Errorf("connection rate exceeded")
	}

	if c.handshakeSlots != nil {
		select {
		case c.handshakeSlots <- struct{}{}:
		default:
			c.rejectConn(nc, packet.ReasonServerBusy, "too many concurrent handshakes")
			return fmt.Errorf("too many concurrent handshakes")
		}
	}

	c.trackHandshakeConn(nc)

	if !c.spawnTask(func() { c.handleConn(nc) }) {
		c.untrackHandshakeConn(nc)
		c.releaseHandshakeSlot()
		_ = nc.Close()
		return ErrCoreNotRunning
	}
	return nil
}

func (c *Core) rejectConn(nc net.Conn, reason packet.ReasonCode, msg string) {
	c.logger.Warn("rejecting connection", zap.String("remote", safeAddr(nc)), zap.String("reason", msg))
	connack := packet.NewConnackPacket(reason)
	connack.ServerMaxPacketSize = c.options.MaxPacketSize
	if c.options.WriteTimeout > 0 {
		_ = nc.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
	}
	_ = packet.WritePacket(nc, connack, c.options.MaxPacketSize)
	_ = nc.Close()
}

func safeAddr(nc net.Conn) string {
	if nc == nil || nc.RemoteAddr() == nil {
		return ""
	}
	return nc.RemoteAddr().String()
}

// handleConn 完成握手并进入读循环。
func (c *Core) handleConn(nc net.Conn) {
	cn := newConn(c, nc)

	var (
		reason       = packet.ReasonNormalDisconnect
		will         *willMessage
		suppressWill bool
		clientID     string
	)

	defer func() {
		c.closeStreamsForConn(cn)
		c.untrackHandshakeConn(nc)
		c.releaseHandshakeSlot()
		cn.close()
		// 由连接协程（而不是 waitLoop 自身）等待后台任务退出。
		cn.wait()

		cl := cn.client
		if cl != nil {
			// 仅当本连接仍是会话的当前连接时才处理遗嘱；被接管的旧连接不能触发。
			wasOwner := c.detachConn(cl, cn)
			if wasOwner && will != nil && !suppressWill && !cn.serverBy.Load() {
				c.publishWill(cl, will)
			}
			c.options.Hooks.callOnDisconnect(&DisconnectContext{
				ClientID: cl.ID,
				Reason:   reason,
			})
		}
	}()

	// 读取 CONNECT
	if c.options.ConnectTimeout > 0 {
		_ = nc.SetReadDeadline(time.Now().Add(time.Duration(c.options.ConnectTimeout) * time.Second))
	}

	pkt, size, err := packet.ReadPacketWithSize(nc, c.options.MaxPacketSize)
	if err != nil {
		var tooLarge *packet.PacketTooLargeError
		if errors.As(err, &tooLarge) {
			c.rejectConn(nc, packet.ReasonPacketTooLarge, "connect packet too large")
		}
		return
	}
	c.stats.BytesReceived.Add(int64(size))
	_ = nc.SetReadDeadline(time.Time{})

	connect, ok := pkt.(*packet.ConnectPacket)
	if !ok {
		c.rejectConn(nc, packet.ReasonProtocolError, "first packet is not CONNECT")
		return
	}

	if connect.Flags.Unknown() != 0 {
		c.rejectConn(nc, packet.ReasonProtocolError, "unknown connect flags")
		return
	}

	if connect.Version != packet.ProtocolVersion {
		// 不向后兼容：非 v4 的包不做任何协商，直接静默关闭（不发 CONNACK）。
		// 真正的旧版 CONNECT 通常在此之前就已因布局不同而解码失败。
		c.logger.Debug("unsupported protocol version",
			zap.String("remote", cn.remoteAddr), zap.Uint8("version", connect.Version))
		return
	}

	cn.streamCapable.Store(connect.StreamCapable())

	clientID = connect.ClientID
	if clientID == "" {
		clientID = c.generateClientID()
	} else if !packet.ValidateClientID(clientID) {
		c.rejectConn(nc, packet.ReasonClientIDNotValid, "invalid client id")
		return
	}

	// 认证（单轮）
	var identity *Identity
	authCtx := &AuthContext{
		ClientID:   clientID,
		RemoteAddr: cn.remoteAddr,
		Method:     connect.AuthMethod,
		Data:       connect.AuthData,
	}
	if err := c.options.Hooks.callAuth(authCtx); err != nil {
		reason := packet.ReasonNotAuthorized
		var ae *AuthError
		if errors.As(err, &ae) && ae.Reason != 0 {
			reason = ae.Reason
		}
		c.rejectConn(nc, reason, "authentication failed")
		return
	}
	identity = authCtx.Identity

	// will 校验（在建立会话前做，避免无效 will 占用会话）
	if connect.HasWill {
		if !packet.ValidateClientChannelName(connect.WillChannel, c.options.MaxChannelLength) {
			c.rejectConn(nc, packet.ReasonChannelNameInvalid, "invalid will channel")
			return
		}
		if !c.canWrite(identity, connect.WillChannel) {
			c.rejectConn(nc, packet.ReasonNotAuthorized, "will channel not authorized")
			return
		}
		priority := packet.PriorityMedium
		will = &willMessage{
			Channel:  connect.WillChannel,
			Payload:  connect.WillPayload,
			Headers:  connect.WillHeaders,
			Priority: priority,
		}
	}

	// 绑定会话（必要时接管）。会话状态在 registerMu 内一次性设置，
	// 避免本连接在后续步骤中丢失所有权后仍覆盖新连接的状态。
	cl, sessionPresent, oldConn, err := c.attachSession(clientID, connect, cn, identity)
	if err != nil {
		c.rejectConn(nc, packet.ReasonServerBusy, err.Error())
		return
	}
	cn.client = cl

	// 接管：先切换连接，再通知旧连接。即使本连接随后发现自己也已被接管，
	// 也必须终止它挤掉的旧连接。
	if oldConn != nil {
		c.stats.SessionTakeovers.Add(1)
		go oldConn.terminate(packet.ReasonSessionTakenOver, true)
	}

	// attachSession 之后可能又有同 ClientID 的连接接管了本连接。
	if !cl.owns(cn) {
		return
	}

	// 连接参数：服务端显式配置优先，其次用客户端声明，最后回落到默认值。
	// keepAlive=0 会导致永不超时，必须兜底。
	keepAlive := connect.KeepAlive
	if c.options.KeepAlive > 0 {
		keepAlive = c.options.KeepAlive
	}
	if keepAlive == 0 {
		keepAlive = packet.DefaultKeepAlive
	}
	cn.keepAlive = keepAlive

	outMax := connect.MaxPacketSize
	if outMax == 0 || outMax > c.options.MaxPacketSize {
		outMax = c.options.MaxPacketSize
	}
	cn.maxPacketSize = outMax

	pending, delivered, oldest := cl.mb.depth()
	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	connack.SetSessionPresent(sessionPresent)
	connack.SetStreamSupported(true) // 本实现支持 stream 控制包
	_, _, hasStreamListener := c.streamAdvertise()
	connack.SetStreamEnabled(connect.StreamCapable() && c.streamEnabled.Load() && hasStreamListener)
	connack.AssignedClientID = clientID
	connack.ServerMaxPacketSize = c.options.MaxPacketSize
	connack.KeepAlive = keepAlive
	connack.SessionID = cl.sessionID.Load()
	connack.Pending = pending
	connack.Delivered = delivered
	connack.OldestUncommitted = oldest

	if err := cn.writePacket(connack); err != nil {
		return
	}

	cn.registered.Store(true)
	c.stats.ClientsConnected.Add(1)

	c.options.Hooks.callOnConnect(&ConnectContext{
		ClientID:       clientID,
		RemoteAddr:     cn.remoteAddr,
		SessionPresent: sessionPresent,
	})

	c.logger.Debug("client connected",
		zap.String("clientID", clientID),
		zap.String("remote", cn.remoteAddr),
		zap.Bool("sessionPresent", sessionPresent))

	err = cn.serve()
	switch {
	case errors.Is(err, io.EOF):
		reason = packet.ReasonNormalDisconnect
		if dp := cn.lastDisconnect(); dp != nil {
			reason = dp.ReasonCode
			if dp.Flags&packet.DisconnectFlagSuppressWill != 0 || reason == packet.ReasonNormalDisconnect {
				suppressWill = true
			}
		}
	default:
		// 服务端主动终止（接管/超时/协议错误）时 serve 已记录真实原因。
		if r := cn.endReasonCode(); r != 0 {
			reason = r
		} else if err != nil {
			reason = packet.ReasonKeepAliveTimeout
		}
	}
}

// attachSession 绑定（或新建）会话，返回会话、是否复用会话、被替换的旧连接。
//
// 会话层面的全部状态（channel 声明、在线时间）都在 registerMu 内写入，
// 因此不会出现两个并发握手互相覆盖会话状态的情况。
func (c *Core) attachSession(
	clientID string,
	connect *packet.ConnectPacket,
	cn *conn,
	identity *Identity,
) (*Client, bool, *conn, error) {
	c.registerMu.Lock()
	defer c.registerMu.Unlock()

	c.clientsMu.RLock()
	existing := c.clients[clientID]
	c.clientsMu.RUnlock()

	var (
		cl             *Client
		sessionPresent bool
		needSlot       bool
	)

	if existing != nil {
		cl = existing
		sessionPresent = true
		if connect.CleanSession() {
			sessionPresent = false
		}
		needSlot = cl.currentConn() == nil
	} else {
		if c.options.MaxSessions > 0 && len(c.clients) >= c.options.MaxSessions {
			return nil, false, nil, ErrMaxSessions
		}
		needSlot = true
		cl = newClient(c, clientID)
		c.clientsMu.Lock()
		c.clients[clientID] = cl
		c.clientsMu.Unlock()
		c.stats.ClientsTotal.Add(1)
	}

	// 在线名额在 registerMu 内预留，避免并发握手同时通过检查后超卖。
	if needSlot {
		if c.options.MaxClients > 0 && c.onlineConns >= int(c.options.MaxClients) {
			if existing == nil {
				c.removeClientLocked(cl)
			}
			return nil, false, nil, ErrMaxClients
		}
		c.onlineConns++
	}

	// 认证身份与本次连接绑定；即使恢复会话也以本次认证结果为准。
	cl.setIdentity(identity)

	old := cl.swapConn(cn)
	if old != nil {
		// 接管：更换会话租约标识，旧连接此后无法通过 fencing 检查
		cl.sessionID.Store(newSessionID())
	} else if !sessionPresent {
		cl.sessionID.Store(newSessionID())
	}

	// CleanSession：在新连接绑定后清空既有队列、声明与统计。
	if connect.CleanSession() && existing != nil {
		cl.reset()
	}

	// 恢复会话时按本次认证得到的身份重新校验保留的声明：
	// 上次连接允许的过滤器可能超出本次身份的授权范围。
	cl.replaceChannels(c.filterChannels(identity, cl.declaredChannels()))

	now := time.Now().UnixMilli()
	cl.connectedAt.Store(now)
	cl.lastSeen.Store(now)
	cl.expiresAt.Store(0)

	return cl, sessionPresent, old, nil
}

// removeClientLocked 移除刚创建但未能完成握手的会话。
func (c *Core) removeClientLocked(cl *Client) {
	c.clientsMu.Lock()
	if cur, ok := c.clients[cl.ID]; ok && cur == cl {
		delete(c.clients, cl.ID)
	}
	c.clientsMu.Unlock()
	c.matcher.remove(cl.ID)
}

// detachConn 在连接结束时解绑会话，返回本连接是否仍是会话的所有者。
func (c *Core) detachConn(cl *Client, cn *conn) bool {
	if cl == nil {
		return false
	}

	if cn != nil && cn.registered.Swap(false) {
		c.stats.ClientsConnected.Add(-1)
	}

	if !cl.releaseConn(cn) {
		// 已被新连接接管：不改变会话状态
		return false
	}

	// 释放 attachSession 预留的在线名额。
	c.registerMu.Lock()
	if c.onlineConns > 0 {
		c.onlineConns--
	}
	c.registerMu.Unlock()

	// 会话转入离线：Low 是尽力而为的消息，不保留在离线队列中。
	if n := cl.mb.dropLow(); n > 0 {
		c.stats.MessagesDropped.Add(int64(n))
		cl.dropped.Add(int64(n))
	}

	cl.markExpiry(time.Now())
	if cl.sessionTimeout() == 0 {
		c.destroySession(cl, packet.ReasonNormalDisconnect)
	}
	return true
}

// filterChannels 过滤非法与未授权的声明。
func (c *Core) filterChannels(identity *Identity, decls []packet.ChannelDecl) []packet.ChannelDecl {
	if len(decls) == 0 {
		return nil
	}
	if c.options.MaxChannelsPerClient > 0 && len(decls) > c.options.MaxChannelsPerClient {
		decls = decls[:c.options.MaxChannelsPerClient]
	}

	out := make([]packet.ChannelDecl, 0, len(decls))
	seen := make(map[string]struct{}, len(decls))
	for _, d := range decls {
		if _, dup := seen[d.Channel]; dup {
			continue
		}
		seen[d.Channel] = struct{}{}

		if !packet.ValidateChannelFilterLimit(d.Channel, c.options.MaxChannelLength) {
			continue
		}
		if !c.canRead(identity, d.Channel) {
			continue
		}
		out = append(out, d)
	}
	return out
}

// publishWill 发布遗嘱消息。
func (c *Core) publishWill(cl *Client, w *willMessage) {
	rec := &packet.Record{
		Channel: w.Channel,
		Source:  cl.ID,
		Headers: w.Headers,
		Payload: w.Payload,
	}
	rec.SetPriority(w.Priority)

	c.pushRecord(nil, rec, true)
	c.logger.Debug("published will", zap.String("clientID", cl.ID), zap.String("channel", w.Channel))
}

// ============================================================================
// 握手辅助
// ============================================================================

func (c *Core) trackHandshakeConn(nc net.Conn) {
	c.handshakeConnsMu.Lock()
	if c.handshakeConns == nil {
		c.handshakeConns = make(map[net.Conn]struct{})
	}
	c.handshakeConns[nc] = struct{}{}
	c.handshakeConnsMu.Unlock()
}

func (c *Core) untrackHandshakeConn(nc net.Conn) {
	c.handshakeConnsMu.Lock()
	delete(c.handshakeConns, nc)
	c.handshakeConnsMu.Unlock()
}

func (c *Core) releaseHandshakeSlot() {
	if c.handshakeSlots == nil {
		return
	}
	select {
	case <-c.handshakeSlots:
	default:
	}
}

func (c *Core) closeHandshakeConns() {
	c.handshakeConnsMu.Lock()
	conns := make([]net.Conn, 0, len(c.handshakeConns))
	for nc := range c.handshakeConns {
		conns = append(conns, nc)
	}
	c.handshakeConnsMu.Unlock()

	for _, nc := range conns {
		_ = nc.Close()
	}
}

func (c *Core) generateClientID() string {
	return "auto-" + packet.NewPacketID().Hex()
}

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
		c.untrackHandshakeConn(nc)
		c.releaseHandshakeSlot()
		cn.close()

		cl := cn.client
		if cl != nil {
			c.detachConn(cl, cn)
			if will != nil && !suppressWill && !cn.serverBy.Load() {
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

	if connect.Version != packet.ProtocolVersion {
		c.rejectConn(nc, packet.ReasonUnsupportedProtocol, "unsupported protocol version")
		return
	}

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

	// 客户端数量限制
	if c.options.MaxClients > 0 && c.stats.ClientsConnected.Load() >= int64(c.options.MaxClients) {
		c.rejectConn(nc, packet.ReasonServerBusy, "max clients reached")
		return
	}

	// will 校验（在建立会话前做，避免无效 will 占用会话）
	if connect.HasWill {
		if !packet.ValidateChannelName(connect.WillChannel) {
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

	// 绑定会话（必要时接管）
	cl, sessionPresent, oldConn := c.attachSession(clientID, connect, cn, identity)
	cn.client = cl

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

	if will != nil {
		cl.setWill(will)
	} else {
		cl.clearWill()
	}

	// 初始声明（无效项被忽略，客户端可用 DECLARE 获取逐项结果）
	if len(connect.Channels) > 0 {
		cl.replaceChannels(c.filterChannels(identity, connect.Channels))
	}

	pending, delivered, oldest := cl.mb.depth()
	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	connack.SessionPresent = sessionPresent
	connack.AssignedClientID = clientID
	connack.ServerMaxPacketSize = c.options.MaxPacketSize
	connack.SessionID = cl.sessionID.Load()
	connack.Pending = pending
	connack.Delivered = delivered
	connack.OldestUncommitted = oldest

	if err := cn.writePacket(connack); err != nil {
		return
	}

	cn.registered.Store(true)
	c.stats.ClientsConnected.Add(1)
	cl.connectedAt.Store(time.Now().UnixMilli())
	cl.lastSeen.Store(time.Now().UnixMilli())
	cl.expiresAt.Store(0)

	c.options.Hooks.callOnConnect(&ConnectContext{
		ClientID:       clientID,
		RemoteAddr:     cn.remoteAddr,
		SessionPresent: sessionPresent,
	})

	// 接管：先切换连接，再通知旧连接
	if oldConn != nil {
		c.stats.SessionTakeovers.Add(1)
		go oldConn.terminate(packet.ReasonSessionTakenOver, true)
	}

	c.logger.Debug("client connected",
		zap.String("clientID", clientID),
		zap.String("remote", cn.remoteAddr),
		zap.Bool("sessionPresent", sessionPresent))

	err = cn.serve()
	if errors.Is(err, io.EOF) {
		reason = packet.ReasonNormalDisconnect
		if dp := cn.lastDisconnect(); dp != nil {
			reason = dp.ReasonCode
			if dp.Flags&packet.DisconnectFlagSuppressWill != 0 || reason == packet.ReasonNormalDisconnect {
				suppressWill = true
			}
		}
	} else if err != nil {
		reason = packet.ReasonKeepAliveTimeout
	}
}

// attachSession 绑定（或新建）会话，返回会话、是否复用会话、被替换的旧连接。
func (c *Core) attachSession(clientID string, connect *packet.ConnectPacket, cn *conn, identity *Identity) (*Client, bool, *conn) {
	c.registerMu.Lock()
	defer c.registerMu.Unlock()

	c.clientsMu.RLock()
	existing := c.clients[clientID]
	c.clientsMu.RUnlock()

	var (
		cl             *Client
		sessionPresent bool
	)

	if existing != nil {
		cl = existing
		sessionPresent = true
		cl.timeout.Store(effectiveSessionTimeout(c.options, connect.SessionTimeout))
		cl.setIdentity(identity)
	} else {
		cl = newClient(c, clientID, connect)
		cl.setIdentity(identity)
		c.clientsMu.Lock()
		c.clients[clientID] = cl
		c.clientsMu.Unlock()
		c.stats.ClientsTotal.Add(1)
	}

	old := cl.swapConn(cn)
	if old != nil {
		// 接管：更换会话租约标识，旧连接此后无法通过 fencing 检查
		cl.sessionID.Store(newSessionID())
	} else if !sessionPresent {
		cl.sessionID.Store(newSessionID())
	}

	return cl, sessionPresent, old
}

// detachConn 在连接结束时解绑会话。
func (c *Core) detachConn(cl *Client, cn *conn) {
	if cl == nil {
		return
	}

	if cn != nil && cn.registered.Swap(false) {
		c.stats.ClientsConnected.Add(-1)
	}

	if !cl.releaseConn(cn) {
		// 已被新连接接管：不改变会话状态
		return
	}

	cl.markExpiry(time.Now())
	if cl.sessionTimeout() == 0 {
		c.destroySession(cl, packet.ReasonNormalDisconnect)
	}
}

// filterChannels 过滤非法与未授权的声明。
func (c *Core) filterChannels(identity *Identity, decls []packet.ChannelDecl) []packet.ChannelDecl {
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

		if !packet.ValidateChannelFilter(d.Channel) {
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

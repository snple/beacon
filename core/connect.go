package core

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// connackBaseSize 是 CONNACK 除 AssignedClientID 与 Properties 内容外的固定线格式大小
// （固定头 + Flags/ReasonCode + 字符串长度前缀 + SessionID + KeepAlive
// + ReceiveMaximum + Pending + Limits）。用于判断客户端的接收上限
// 能否容纳握手响应。
const connackBaseSize = 5 + 1 + 1 + 2 + 8 + 2 + 2 + 12 + 26

// connackEncodedSize 返回 CONNACK 的完整编码大小。
func connackEncodedSize(clientID string, props map[string]string) int {
	size := connackBaseSize + len(clientID)
	if len(props) > 0 {
		size += packet.StringMapSize(props)
	}
	return size
}

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
	connack.Limits = c.limits()
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
		reason   = packet.ReasonNormalDisconnect
		clientID string
		serveErr error
	)

	defer func() {
		c.untrackHandshakeConn(nc)
		c.releaseHandshakeSlot()
		cn.close()
		// 由连接协程（而不是 waitLoop 自身）等待后台任务退出。
		cn.wait()

		cl := cn.client
		if cl != nil {
			// registered 在 CONNACK 写出成功、OnConnect 即将触发前设置：
			// 从未完成握手的连接（例如在 CONNACK 前就被接管）不应产生
			// 一条没有对应 OnConnect 的断开事件。
			wasConnected := cn.registered.Load()
			c.detachConn(cl, cn)
			if !wasConnected {
				return
			}
			dcErr := serveErr
			if errors.Is(dcErr, io.EOF) {
				dcErr = nil
			}
			c.hooks.callOnDisconnect(&DisconnectContext{
				ClientID:   cl.ID,
				SessionID:  cn.sessionID,
				RemoteAddr: cn.remoteAddr,
				Reason:     reason,
				Err:        dcErr,
				Identity:   cn.identity,
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
		// 不向后兼容：非 v5 的包不做任何协商，直接静默关闭（不发 CONNACK）。
		// 真正的旧版 CONNECT 通常在此之前就已因布局不同而解码失败。
		c.logger.Debug("unsupported protocol version",
			zap.String("remote", cn.remoteAddr), zap.Uint8("version", connect.Version))
		return
	}

	clientID = connect.ClientID
	if clientID == "" {
		clientID = c.generateClientID()
	} else if !packet.ValidateClientID(clientID) {
		c.rejectConn(nc, packet.ReasonClientIDNotValid, "bad client id")
		return
	}

	// 客户端声明的接收上限必须至少能容纳 CONNACK，否则握手响应根本发不出去，
	// 对端只能看到静默断开。0 表示按服务端上限处理。
	if connect.MaxPacketSize > 0 {
		baseProps := maps.Clone(c.options.ServerProperties)
		need := uint32(connackEncodedSize(clientID, baseProps))
		if connect.MaxPacketSize < packet.MinPacketSize || connect.MaxPacketSize < need {
			c.rejectConn(nc, packet.ReasonPacketTooLarge, "client max packet size too small")
			return
		}
	}

	// 认证（单轮）
	var identity *Identity
	authCtx := &AuthContext{
		ClientID:        clientID,
		RemoteAddr:      cn.remoteAddr,
		ProtocolVersion: connect.Version,
		CleanSession:    connect.CleanSession(),
		KeepAlive:       connect.KeepAlive,
		MaxPacketSize:   connect.MaxPacketSize,
		AuthMethod:      connect.AuthMethod,
		AuthData:        connect.AuthData,
		Properties:      maps.Clone(connect.Properties),
	}
	if err := c.hooks.callAuth(authCtx); err != nil {
		reason := packet.ReasonNotAuthorized
		var ae *AuthError
		if errors.As(err, &ae) && ae.Reason.IsError() {
			reason = ae.Reason
		}
		c.rejectConn(nc, reason, "authentication failed")
		return
	}
	identity = authCtx.Identity
	if identity == nil && c.hooks.Auth != nil {
		// 认证通过但没有身份：按“已认证、无任何 channel 权限”处理。
		// 这几乎总是认证器的实现疏漏，显式提示一次。
		c.logger.Warn("auth hook returned nil identity; channel access is denied",
			zap.String("clientID", clientID),
			zap.String("remote", cn.remoteAddr))
	}

	// 合并认证钩子返回的属性，并确认最终 CONNACK 仍能放进客户端声明的上限。
	properties := maps.Clone(c.options.ServerProperties)
	for k, v := range authCtx.ResponseProperties {
		if properties == nil {
			properties = make(map[string]string, len(authCtx.ResponseProperties))
		}
		properties[k] = v
	}
	if connect.MaxPacketSize > 0 && uint32(connackEncodedSize(clientID, properties)) > connect.MaxPacketSize {
		c.rejectConn(nc, packet.ReasonPacketTooLarge, "connack exceeds client max packet size")
		return
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
		go oldConn.terminate(packet.ReasonSessionTakenOver)
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
	// 限制客户端可声明的心跳间隔，避免超长 KeepAlive 长期占用连接名额。
	if max := c.options.MaxKeepAlive; max > 0 && keepAlive > max {
		keepAlive = max
	}
	cn.keepAlive = keepAlive

	outMax := connect.MaxPacketSize
	if outMax == 0 || outMax > c.options.MaxPacketSize {
		outMax = c.options.MaxPacketSize
	}
	cn.maxPacketSize = outMax

	// 未确认可靠消息窗口完全由服务端决定。
	window := c.options.ReceiveMaximum
	// 记录接收上限/窗口，供发布与拉取路径使用。
	// 期间可能已被新连接接管：被接管的旧连接不得再修改会话状态。
	if !cl.setReceiveParamsIfOwner(cn, outMax, uint32(window)) {
		return
	}

	pending, _, _ := cl.mb.depth()
	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	connack.SetSessionPresent(sessionPresent)
	connack.AssignedClientID = clientID
	connack.SessionID = cl.sessionID.Load()
	connack.KeepAlive = keepAlive
	connack.ReceiveMaximum = uint16(window)
	connack.Pending = pending
	connack.Limits = c.limits()
	connack.Properties = properties

	if err := cn.writePacket(connack); err != nil {
		return
	}

	cn.registered.Store(true)
	c.stats.ClientsConnected.Add(1)

	c.hooks.callOnConnect(&ConnectContext{
		ClientID:       clientID,
		SessionID:      cn.sessionID,
		RemoteAddr:     cn.remoteAddr,
		SessionPresent: sessionPresent,
		CleanSession:   connect.CleanSession(),
		Identity:       cn.identity,
		Properties:     maps.Clone(connect.Properties),
	})

	c.logger.Debug("client connected",
		zap.String("clientID", clientID),
		zap.String("remote", cn.remoteAddr),
		zap.Bool("sessionPresent", sessionPresent))

	err = cn.serve()
	serveErr = err
	switch {
	case errors.Is(err, io.EOF):
		reason = packet.ReasonNormalDisconnect
		if dp := cn.lastDisconnect(); dp != nil {
			reason = dp.ReasonCode
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

	// 快照本次连接拿到的租约与身份，供 OnConnect/OnDisconnect 使用：
	// 会话被接管或 CleanSession 重置后，cl.sessionID 与 cl.identity 会变化，
	// 但历史连接的事件仍应带上它自己的那份。
	cn.sessionID = cl.sessionID.Load()
	cn.identity = cl.getIdentity()

	// 恢复会话时按本次认证得到的身份重新校验保留的声明：
	// 上次连接允许的过滤器可能超出本次身份的授权范围。
	cl.replaceChannels(c.filterChannels(identity, cl.declaredChannels()))

	now := time.Now().UnixMilli()
	cl.connectedAt.Store(now)
	cl.lastSeen.Store(now)
	cl.expiresAt.Store(0)

	// 重连后重新开始计算 delivered 未确认超时：离线期间不应被判定为
	// “消费端卡住”，否则恢复的会话会在下一次清理循环里被立刻踢掉。
	if _, delivered, _ := cl.mb.depth(); delivered > 0 {
		cl.deliveredSince.Store(now)
	} else {
		cl.deliveredSince.Store(0)
	}

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

	// 会话转入离线：不可靠消息尽力而为，不保留在离线队列中。
	if n := cl.mb.dropUnreliable(); n > 0 {
		c.stats.MessagesDropped.Add(int64(n))
		cl.dropped.Add(int64(n))
	}

	cl.markExpiry(time.Now())
	// 离线期间不参与 delivered 未确认超时判定；重连时会重新计时。
	cl.deliveredSince.Store(0)
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
	index := make(map[string]int, len(decls))
	for _, d := range decls {
		if pos, dup := index[d.Channel]; dup {
			if pos >= 0 && d.Flags.Unknown() == 0 && !d.TargetOnly() {
				out[pos].SetTargetOnly(false)
			}
			continue
		}

		if d.Flags.Unknown() != 0 {
			index[d.Channel] = -1
			continue
		}
		if !packet.ValidateClientChannelFilter(d.Channel, c.options.MaxChannelLength) {
			index[d.Channel] = -1
			continue
		}
		if !c.canRead(identity, d.Channel) {
			index[d.Channel] = -1
			continue
		}
		index[d.Channel] = len(out)
		out = append(out, d)
	}
	return out
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

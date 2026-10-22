package core

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// HandleConn 处理一个已建立的连接
// 这是 Core 的主要 API，用户可以从任何 net.Listener 获取连接后调用此方法
// 支持 TCP、TLS、WebSocket、QUIC 等任何实现了 net.Conn 接口的连接
func (c *Core) HandleConn(conn net.Conn) error {
	if !c.running.Load() {
		_ = conn.Close()
		return fmt.Errorf("core is not running")
	}

	// 检查连接速率限制
	if c.connLimiter != nil && !c.connLimiter.Allow() {
		c.logger.Warn("Connection rate exceeded, rejecting connection",
			zap.String("remote", conn.RemoteAddr().String()))
		c.sendConnack(conn, false, packet.ReasonConnectionRateExceeded, nil)
		_ = conn.Close()
		return fmt.Errorf("connection rate exceeded")
	}

	// 认证前的并发上限：CONNECT 还没读，MaxClients 还统计不到这些连接。
	// 未限流时每个连接可占用最多 MaxPacketSize 的包缓冲直到 ConnectTimeout，
	// 无限并发等于给出一个未认证的内存/协程耗尽入口。
	if c.handshakeSlots != nil {
		select {
		case c.handshakeSlots <- struct{}{}:
		default:
			c.logger.Warn("Too many concurrent handshakes, rejecting connection",
				zap.String("remote", conn.RemoteAddr().String()))
			c.sendConnack(conn, false, packet.ReasonServerBusy, nil)
			_ = conn.Close()
			return fmt.Errorf("too many concurrent handshakes")
		}
	}

	c.trackHandshakeConn(conn)

	if !c.spawnTask(func() { c.handleConn(conn) }) {
		c.untrackHandshakeConn(conn)
		c.releaseHandshakeSlot()
		_ = conn.Close()
		return ErrCoreNotRunning
	}
	return nil
}

// trackHandshakeConn 记录尚未完成握手的连接，供 Stop 主动关闭。
func (c *Core) trackHandshakeConn(conn net.Conn) {
	c.handshakeConnsMu.Lock()
	if c.handshakeConns == nil {
		c.handshakeConns = make(map[net.Conn]struct{})
	}
	c.handshakeConns[conn] = struct{}{}
	c.handshakeConnsMu.Unlock()
}

// untrackHandshakeConn 连接已完成握手（或被拒绝），不再需要 Stop 兜底关闭。
func (c *Core) untrackHandshakeConn(conn net.Conn) {
	c.handshakeConnsMu.Lock()
	delete(c.handshakeConns, conn)
	c.handshakeConnsMu.Unlock()
}

// releaseHandshakeSlot 释放认证前的并发名额。
func (c *Core) releaseHandshakeSlot() {
	if c.handshakeSlots == nil {
		return
	}
	select {
	case <-c.handshakeSlots:
	default:
	}
}

// closeHandshakeConns 关闭所有仍在握手的连接，避免优雅停机被 ConnectTimeout 拖住。
func (c *Core) closeHandshakeConns() {
	c.handshakeConnsMu.Lock()
	conns := make([]net.Conn, 0, len(c.handshakeConns))
	for conn := range c.handshakeConns {
		conns = append(conns, conn)
	}
	c.handshakeConnsMu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

// 注意：协程的 wg 跟踪由 spawnTask 统一负责，这里不能重复 Done。
func (c *Core) handleConn(conn net.Conn) {
	defer func() {
		c.untrackHandshakeConn(conn)
		c.releaseHandshakeSlot()
	}()

	// 读取并验证 CONNECT 包
	connect, err := c.readConnectPacket(conn)
	if err != nil {
		return
	}

	// 处理 Client ID（分配或使用客户端提供的）
	{
		clientID := connect.ClientID
		if clientID == "" {
			clientID = c.generateClientID()
		}

		// Core 用 "client:<clientID>:" 作为持久化队列前缀；允许 ':' 会让
		// 一个客户端的队列前缀包含另一个客户端的队列（跨会话消息泄露）。
		if !packet.ValidateClientID(clientID) {
			c.logger.Warn("Invalid client ID, rejecting connection",
				zap.String("clientID", clientID),
				zap.String("remote", conn.RemoteAddr().String()))
			c.sendConnack(conn, false, packet.ReasonClientIDNotValid, nil)
			_ = conn.Close()
			return
		}

		connect.ClientID = clientID
	}

	if c.options.MaxClients > 0 && !c.canAcceptClientConnection(connect.ClientID) {
		c.logger.Warn("Max clients reached, rejecting connection",
			zap.String("clientID", connect.ClientID),
			zap.String("remote", conn.RemoteAddr().String()))
		c.sendConnack(conn, false, packet.ReasonServerBusy, nil)
		_ = conn.Close()
		return
	}

	// 认证
	authCtx, ok := c.authClient(conn, connect)
	if !ok {
		// 认证失败时已经发送 CONNACK 并关闭连接
		return
	}

	// 注册客户端并发送 CONNACK
	client, sessionPresent, replacedOnline, oldConns := c.registerClient(conn, connect)

	// Core 在注册过程中停止（与 Stop 的客户端快照互斥）：拒绝本次连接。
	// 否则该连接会在 Stop 快照之后注册，永远不会被关闭，拖住优雅停机。
	if client == nil {
		c.sendConnack(conn, false, packet.ReasonServerBusy, nil)
		_ = conn.Close()
		return
	}

	// 认证身份只保存在内存中（与"认证数据不持久化"的决定一致）。
	if authCtx != nil && authCtx.Identity != nil {
		client.session.setIdentity(authCtx.Identity)
	}

	// 会话元数据在锁外持久化：badger 写入不应发生在全局 clientsMu 临界区内。
	c.saveSession(client, 0)

	// 通过 client.getConn() 读取连接状态。
	// 注意：不能直接访问 client.conn，否则会绕过 connMu 的保护，
	// 与 attachConn 的写入形成 data race（M2）。
	clientConn := client.getConn()
	var (
		keepAlive uint16
		traceID   string
	)
	if clientConn != nil {
		keepAlive = clientConn.keepAlive
		traceID = clientConn.traceID
	}

	// 构建 CONNACK 属性
	connackProp := packet.NewConnackProperties()
	connackProp.SessionTimeout = client.session.timeout.Load()
	connackProp.ClientID = connect.ClientID
	connackProp.KeepAlive = keepAlive
	connackProp.MaxPacketSize = c.options.MaxPacketSize
	connackProp.ReceiveWindow = c.options.ReceiveWindow
	connackProp.TraceID = traceID

	// 如果认证器设置了响应属性，传递给客户端
	if authCtx != nil && len(authCtx.ResponseProperties) > 0 {
		connackProp.UserProperties = authCtx.ResponseProperties
	}

	c.sendConnack(conn, sessionPresent, packet.ReasonSuccess, connackProp)

	// 旧连接已在锁内被 detach（停止处理），这里在锁外真正关闭 socket。
	// 关闭动作不再发送 DISCONNECT：避免慢连接让接管路径等一个写超时。
	for _, oldConn := range oldConns {
		oldConn.closeLocal(packet.ReasonSessionTakenOver)
	}

	// 更新统计信息
	if !replacedOnline {
		client.counted.Store(true)
		c.stats.ClientsConnected.Add(1)
	}
	if !sessionPresent {
		c.stats.ClientsTotal.Add(1)
	}

	c.logger.Info("Client connected",
		zap.String("clientID", client.ID),
		zap.Uint16("keepAlive", keepAlive),
		zap.Uint32("sessionTimeout", client.session.timeout.Load()),
		zap.Bool("sessionPresent", sessionPresent))

	c.emit(Event{
		Type:           EventClientConnected,
		ClientID:       client.ID,
		RemoteAddr:     conn.RemoteAddr().String(),
		SessionPresent: sessionPresent,
	})

	// 调用 OnConnect 钩子
	connectCtx := &ConnectContext{
		ClientID:       connect.ClientID,
		RemoteAddr:     conn.RemoteAddr().String(),
		Packet:         connect,
		SessionPresent: sessionPresent,
	}
	c.options.Hooks.callOnConnect(connectCtx)

	// 开始处理客户端消息。
	// 如果 serve 期间连接已被新连接替换，说明当前连接是被接管的旧连接，
	// 退出时不能再清理当前 Client 对应的会话。
	servedConn := client.serve()
	if servedConn == nil || client.getConn() != servedConn {
		return
	}

	// 客户端断开后处理
	c.handleClientDisconnect(client)
}

func (c *Core) canAcceptClientConnection(clientID string) bool {
	if uint32(c.stats.ClientsConnected.Load()) < c.options.MaxClients {
		return true
	}

	c.clientsMu.RLock()
	existingClient, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	return exists && !existingClient.Closed()
}

// readConnectPacket 读取并验证 CONNECT 包
func (c *Core) readConnectPacket(conn net.Conn) (*packet.ConnectPacket, error) {
	// 设置连接超时
	if c.options.ConnectTimeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(time.Duration(c.options.ConnectTimeout) * time.Second))
	}
	defer func() { _ = conn.SetDeadline(time.Time{}) }() // 清除超时

	// 读取 CONNECT 包
	pkt, size, err := packet.ReadPacketWithSize(conn, c.MaxPacketSize())
	if err != nil {
		c.logger.Debug("Failed to read CONNECT packet", zap.Error(err), zap.String("remote", conn.RemoteAddr().String()))
		_ = conn.Close()
		return nil, err
	}

	c.stats.BytesReceived.Add(int64(size))

	connect, ok := pkt.(*packet.ConnectPacket)
	if !ok {
		c.logger.Debug("First packet is not CONNECT", zap.Uint8("type", uint8(pkt.Type())))
		_ = conn.Close()
		return nil, ErrInvalidConnectPacket
	}

	// 验证协议名
	if connect.ProtocolName != packet.ProtocolName {
		c.logger.Debug("unsupported protocol name",
			zap.String("name", connect.ProtocolName),
			zap.String("expected", packet.ProtocolName))
		c.sendConnack(conn, false, packet.ReasonUnsupportedProtocol, nil)
		_ = conn.Close()

		return nil, ErrUnsupportedProtocol
	}

	// 验证协议版本
	if connect.ProtocolVersion != packet.ProtocolVersion {
		c.logger.Debug("unsupported protocol version",
			zap.Uint8("version", connect.ProtocolVersion),
			zap.Uint8("expected", packet.ProtocolVersion))
		c.sendConnack(conn, false, packet.ReasonUnsupportedProtocol, nil)
		_ = conn.Close()

		return nil, ErrUnsupportedProtocol
	}

	return connect, nil
}

func (c *Core) generateClientID() string {
	return nson.NewId().Hex()
}

// authClient 认证客户端
// 返回认证上下文（包含响应属性）和是否成功
func (c *Core) authClient(conn net.Conn, connect *packet.ConnectPacket) (*AuthConnectContext, bool) {
	if c.options.Hooks.AuthHandler == nil {
		return nil, true
	}

	// 构建认证上下文（直接引用原始 packet）
	authCtx := &AuthConnectContext{
		ClientID:   connect.ClientID,
		RemoteAddr: conn.RemoteAddr().String(),
		Packet:     connect,
	}

	if err := c.options.Hooks.callAuthOnConnect(authCtx); err != nil {
		c.logger.Debug("Authentication failed", zap.String("clientID", connect.ClientID), zap.Error(err))

		// 认证器可以用 *AuthError 指定返回给客户端的原因码（如 ReasonBanned）；
		// 其它错误保持 ReasonNotAuthorized 的历史行为。
		reason := packet.ReasonNotAuthorized
		var authErr *AuthError
		if errors.As(err, &authErr) && authErr.Reason != packet.ReasonSuccess {
			reason = authErr.Reason
		}

		c.sendConnack(conn, false, reason, nil)
		_ = conn.Close()
		return nil, false
	}
	return authCtx, true
}

// registerClient 注册客户端，处理会话恢复和接管
// 返回客户端和 sessionPresent 标志
//
// 核心思路：会话恢复时复用现有 Client 对象，只替换其 Conn
//
// 会话管理逻辑：
// 1. 无旧客户端：创建新 Client
// 2. 有旧客户端 + KeepSession=true：复用旧 Client，替换其 Conn
// 3. 有旧客户端 + KeepSession=false：清理旧 Client，创建新 Client
func (c *Core) registerClient(netConn net.Conn, connect *packet.ConnectPacket) (*Client, bool, bool, []*conn) {
	c.clientsMu.Lock()

	// 停机后不再接受注册：与 Stop 的客户端快照共用同一把锁，
	// 避免出现"快照之后才注册、永远不会被关闭"的连接。
	if !c.running.Load() {
		c.clientsMu.Unlock()
		return nil, false, false, nil
	}

	clientID := connect.ClientID

	existingClient, exists := c.clients[clientID]

	wasOnline := false
	if exists {
		wasOnline = !existingClient.Closed()
	}

	// oldConns 收集需要在锁外关闭的旧连接。
	// 锁内只调用 detach（原子状态切换，无网络 IO），真正的 socket 关闭
	// 由调用方在释放 clientsMu 之后完成——关闭连接会写 DISCONNECT，
	// 在全局锁内等待慢客户端会阻塞所有新连接注册。
	var oldConns []*conn

	// CleanStart：客户端要求全新会话，先丢弃旧会话（订阅 + 待投递消息 + 持久化元数据）
	if exists && connect.CleanStart {
		if wasOnline {
			if oc := existingClient.getConn(); oc != nil {
				oc.detach(packet.ReasonSessionTakenOver)
				oldConns = append(oldConns, oc)
			}
		}
		delete(c.clients, clientID)
		c.cleanupClientWithoutLock(clientID, existingClient)
		// 旧会话记录已被丢弃：同步移除离线会话表，避免留下过期时间戳。
		c.offlineSessionsMu.Lock()
		delete(c.offlineSessions, clientID)
		c.offlineSessionsMu.Unlock()
		existingClient = nil
		exists = false
	}

	// 情况1: 无旧客户端，创建新 Client
	if !exists {
		client := newClient(netConn, connect, c)
		c.clients[clientID] = client
		c.clientsMu.Unlock()
		return client, false, wasOnline, oldConns
	}

	// 从离线会话列表中移除，同时判断会话是否已过期。
	// 清理协程是周期性运行的，会话过期后到被清理之间仍可能收到重连；
	// 此时必须按"会话不存在"处理（SessionPresent=false），
	// 否则客户端会拿到一个本应已失效的会话。
	sessionExpired := false
	c.offlineSessionsMu.Lock()
	// 只对离线会话判过期：在线会话（接管场景）不应因为表里的残留时间戳被误判。
	if expiry, ok := c.offlineSessions[clientID]; ok && existingClient.Closed() && time.Now().After(expiry) {
		sessionExpired = true
	}
	delete(c.offlineSessions, clientID)
	c.offlineSessionsMu.Unlock()

	canResumeSession := !sessionExpired && existingClient.session.timeout.Load() > 0

	// 情况2: 复用旧 Client，只替换 Conn
	if connect.SessionTimeout > 0 && canResumeSession {
		oldConn := existingClient.getConn()

		// 用本次 CONNECT 更新会话参数（超时、认证信息、遗嘱）
		existingClient.applyConnect(connect)

		// 复用 Client，附加新连接
		existingClient.attachConn(netConn, connect)

		// 只关闭旧连接，避免误伤刚替换进去的新连接。
		if wasOnline && oldConn != nil {
			oldConn.detach(packet.ReasonSessionTakenOver)
			oldConns = append(oldConns, oldConn)
		}

		c.logger.Info("Session restored (reusing client, replacing conn)",
			zap.String("clientID", clientID),
			zap.Bool("wasOnline", wasOnline))

		c.clientsMu.Unlock()
		return existingClient, true, wasOnline, oldConns
	}

	// 情况3: SessionTimeout=0 或旧会话不可恢复，清理旧 Client，创建新 Client

	// 踢掉旧连接，因为我们会手动清理旧会话
	if wasOnline {
		if oc := existingClient.getConn(); oc != nil {
			oc.detach(packet.ReasonSessionTakenOver)
			oldConns = append(oldConns, oc)
		}
	}

	// 清理旧资源（在持有锁的情况下调用不需要锁的版本）
	delete(c.clients, clientID)
	c.cleanupClientWithoutLock(clientID, existingClient)

	// 创建新 Client
	client := newClient(netConn, connect, c)
	c.clients[clientID] = client

	c.clientsMu.Unlock()
	return client, false, wasOnline, oldConns
}

// sendConnack 发送 CONNACK 包
func (c *Core) sendConnack(conn net.Conn, sessionPresent bool, code packet.ReasonCode, connackProp *packet.ConnackProperties) {
	// CONNACK 也必须受写超时约束：客户端可能连上后就不再读取。
	if c.options.WriteTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(c.options.WriteTimeout))
		defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()
	}

	connack := packet.NewConnackPacket(code)
	connack.SessionPresent = sessionPresent

	// 设置服务器属性
	connack.Properties.MaxPacketSize = uint32(c.options.MaxPacketSize)

	if connackProp != nil {
		connack.Properties = connackProp
	} else {
		// 错误响应时使用默认 KeepAlive
		connack.Properties.KeepAlive = uint16(packet.DefaultKeepAlive)
	}

	n, _ := packet.WritePacketWithSize(conn, connack, c.options.MaxPacketSize)
	if n > 0 {
		c.stats.BytesSent.Add(int64(n))
	}
}

// handleClientDisconnect 处理客户端断开连接
//
// 处理逻辑：
// 1. session.timeout=0: 立即清理会话
// 2. session.timeout>0: 保留会话，记录过期时间，等待重连或过期清理
// 3. 正常断开（收到 DISCONNECT 包）：清除遗嘱消息，不发送
// 4. 异常断开（没有 DISCONNECT 包）：发布遗嘱消息
func (c *Core) handleClientDisconnect(client *Client) {
	// 如果同一 ClientID 已经被新的 Client 实例接管，这里只能结束旧连接的生命周期，
	// 不能再修改当前 map 中的新会话状态。
	c.clientsMu.RLock()
	currentClient, exists := c.clients[client.ID]
	c.clientsMu.RUnlock()
	if exists && currentClient != client {
		// 会话接管（同一 ClientID 的新连接替换旧连接）视为客户端主动行为：
		// 不发布旧连接的遗嘱，也不修改新会话的任何状态。
		client.touch()
		return
	}

	c.uncountClient(client)
	client.touch()

	// 连接层原因与底层错误：管理面据此区分"心跳超时/被踢/接管/关机"。
	closeReason := packet.ReasonNormalDisconnect
	var closeErr error
	if conn := client.getConn(); conn != nil {
		closeReason = conn.closeReasonCode()
		closeErr = conn.err()
	}

	// 调用 OnDisconnect 钩子
	// Packet 可能为 nil（连接异常断开时）
	disconnectCtx := &DisconnectContext{
		ClientID: client.ID,
		Packet:   client.DisconnectPacket(), // 可能为 nil
		Duration: time.Now().Unix() - client.ConnectedAt().Unix(),
		Reason:   closeReason,
		Err:      closeErr,
	}
	c.options.Hooks.callOnDisconnect(disconnectCtx)

	var connDuration time.Duration
	if connectedAt := client.ConnectedAt(); !connectedAt.IsZero() {
		connDuration = time.Since(connectedAt)
	}

	c.emit(Event{
		Type:       EventClientDisconnected,
		ClientID:   client.ID,
		RemoteAddr: connAddress(client.getConn()),
		Reason:     closeReason.String(),
		Duration:   connDuration,
	})

	// 决定是否发布遗嘱：
	//  1. 收到 DISCONNECT 且原因码为 DisconnectWithWill → 客户端显式要求发布
	//  2. 收到其它 DISCONNECT → 正常断开，清除遗嘱
	//  3. 没收到 DISCONNECT：服务端主动关闭（如停机）不发布；其它异常断开发布
	isNormalDisconnect := client.DisconnectPacket() != nil
	disconnectWithWill := isNormalDisconnect &&
		client.DisconnectPacket().ReasonCode == packet.ReasonDisconnectWithWill
	serverShutdown := false
	if conn := client.getConn(); conn != nil {
		serverShutdown = conn.closeReasonCode() == packet.ReasonServerShuttingDown
	}

	publishWill := disconnectWithWill || (!isNormalDisconnect && !serverShutdown)
	if !publishWill {
		client.clearWill()
	}

	// 检查是否需要保留会话
	if client.session.timeout.Load() == 0 {
		// 立即清理
		if publishWill {
			client.publishWill()
		}

		c.clientsMu.Lock()
		delete(c.clients, client.ID)
		c.clientsMu.Unlock()

		// 清理客户端相关资源
		c.cleanupClient(client.ID, client)
		return
	}

	// 保留会话，记录过期时间
	expiryTime := time.Now().Add(time.Duration(client.session.timeout.Load()) * time.Second)

	// 先持久化会话（订阅 + 过期时间），再纳入离线会话表：
	// 若达到 MaxOfflineSessions 上限，addOfflineSession 会驱逐最早过期的
	// 会话（包括可能刚加入的这一个），并一并删除其记录与队列。
	c.saveSessionExpiring(client, expiryTime)
	c.addOfflineSession(client.ID, expiryTime)

	// 需要时发布遗嘱消息（publishWill 内部会清除遗嘱，防止重复发送）
	if publishWill {
		client.publishWill()
	}

	c.logger.Info("Client disconnected",
		zap.String("clientID", client.ID),
		zap.Uint32("sessionTimeout", client.session.timeout.Load()),
		zap.String("expiresAt", expiryTime.Format(time.RFC3339)),
		zap.Bool("sessionPreserved", c.isOfflineSession(client.ID)))
}

// uncountClient 把客户端从在线计数中移除（恰好一次）。
func (c *Core) uncountClient(client *Client) {
	if client != nil && client.counted.CompareAndSwap(true, false) {
		c.stats.ClientsConnected.Add(-1)
	}
}

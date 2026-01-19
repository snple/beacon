package core

import (
	"crypto/tls"
	"fmt"
	"maps"
	"net"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// startListener 启动网络监听器
func (c *Core) startListener() (net.Listener, error) {
	var listener net.Listener
	var err error

	if c.options.TLSConfig != nil {
		listener, err = tls.Listen("tcp", c.options.Address, c.options.TLSConfig)
		c.logger.Info("Starting core with TLS", zap.String("address", c.options.Address))
	} else {
		listener, err = net.Listen("tcp", c.options.Address)
		c.logger.Warn("Starting core without TLS (not recommended for production)",
			zap.String("address", c.options.Address))
	}

	if err != nil {
		return nil, fmt.Errorf("failed to start listener: %w", err)
	}
	return listener, nil
}

func (c *Core) accept() {
	defer c.wg.Done()

	for c.running.Load() {
		conn, err := c.listener.Accept()
		if err != nil {
			if c.running.Load() {
				c.logger.Error("Accept error", zap.Error(err))
			}
			continue
		}

		// 检查客户端数量限制
		if c.options.MaxClients > 0 && int(c.stats.ClientsConnected.Load()) >= c.options.MaxClients {
			c.logger.Warn("Max clients reached, rejecting connection")
			conn.Close()
			continue
		}

		c.wg.Add(1)
		go c.handleConn(conn)
	}
}

func (c *Core) handleConn(conn net.Conn) {
	defer c.wg.Done()

	// 读取并验证 CONNECT 包
	connect, err := c.readConnectPacket(conn)
	if err != nil {
		return
	}

	// 处理 Client ID（分配或使用客户端提供的）
	assignedClientID := c.handleClientID(connect)

	// 认证
	authCtx, ok := c.authClient(conn, connect)
	if !ok {
		// 认证失败时已经发送 CONNACK 并关闭连接
		return
	}

	// 处理 KeepAlive
	keepAlive := c.handleKeepAlive(connect)

	// 注册客户端并发送 CONNACK
	client, sessionPresent := c.registerClient(conn, connect)

	// 构建 CONNACK 属性
	connackProp := packet.NewConnackProperties()
	connackProp.ClientID = assignedClientID
	connackProp.KeepAlive = &keepAlive
	connackProp.SessionExpiry = &client.SessionExpiry
	receiveWindow := uint16(c.options.ReceiveWindow)
	connackProp.ReceiveWindow = &receiveWindow

	// 如果认证器设置了响应属性，传递给客户端
	if authCtx != nil && len(authCtx.ResponseProperties) > 0 {
		connackProp.UserProperties = authCtx.ResponseProperties
	}

	c.sendConnack(conn, sessionPresent, packet.ReasonSuccess, connackProp)

	// 更新统计信息
	c.stats.ClientsConnected.Add(1)
	if !sessionPresent {
		c.stats.ClientsTotal.Add(1)
	}

	c.logger.Info("Client connected",
		zap.String("clientID", connect.ClientID),
		zap.Bool("KeepSession", connect.KeepSession),
		zap.Uint16("keepAlive", keepAlive),
		zap.Uint32("sessionExpiry", client.SessionExpiry),
		zap.Bool("sessionPresent", sessionPresent))

	// 调用 OnConnect 钩子
	connectCtx := &ConnectContext{
		ClientID:       connect.ClientID,
		RemoteAddr:     conn.RemoteAddr().String(),
		Packet:         connect,
		SessionPresent: sessionPresent,
	}
	c.options.Hooks.callOnConnect(connectCtx)

	// 开始处理客户端消息
	client.Serve()

	// 客户端断开后处理
	c.handleClientDisconnect(client)
}

// readConnectPacket 读取并验证 CONNECT 包
func (c *Core) readConnectPacket(conn net.Conn) (*packet.ConnectPacket, error) {
	// 设置连接超时
	if c.options.ConnectTimeout > 0 {
		conn.SetDeadline(time.Now().Add(time.Duration(c.options.ConnectTimeout) * time.Second))
	}
	defer conn.SetDeadline(time.Time{}) // 清除超时

	// 读取 CONNECT 包
	pkt, err := packet.ReadPacket(conn)
	if err != nil {
		c.logger.Debug("Failed to read CONNECT packet", zap.Error(err), zap.String("remote", conn.RemoteAddr().String()))
		conn.Close()
		return nil, err
	}

	connect, ok := pkt.(*packet.ConnectPacket)
	if !ok {
		c.logger.Debug("First packet is not CONNECT", zap.Uint8("type", uint8(pkt.Type())))
		conn.Close()
		return nil, ErrInvalidConnectPacket
	}

	// 验证协议名
	if connect.ProtocolName != packet.ProtocolName {
		c.logger.Debug("unsupported protocol name",
			zap.String("name", connect.ProtocolName),
			zap.String("expected", packet.ProtocolName))
		c.sendConnack(conn, false, packet.ReasonUnsupportedProtocol, nil)
		conn.Close()

		return nil, ErrUnsupportedProtocol
	}

	// 验证协议版本
	if connect.ProtocolVersion != packet.ProtocolVersion {
		c.logger.Debug("unsupported protocol version",
			zap.Uint8("version", connect.ProtocolVersion),
			zap.Uint8("expected", packet.ProtocolVersion))
		c.sendConnack(conn, false, packet.ReasonUnsupportedProtocol, nil)
		conn.Close()

		return nil, ErrUnsupportedProtocol
	}

	return connect, nil
}

// handleClientID 处理 Client ID，如果为空则分配
func (c *Core) handleClientID(connect *packet.ConnectPacket) string {
	if connect.ClientID != "" {
		return "" // 客户端提供了 ID
	}
	assignedID := generateClientID()
	connect.ClientID = assignedID
	return assignedID
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
		c.sendConnack(conn, false, packet.ReasonNotAuthorized, nil)
		conn.Close()
		return nil, false
	}
	return authCtx, true
}

// handleKeepAlive 处理 KeepAlive 时间
func (c *Core) handleKeepAlive(connect *packet.ConnectPacket) uint16 {
	keepAlive := connect.KeepAlive
	if c.options.KeepAlive > 0 && (keepAlive == 0 || c.options.KeepAlive < keepAlive) {
		keepAlive = c.options.KeepAlive
	}
	connect.KeepAlive = keepAlive

	return keepAlive
}

// registerClient 注册新客户端，处理会话恢复和接管
// 返回新客户端和 sessionPresent 标志
//
// 会话管理逻辑：
// 1. 无旧客户端：创建新会话
// 2. 旧客户端在线：踢掉旧连接，根据 KeepSession 决定是否保留会话
// 3. 旧客户端离线：根据 KeepSession 决定是否保留会话
func (c *Core) registerClient(conn net.Conn, connect *packet.ConnectPacket) (*Client, bool) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	clientID := connect.ClientID

	// 创建客户端
	newClient := NewClient(conn, connect, c)

	// 检查是否有旧客户端
	existingClient, exists := c.clients[clientID]

	// 无旧客户端，直接注册新客户端并返回
	if !exists {
		c.clients[clientID] = newClient
		return newClient, false
	}

	// 有旧客户端，检查其是否在线
	isOldOnline := !existingClient.IsClosed()

	// 从离线会话列表中移除旧客户端
	c.offlineSessionsMu.Lock()
	delete(c.offlineSessions, clientID)
	c.offlineSessionsMu.Unlock()

	if connect.KeepSession {
		// KeepSession=true: 恢复会话
		// 先恢复会话数据，再替换客户端，最后踢掉旧连接
		newClient.restoreSession(existingClient)
		c.clients[clientID] = newClient

		// 如果旧客户端在线，踢掉它（标记为已接管，防止清理资源）
		if isOldOnline {
			existingClient.takenOver.Store(true)
			existingClient.Close(packet.ReasonSessionTakenOver)
		}

		c.logger.Info("Session restored",
			zap.String("clientID", clientID),
			zap.Bool("wasOnline", isOldOnline))

		return newClient, true
	} else {
		// KeepSession=false: 不恢复会话，清理旧数据

		// 先从 clients map 删除旧客户端（防止其断开时误清理）
		delete(c.clients, clientID)

		// 如果旧客户端在线，踢掉它（标记为已接管，防止清理资源）
		if isOldOnline {
			existingClient.takenOver.Store(true)
			existingClient.Close(packet.ReasonSessionTakenOver)
		}

		// 清理旧客户端相关资源
		c.cleanupClient(clientID)

		c.clients[clientID] = newClient
		return newClient, false
	}
}

// sendConnack 发送 CONNACK 包
func (c *Core) sendConnack(conn net.Conn, sessionPresent bool, code packet.ReasonCode, connackProp *packet.ConnackProperties) {
	connack := packet.NewConnackPacket(code)
	connack.SessionPresent = sessionPresent

	// 设置服务器属性
	maxPacketSize := uint32(c.options.MaxPacketSize)
	connack.Properties.MaxPacketSize = &maxPacketSize

	if connackProp != nil {
		connack.Properties = connackProp
	} else {
		// 错误响应时使用默认 KeepAlive
		keepAlive := uint16(packet.DefaultKeepAlive)
		connack.Properties.KeepAlive = &keepAlive
	}

	packet.WritePacket(conn, connack)
}

// handleClientDisconnect 处理客户端断开连接
//
// 处理逻辑：
// 1. SessionExpiry=0: 立即清理会话
// 2. SessionExpiry>0: 保留会话，记录过期时间，等待重连或过期清理
// 3. 正常断开（收到 DISCONNECT 包）：清除遗嘱消息，不发送
// 4. 异常断开（没有 DISCONNECT 包）：发布遗嘱消息
func (c *Core) handleClientDisconnect(client *Client) {
	c.stats.ClientsConnected.Add(-1)

	// 如果客户端已被接管，不做任何清理（资源已由新客户端接管）
	if client.takenOver.Load() {
		c.logger.Debug("Client disconnected (session taken over, skipping cleanup)",
			zap.String("clientID", client.ID))
		return
	}

	// 调用 OnDisconnect 钩子
	// Packet 可能为 nil（连接异常断开时）
	disconnectCtx := &DisconnectContext{
		ClientID: client.ID,
		Packet:   client.DisconnectPacket, // 可能为 nil
		Duration: time.Now().Unix() - client.ConnectedAt.Unix(),
	}
	c.options.Hooks.callOnDisconnect(disconnectCtx)

	// 检查是否为正常断开
	isNormalDisconnect := client.DisconnectPacket != nil

	// 正常断开：清除遗嘱消息
	if isNormalDisconnect {
		client.clearWill()
	}

	// 检查是否需要保留会话
	if client.SessionExpiry == 0 {
		// 立即清理
		// 异常断开时发布遗嘱消息
		if !isNormalDisconnect {
			client.publishWill()
		}
		c.removeClient(client)
		return
	}

	// 保留会话，记录过期时间
	expiryTime := time.Now().Add(time.Duration(client.SessionExpiry) * time.Second)

	c.offlineSessionsMu.Lock()
	c.offlineSessions[client.ID] = expiryTime
	c.offlineSessionsMu.Unlock()

	// 异常断开时发布遗嘱消息
	if !isNormalDisconnect {
		client.publishWill()
	}

	c.logger.Info("Client disconnected, session preserved",
		zap.String("clientID", client.ID),
		zap.Uint32("sessionExpiry", client.SessionExpiry),
		zap.String("expiresAt", expiryTime.Format(time.RFC3339)))
}

func (c *Core) removeClient(client *Client) {
	c.clientsMu.Lock()
	isCurrentClient := false
	if existing, ok := c.clients[client.ID]; ok && existing == client {
		delete(c.clients, client.ID)
		isCurrentClient = true
	}
	c.clientsMu.Unlock()

	// 只有确认是当前客户端时才清理资源（防止误删新客户端的资源）
	if !isCurrentClient {
		c.logger.Debug("Client already replaced, skipping cleanup",
			zap.String("clientID", client.ID))
		return
	}

	// 清理客户端相关资源
	c.cleanupClient(client.ID)

	// 注意：ClientsConnected 已在 handleClientDisconnect 中扣减，这里不再重复扣减

	c.logger.Info("Client session removed", zap.String("clientID", client.ID))
}

// restoreSession 从旧客户端恢复会话数据
// 由 core 在 registerClient 时同步调用，无需异步通道
func (c *Client) restoreSession(old *Client) {
	// 迁移订阅
	old.subsMu.RLock()
	c.subsMu.Lock()
	maps.Copy(c.subscriptions, old.subscriptions)
	c.subsMu.Unlock()
	old.subsMu.RUnlock()

	// 更新 core 订阅树（指向新客户端）
	c.subsMu.RLock()
	for topic, opts := range c.subscriptions {
		c.core.subTree.subscribe(c.ID, topic, opts.QoS)
	}
	c.subsMu.RUnlock()

	// 迁移 pendingAck（等待确认的消息）
	old.pendingAckMu.Lock()
	c.pendingAckMu.Lock()
	maps.Copy(c.pendingAck, old.pendingAck)
	old.pendingAck = make(map[uint16]pendingMessage)
	c.pendingAckMu.Unlock()
	old.pendingAckMu.Unlock()

	// 迁移 sendQueue（尽力而为，包括 QoS 0 和 QoS 1 未发送的消息）
	// 从旧队列中取出所有消息，放入新队列
	migratedCount := 0
	droppedCount := 0
	for {
		msg, ok := old.sendQueue.tryDequeue()
		if !ok {
			break // 队列已空
		}
		// 尝试放入新队列，如果新队列满了则停止迁移
		if c.sendQueue.tryEnqueue(msg) {
			migratedCount++
		} else {
			// 新队列已满，剩余消息需要处理
			// QoS 1 的消息如果已持久化，会在重传时重新加载
			// 对于未持久化或 QoS 0 的消息，只能丢弃
			droppedCount++
			c.core.logger.Warn("Message dropped during sendQueue migration (queue full)",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic),
				zap.Uint8("qos", uint8(msg.Packet.QoS)))
		}
	}

	// 如果有消息被丢弃，更新统计
	if droppedCount > 0 {
		c.core.stats.MessagesDropped.Add(int64(droppedCount))
	}

	// 迁移 packetID
	c.nextPacketID.Store(old.nextPacketID.Load())

	c.core.logger.Debug("Session restored from previous client",
		zap.String("clientID", c.ID),
		zap.Int("subscriptions", len(c.subscriptions)),
		zap.Int("pendingAck", len(c.pendingAck)),
		zap.Int("sendQueueMigrated", migratedCount),
		zap.Int("sendQueueDropped", droppedCount))
}

// HasSession 检查客户端是否有需要保留的会话
func (c *Client) HasSession() bool {
	c.subsMu.RLock()
	hasSubs := len(c.subscriptions) > 0
	c.subsMu.RUnlock()

	c.pendingAckMu.Lock()
	hasPending := len(c.pendingAck) > 0
	c.pendingAckMu.Unlock()

	return hasSubs || hasPending
}

// GetSessionInfo 获取会话信息快照（用于调试）
func (c *Client) GetSessionInfo() (subscriptions int, pendingAck int) {
	c.subsMu.RLock()
	subscriptions = len(c.subscriptions)
	c.subsMu.RUnlock()

	c.pendingAckMu.Lock()
	pendingAck = len(c.pendingAck)
	c.pendingAckMu.Unlock()

	return subscriptions, pendingAck
}

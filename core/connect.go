package core

import (
	"crypto/tls"
	"fmt"
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
		if c.options.MaxClients > 0 && uint32(c.stats.ClientsConnected.Load()) >= c.options.MaxClients {
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
	clientID := connect.ClientID
	if clientID == "" {
		clientID = c.generateClientID()
	}

	// 认证
	authCtx, ok := c.authClient(conn, connect)
	if !ok {
		// 认证失败时已经发送 CONNACK 并关闭连接
		return
	}

	// 注册客户端并发送 CONNACK
	client, sessionPresent := c.registerClient(clientID, conn, connect)

	// 构建 CONNACK 属性
	connackProp := packet.NewConnackProperties()
	connackProp.SessionTimeout = client.session.timeout
	connackProp.ClientID = clientID
	connackProp.KeepAlive = client.conn.KeepAlive
	connackProp.MaxPacketSize = c.options.MaxPacketSize
	connackProp.ReceiveWindow = c.options.ReceiveWindow
	connackProp.TraceID = client.conn.TraceID

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
		zap.String("clientID", client.ID),
		zap.Uint16("keepAlive", client.conn.KeepAlive),
		zap.Bool("keepSession", client.session.keep),
		zap.Uint32("sessionTimeout", client.session.timeout),
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

func (c *Core) generateClientID() string {
	return fmt.Sprintf("beacon-%d", time.Now().UnixNano())
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

// registerClient 注册客户端，处理会话恢复和接管
// 返回客户端和 sessionPresent 标志
//
// 核心思路：会话恢复时复用现有 Client 对象，只替换其 Conn
//
// 会话管理逻辑：
// 1. 无旧客户端：创建新 Client
// 2. 有旧客户端 + KeepSession=true：复用旧 Client，替换其 Conn
// 3. 有旧客户端 + KeepSession=false：清理旧 Client，创建新 Client
func (c *Core) registerClient(clientID string, netConn net.Conn, connect *packet.ConnectPacket) (*Client, bool) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	existingClient, exists := c.clients[clientID]

	// 情况1: 无旧客户端，创建新 Client
	if !exists {
		client := newClient(netConn, connect, c)
		c.clients[clientID] = client
		return client, false
	}

	// 从离线会话列表中移除
	c.offlineSessionsMu.Lock()
	delete(c.offlineSessions, clientID)
	c.offlineSessionsMu.Unlock()

	// 检查旧客户端是否在线
	wasOnline := !existingClient.IsClosed()

	// 情况2: KeepSession=true，复用旧 Client，只替换 Conn
	if connect.KeepSession {
		// 先踢掉旧连接（如果在线），cleanup=false 表示会话被接管
		if wasOnline {
			existingClient.Close(packet.ReasonSessionTakenOver, false)
		}

		// 复用 Client，附加新连接
		existingClient.attachConn(netConn, connect)

		// 迁移旧 sendQueue 中的消息到新 Conn 的 sendQueue
		existingClient.migrateSendQueue()

		c.logger.Info("Session restored (reusing client, replacing conn)",
			zap.String("clientID", clientID),
			zap.Bool("wasOnline", wasOnline))

		return existingClient, true
	}

	// 情况3: KeepSession=false，清理旧 Client，创建新 Client

	// 踢掉旧连接，cleanup=false 因为我们会手动清理
	if wasOnline {
		existingClient.Close(packet.ReasonSessionTakenOver, false)
	}

	// 清理旧资源
	c.removeClient(existingClient)

	// 创建新 Client
	client := newClient(netConn, connect, c)
	c.clients[clientID] = client

	return client, false
}

// sendConnack 发送 CONNACK 包
func (c *Core) sendConnack(conn net.Conn, sessionPresent bool, code packet.ReasonCode, connackProp *packet.ConnackProperties) {
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

	// 如果 cleanup=false，不做任何清理（会话已被接管或由调用方负责清理）
	if !client.conn.cleanup.Load() {
		c.logger.Debug("Client disconnected (skipping cleanup)",
			zap.String("clientID", client.ID))
		return
	}

	// 调用 OnDisconnect 钩子
	// Packet 可能为 nil（连接异常断开时）
	disconnectCtx := &DisconnectContext{
		ClientID: client.ID,
		Packet:   client.DisconnectPacket(), // 可能为 nil
		Duration: time.Now().Unix() - client.ConnectedAt().Unix(),
	}
	c.options.Hooks.callOnDisconnect(disconnectCtx)

	// 检查是否为正常断开
	isNormalDisconnect := client.DisconnectPacket() != nil

	// 正常断开：清除遗嘱消息
	if isNormalDisconnect {
		client.clearWill()
	}

	// 检查是否需要保留会话
	if client.session.timeout == 0 {
		// 立即清理
		// 异常断开时发布遗嘱消息
		if !isNormalDisconnect {
			client.publishWill()
		}
		c.removeClient(client)
		return
	}

	// 保留会话，记录过期时间
	expiryTime := time.Now().Add(time.Duration(client.session.timeout) * time.Second)

	c.offlineSessionsMu.Lock()
	c.offlineSessions[client.ID] = expiryTime
	c.offlineSessionsMu.Unlock()

	// 异常断开时发布遗嘱消息
	if !isNormalDisconnect {
		client.publishWill()
	}

	c.logger.Info("Client disconnected, session preserved",
		zap.String("clientID", client.ID),
		zap.Uint32("sessionTimeout", client.session.timeout),
		zap.String("expiresAt", expiryTime.Format(time.RFC3339)))
}

func (c *Core) removeClient(client *Client) {
	c.clientsMu.Lock()
	if existing, ok := c.clients[client.ID]; ok && existing == client {
		delete(c.clients, client.ID)
	}
	c.clientsMu.Unlock()

	// 清理客户端相关资源
	c.cleanupClient(client.ID)

	// 注意：ClientsConnected 已在 handleClientDisconnect 中扣减，这里不再重复扣减

	c.logger.Info("Client session removed", zap.String("clientID", client.ID))
}

// migrateSendQueue 迁移旧连接的 sendQueue 到新连接
// 在会话恢复（attachConn）后调用
func (c *Client) migrateSendQueue() {
	c.connMu.RLock()
	newConn := c.conn
	oldConn := c.oldConn
	c.connMu.RUnlock()

	if oldConn == nil || newConn == nil {
		return
	}

	migratedCount := 0
	droppedCount := 0

	for {
		msg, ok := oldConn.sendQueue.tryDequeue()
		if !ok {
			break // 队列已空
		}
		if newConn.sendQueue.tryEnqueue(msg) {
			migratedCount++
		} else {
			// 新队列已满，QoS1 消息会在重传时重新加载
			droppedCount++
			c.core.logger.Warn("Message dropped during sendQueue migration",
				zap.String("clientID", c.ID),
				zap.String("topic", msg.Packet.Topic))
		}
	}

	if droppedCount > 0 {
		c.core.stats.MessagesDropped.Add(int64(droppedCount))
	}

	// 清除旧连接引用
	c.connMu.Lock()
	c.oldConn = nil
	c.connMu.Unlock()

	if migratedCount > 0 || droppedCount > 0 {
		c.core.logger.Debug("SendQueue migrated",
			zap.String("clientID", c.ID),
			zap.Int("migrated", migratedCount),
			zap.Int("dropped", droppedCount))
	}
}

// HasSession 检查客户端是否有需要保留的会话
func (c *Client) HasSession() bool {
	c.session.subsMu.RLock()
	hasSubs := len(c.session.subscriptions) > 0
	c.session.subsMu.RUnlock()

	c.session.pendingAckMu.Lock()
	hasPending := len(c.session.pendingAck) > 0
	c.session.pendingAckMu.Unlock()

	return hasSubs || hasPending
}

// GetSessionInfo 获取会话信息快照（用于调试）
func (c *Client) GetSessionInfo() (subscriptions int, pendingAck int) {
	c.session.subsMu.RLock()
	subscriptions = len(c.session.subscriptions)
	c.session.subsMu.RUnlock()

	c.session.pendingAckMu.Lock()
	pendingAck = len(c.session.pendingAck)
	c.session.pendingAckMu.Unlock()

	return subscriptions, pendingAck
}

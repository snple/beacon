package core

import (
	"fmt"
	"net"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// HandleConn 处理一个已建立的连接
// 这是 Core 的主要 API，用户可以从任何 net.Listener 获取连接后调用此方法
// 支持 TCP、TLS、WebSocket、QUIC 等任何实现了 net.Conn 接口的连接
func (c *Core) HandleConn(conn net.Conn) error {
	if !c.running.Load() {
		conn.Close()
		return fmt.Errorf("core is not running")
	}

	// 检查客户端数量限制
	if c.options.MaxClients > 0 && uint32(c.stats.ClientsConnected.Load()) >= c.options.MaxClients {
		c.logger.Warn("Max clients reached, rejecting connection",
			zap.String("remote", conn.RemoteAddr().String()))
		conn.Close()
		return fmt.Errorf("max clients reached")
	}

	c.wg.Add(1)
	go c.handleConn(conn)
	return nil
}

func (c *Core) handleConn(conn net.Conn) {
	defer c.wg.Done()

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
		connect.ClientID = clientID
	}

	// 认证
	authCtx, ok := c.authClient(conn, connect)
	if !ok {
		// 认证失败时已经发送 CONNACK 并关闭连接
		return
	}

	// 注册客户端并发送 CONNACK
	client, sessionPresent := c.registerClient(conn, connect)

	// 构建 CONNACK 属性
	connackProp := packet.NewConnackProperties()
	connackProp.SessionTimeout = client.session.timeout
	connackProp.ClientID = connect.ClientID
	connackProp.KeepAlive = client.conn.keepAlive
	connackProp.MaxPacketSize = c.options.MaxPacketSize
	connackProp.ReceiveWindow = c.options.ReceiveWindow
	connackProp.TraceID = client.conn.traceID

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
		zap.Uint16("keepAlive", client.conn.keepAlive),
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
	client.serve()

	if client.skipHandle.Load() {
		// 会话被接管，跳过断开处理
		return
	}

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
	pkt, err := packet.ReadPacket(conn, c.MaxPacketSize())
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
func (c *Core) registerClient(netConn net.Conn, connect *packet.ConnectPacket) (*Client, bool) {
	c.clientsMu.Lock()
	defer c.clientsMu.Unlock()

	clientID := connect.ClientID

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
	wasOnline := !existingClient.Closed()

	// 情况2: SessionTimeout>0，复用旧 Client，只替换 Conn
	if connect.SessionTimeout > 0 {
		// 先踢掉旧连接（如果在线），cleanup=false 表示会话被接管
		if wasOnline {
			existingClient.closeAndSkipHandle(packet.ReasonSessionTakenOver)
		}

		// 复用 Client，附加新连接
		existingClient.attachConn(netConn, connect)

		c.logger.Info("Session restored (reusing client, replacing conn)",
			zap.String("clientID", clientID),
			zap.Bool("wasOnline", wasOnline))

		return existingClient, true
	}

	// 情况3: SessionTimeout=0，清理旧 Client，创建新 Client

	// 踢掉旧连接，cleanup=false 因为我们会手动清理
	if wasOnline {
		existingClient.closeAndSkipHandle(packet.ReasonSessionTakenOver)
	}

	// 清理旧资源（在持有锁的情况下调用不需要锁的版本）
	delete(c.clients, clientID)
	c.cleanupClientWithoutLock(clientID, existingClient)

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

	packet.WritePacket(conn, connack, c.options.MaxPacketSize)
}

// handleClientDisconnect 处理客户端断开连接
//
// 处理逻辑：
// 1. session.timeout=0: 立即清理会话
// 2. session.timeout>0: 保留会话，记录过期时间，等待重连或过期清理
// 3. 正常断开（收到 DISCONNECT 包）：清除遗嘱消息，不发送
// 4. 异常断开（没有 DISCONNECT 包）：发布遗嘱消息
func (c *Core) handleClientDisconnect(client *Client) {
	c.stats.ClientsConnected.Add(-1)

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

		c.clientsMu.Lock()
		delete(c.clients, client.ID)
		c.clientsMu.Unlock()

		// 清理客户端相关资源
		c.cleanupClient(client.ID)
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

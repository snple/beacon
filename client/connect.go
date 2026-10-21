package client

import (
	"net"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// WillMessage 遗嘱消息（连接异常断开时由 core 代发）
type WillMessage struct {
	Packet *packet.PublishPacket

	Expiry time.Duration // 相对过期时间（如 30*time.Second），0 表示不过期
}

// ConnectWithDialer 使用自定义 Dialer 连接到 core
// 用户可以实现 Dialer 接口来支持自定义传输层（WebSocket、QUIC 等）
// 支持自动重连（重连时会调用 dialer.Dial()）
func (c *Client) ConnectWithDialer(dialer Dialer) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	// 保存 dialer 用于重连
	c.connMu.Lock()
	c.dialer = dialer
	c.connMu.Unlock()

	// 建立连接
	netConn, err := dialer.Dial()
	if err != nil {
		return err
	}

	return c.connectWithConn(netConn)
}

// ConnectWithConn 使用已建立的连接
// 注意：此方法会禁用自动重连，因为客户端不知道如何重新建立连接
// 如果需要自动重连，请使用 ConnectWithDialer
func (c *Client) ConnectWithConn(conn net.Conn) error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	// 清除 dialer，禁用自动重连
	c.connMu.Lock()
	c.dialer = nil
	c.connMu.Unlock()

	// 禁用自动重连
	c.autoReconnect.Store(false)

	return c.connectWithConn(conn)
}

// connectWithConn 内部方法：使用已建立的连接完成握手和初始化
func (c *Client) connectWithConn(netConn net.Conn) error {
	// 获取旧连接（用于状态迁移）
	c.connMu.Lock()
	oldConn := c.conn
	c.conn = nil
	c.connMu.Unlock()

	// 等待之前的连接完全关闭
	if oldConn != nil {
		oldConn.wait()
	}

	// 握手：发送 CONNECT，接收 CONNACK
	connack, err := c.handshake(netConn)
	if err != nil {
		netConn.Close()
		return err
	}

	// 客户端可能已被 Close()（例如自动重连拨号期间用户主动关闭）。
	// 此时不应再启动新连接，否则会泄漏网络连接与对应的 goroutine（M3）。
	// 注意：底层 Dial 本身无法被 ctx 中断（Dialer 接口无 context 参数），
	// 但拨号成功后在此处检查 ctx 可避免泄漏——新连接不会被创建。
	if c.ctx.Err() != nil {
		netConn.Close()
		return ErrClientClosed
	}

	// 创建新的 Connection 对象（从旧连接迁移状态）
	conn := newConn(c, netConn, connack)

	// 设置为当前连接
	c.connMu.Lock()
	c.conn = conn
	// 如果有 dialer，启用自动重连
	if c.dialer != nil {
		c.autoReconnect.Store(true)
	}
	c.connMu.Unlock()

	c.connected.Store(true)

	// 启动连接的所有协程
	conn.start()

	// 启动重传协程（仅第一次连接时启动）
	// 使用独立的 retransmitLoopStarted 标志，避免与 TriggerRetransmit 的
	// retransmitting 一次性守卫冲突（M6：冲突会导致 retransmitLoop 永不启动）
	if c.queue != nil {
		if c.retransmitLoopStarted.CompareAndSwap(false, true) {
			// 通过 spawnTask 纳入 wg：Close 会在 wg.Wait 之后才关闭存储
			if !c.spawnTask(c.retransmitLoop) {
				// 客户端正在关闭：允许下次连接重试启动
				c.retransmitLoopStarted.Store(false)
			}
		} else {
			// 循环已在运行，手动触发一次重传
			c.TriggerRetransmit()
		}
	}

	// 调用 OnConnect 钩子
	c.options.Hooks.callOnConnect(&ConnectContext{
		ClientID:       conn.clientID,
		SessionPresent: conn.sessionPresent,
		Packet:         connack,
	})

	return nil
}

// handshake 执行 CONNECT/CONNACK 握手
func (c *Client) handshake(conn net.Conn) (*packet.ConnackPacket, error) {
	// 构建 CONNECT 包
	connect := packet.NewConnectPacket()
	connect.ClientID = c.options.ClientID
	connect.KeepAlive = c.options.KeepAlive
	connect.SessionTimeout = c.options.SessionTimeout
	connect.CleanStart = c.options.CleanStart

	connect.Properties.AuthMethod = c.options.AuthMethod
	connect.Properties.AuthData = c.options.AuthData
	connect.Properties.MaxPacketSize = c.options.MaxPacketSize
	connect.Properties.ReceiveWindow = c.options.ReceiveWindow
	connect.Properties.TraceID = c.options.TraceID
	connect.Properties.UserProperties = c.options.UserProperties

	// 遗嘱消息
	if c.options.Will != nil && c.options.Will.Packet != nil {
		willPktCopy := c.options.Will.Packet.DeepCopy()
		willPkt := &willPktCopy

		if willPkt.Topic == "" {
			return nil, ErrWillTopicRequired
		}
		if !packet.ValidateTopicName(willPkt.Topic) {
			return nil, ErrWillTopicInvalid
		}
		if !willPkt.QoS.IsValid() {
			return nil, ErrWillQoSInvalid
		}
		connect.Will = true

		if willPkt.Properties == nil {
			willPkt.Properties = packet.NewPublishProperties()
		}

		if c.options.Will.Expiry > 0 {
			// 遗嘱包中的 ExpiryTime 表示相对 TTL（秒），由服务端在遗嘱实际发布时开始计算
			willPkt.Properties.ExpiryTime = int64(c.options.Will.Expiry / time.Second)
		}

		connect.WillPacket = willPkt
	}

	// 发送 CONNECT
	if err := packet.WritePacket(conn, connect, c.options.MaxPacketSize); err != nil {
		return nil, NewConnectionError("send CONNECT", err)
	}

	// 接收 CONNACK
	conn.SetReadDeadline(time.Now().Add(c.options.ConnectTimeout))
	pkt, err := packet.ReadPacket(conn, c.options.MaxPacketSize) // CONNACK 时不知道限制，传 0
	if err != nil {
		return nil, NewConnectionError("read CONNACK", err)
	}
	conn.SetReadDeadline(time.Time{})

	connack, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		return nil, NewUnexpectedPacketError("CONNACK", pkt.Type().String())
	}

	if connack.ReasonCode != packet.ReasonSuccess {
		return nil, NewConnectionRefusedError(connack.ReasonCode.String())
	}

	return connack, nil
}

// Disconnect 断开连接
func (c *Client) Disconnect() error {
	return c.DisconnectWithReason(packet.ReasonNormalDisconnect)
}

// DisconnectWithReason 带原因断开连接
func (c *Client) DisconnectWithReason(reason packet.ReasonCode) error {
	// 禁用自动重连（用户主动断开）
	c.autoReconnect.Store(false)

	if !c.connected.Load() {
		return nil
	}

	// 发送 DISCONNECT
	disconnect := packet.NewDisconnectPacket(reason)
	if err := c.writePacket(disconnect); err != nil {
		c.logger.Error("Failed to send DISCONNECT packet", zap.Error(err))
	}

	return c.close(nil)
}

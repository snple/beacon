package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/snple/beacon/packet"
)

// WillMessage 遗嘱消息（连接异常断开时由 core 代发）
type WillMessage struct {
	Packet *packet.PublishPacket

	Expiry time.Duration // 相对过期时间（如 30*time.Second），0 表示不过期
}

// Connect 连接到 core
func (c *Client) Connect() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	// 获取旧连接（用于状态迁移）
	c.connMu.Lock()
	oldConn := c.conn
	c.conn = nil
	c.connMu.Unlock()

	// 等待之前的连接完全关闭
	if oldConn != nil {
		oldConn.wait()
	}

	// 建立 TCP 连接
	netConn, err := c.dialTCP()
	if err != nil {
		return err
	}

	reader := bufio.NewReader(netConn)
	writer := bufio.NewWriter(netConn)

	// 握手：发送 CONNECT，接收 CONNACK
	connack, err := c.handshake(netConn, reader, writer)
	if err != nil {
		netConn.Close()
		return err
	}

	// 提取连接参数
	clientID := c.options.ClientID
	if connack.Properties.ClientID != "" {
		clientID = connack.Properties.ClientID
	}

	keepAlive := c.options.KeepAlive
	if connack.Properties.KeepAlive != 0 {
		keepAlive = connack.Properties.KeepAlive
	}

	sendWindow := uint16(defaultReceiveWindow)
	if connack.Properties.ReceiveWindow != 0 {
		sendWindow = connack.Properties.ReceiveWindow
	}

	sessionPresent := connack.SessionPresent

	// 创建新的 Connection 对象（从旧连接迁移状态）
	conn := newConn(c, oldConn, netConn, clientID, keepAlive, sessionPresent, sendWindow, c.options.KeepSession)

	// 设置为当前连接
	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.connected.Store(true)
	c.autoReconnect.Store(true)

	// 启动连接的所有协程
	conn.start()

	// 启动重传协程（仅第一次连接时启动）
	if c.store != nil {
		// 检查重传协程是否已在运行
		if c.retransmitting.CompareAndSwap(false, true) {
			go c.retransmitLoop()
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

// dialTCP 建立 TCP 连接
func (c *Client) dialTCP() (net.Conn, error) {
	address := c.options.Core
	useTLS := false
	if len(address) > 6 && address[:6] == "tls://" {
		address = address[6:]
		useTLS = true
	}

	ctx, cancel := context.WithTimeout(c.rootCtx, c.options.ConnectTimeout)
	defer cancel()

	var conn net.Conn
	var err error

	if useTLS || c.options.TLSConfig != nil {
		tlsConfig := c.options.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		conn, err = (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
	}

	if err != nil {
		return nil, NewConnectionError("connect", err)
	}

	return conn, nil
}

// handshake 执行 CONNECT/CONNACK 握手
func (c *Client) handshake(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer) (*packet.ConnackPacket, error) {
	// 构建 CONNECT 包
	connect := packet.NewConnectPacket()
	connect.ClientID = c.options.ClientID
	connect.KeepAlive = c.options.KeepAlive
	connect.KeepSession = c.options.KeepSession
	connect.Properties.AuthMethod = c.options.AuthMethod
	connect.Properties.AuthData = c.options.AuthData
	connect.Properties.TraceID = c.options.TraceID
	connect.Properties.UserProperties = c.options.UserProperties
	connect.Properties.SessionTimeout = c.options.SessionTimeout
	connect.Properties.ReceiveWindow = uint16(defaultReceiveWindow)

	// 遗嘱消息
	if c.options.Will != nil && c.options.Will.Packet != nil {
		willPkt := c.options.Will.Packet

		if willPkt.Topic == "" {
			return nil, ErrWillTopicRequired
		}
		connect.Will = true

		if c.options.Will.Expiry > 0 {
			willPkt.Properties.ExpiryTime = time.Now().Unix() + int64(c.options.Will.Expiry.Seconds())
		}

		connect.WillPacket = willPkt
	}

	// 发送 CONNECT
	if err := packet.WritePacket(writer, connect); err != nil {
		return nil, NewConnectionError("send CONNECT", err)
	}
	if err := writer.Flush(); err != nil {
		return nil, NewConnectionError("send CONNECT", err)
	}

	// 接收 CONNACK
	conn.SetReadDeadline(time.Now().Add(c.options.ConnectTimeout))
	pkt, err := packet.ReadPacket(reader)
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
	_ = c.writePacket(disconnect)

	return c.close(nil)
}

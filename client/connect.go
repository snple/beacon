package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// Connect 连接到 core
func (c *Client) Connect() error {
	c.connectMu.Lock()
	defer c.connectMu.Unlock()

	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	// 等待上一轮连接相关协程退出，避免复用同一个 Client 时产生并发读写。
	c.connWG.Wait()

	// 解析地址
	address := c.options.Core
	useTLS := false
	if len(address) > 6 && address[:6] == "tls://" {
		address = address[6:]
		useTLS = true
	}

	// 建立连接
	var conn net.Conn
	var err error

	ctx, cancel := context.WithTimeout(c.rootCtx, c.options.ConnectTimeout)
	defer cancel()

	dialer := &net.Dialer{}
	if useTLS || c.options.TLSConfig != nil {
		tlsConfig := c.options.TLSConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		}
		conn, err = (&tls.Dialer{Config: tlsConfig}).DialContext(ctx, "tcp", address)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}

	if err != nil {
		return NewConnectionError("connect", err)
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// 发送 CONNECT
	connect := packet.NewConnectPacket()
	connect.ClientID = c.options.ClientID
	connect.KeepAlive = c.options.KeepAlive
	connect.Flags.CleanSession = c.options.CleanSession
	connect.Properties.AuthMethod = c.options.AuthMethod
	connect.Properties.AuthData = c.options.AuthData
	connect.Properties.TraceID = c.options.TraceID
	connect.Properties.UserProperties = c.options.UserProperties
	if c.options.SessionExpiry > 0 {
		connect.Properties.SessionExpiry = &c.options.SessionExpiry
	}

	// 客户端接收窗口
	receiveWindow := uint16(defaultReceiveWindow)
	connect.Properties.ReceiveWindow = &receiveWindow

	// 遗嘱消息
	if c.options.Will != nil {
		if c.options.Will.Topic == "" {
			conn.Close()
			return ErrWillTopicRequired
		}
		connect.Flags.Will = true

		willPkt := packet.NewPublishPacket(c.options.Will.Topic, c.options.Will.Payload)
		willPkt.QoS = c.options.Will.QoS
		willPkt.Retain = c.options.Will.Retain

		// Will 属性
		if willPkt.Properties == nil {
			willPkt.Properties = packet.NewPublishProperties()
		}
		if c.options.Will.Priority != nil {
			willPkt.Properties.Priority = c.options.Will.Priority
		}
		willPkt.Properties.TraceID = c.options.Will.TraceID
		willPkt.Properties.ContentType = c.options.Will.ContentType
		willPkt.Properties.TargetClientID = c.options.Will.TargetClientID
		willPkt.Properties.ResponseTopic = c.options.Will.ResponseTopic
		willPkt.Properties.CorrelationData = c.options.Will.CorrelationData
		if c.options.Will.Expiry > 0 {
			willPkt.Properties.ExpiryTime = time.Now().Unix() + int64(c.options.Will.Expiry.Seconds())
		}
		if len(c.options.Will.UserProperties) > 0 {
			willPkt.Properties.UserProperties = c.options.Will.UserProperties
		}

		connect.WillPacket = willPkt
	}

	if err := packet.WritePacket(writer, connect); err != nil {
		conn.Close()
		return NewConnectionError("send CONNECT", err)
	}
	if err := writer.Flush(); err != nil {
		conn.Close()
		return NewConnectionError("send CONNECT", err)
	}

	// 等待 CONNACK
	conn.SetReadDeadline(time.Now().Add(c.options.ConnectTimeout))
	pkt, err := packet.ReadPacket(reader)
	if err != nil {
		conn.Close()
		return NewConnectionError("read CONNACK", err)
	}

	connack, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		conn.Close()
		return NewUnexpectedPacketError("CONNACK", pkt.Type().String())
	}

	if connack.ReasonCode != packet.ReasonSuccess {
		conn.Close()
		return NewConnectionRefusedError(connack.ReasonCode.String())
	}
	conn.SetReadDeadline(time.Time{})

	// 握手成功后，再切换 Client 的连接状态（避免 Connect 失败导致内部状态被污染）。
	c.closeOnce = sync.Once{}
	c.connCtx, c.connCancel = context.WithCancel(c.rootCtx)

	c.connMu.Lock()
	c.conn = conn
	c.reader = reader
	c.writer = writer
	c.connMu.Unlock()

	// 保存 sessionPresent 状态（服务端是否恢复了会话）
	c.sessionPresent = connack.SessionPresent

	// CleanSession=true 表示不恢复旧会话，客户端侧也应丢弃未确认的持久化 QoS1 消息。
	if c.options.CleanSession && c.store != nil {
		if err := c.store.Clear(); err != nil {
			c.logger.Warn("Failed to clear persisted messages for clean session", zap.Error(err))
		}
		c.pendingAckMu.Lock()
		clear(c.storedPacketIDs)
		c.pendingAckMu.Unlock()
	}

	// 保存分配的 Client ID
	c.clientID = c.options.ClientID
	if connack.Properties.ClientID != "" {
		c.clientID = connack.Properties.ClientID
	}

	// 使用服务器返回的 KeepAlive（服务器会选择最合适的值）
	if connack.Properties.KeepAlive != nil {
		c.keepAlive = *connack.Properties.KeepAlive
	} else {
		c.keepAlive = c.options.KeepAlive
	}

	// 从 CONNACK 中读取 core 的初始接收窗口（客户端的发送窗口）
	if connack.Properties.ReceiveWindow != nil {
		c.sendWindow = *connack.Properties.ReceiveWindow
	} else {
		c.sendWindow = defaultReceiveWindow
	}

	// 初始化发送队列
	if c.sendQueue == nil {
		c.sendQueue = NewSendQueue(int(c.sendWindow))
	} else {
		c.sendQueue.UpdateCapacity(int(c.sendWindow))
	}

	c.connected.Store(true)

	// 初始化 PacketID 序列。
	// 对嵌入式设备来说，遍历所有持久化消息可能非常昂贵；这里改为从 store 的元数据读取种子值（O(1)）。
	seed := uint16(minPacketID)
	if c.store != nil {
		if s, err := c.store.PacketIDSeed(); err == nil {
			seed = s
		} else {
			c.logger.Warn("Failed to load packet id seed from store", zap.Error(err))
		}
	}
	c.nextPacketID.Store(uint32(seed))

	// 启用自动重连（只要 Connect 成功一次就启用）
	c.autoReconnect.Store(true)

	// 启动接收协程
	c.connWG.Add(1)
	go c.receiveLoop()

	// 启动心跳协程
	c.connWG.Add(1)
	go c.keepAliveLoop()

	// 启动发送队列处理协程（如果启用了流控）
	if c.sendQueue != nil {
		c.logger.Debug("Send queue initialized",
			zap.Int("capacity", c.sendQueue.Capacity()))
	}

	// 启动重传协程（用于重传持久化的 QoS1 消息）
	if c.store != nil {
		c.connWG.Add(1)
		go c.retransmitLoop()
	}

	// 调用 OnConnect 钩子
	c.options.Hooks.callOnConnect(&ConnectContext{
		ClientID:       c.clientID,
		SessionPresent: c.sessionPresent,
		Packet:         connack,
	})

	return nil
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

	// 发送 DISCONNECT（忽略错误，因为连接可能已经断开）
	disconnect := packet.NewDisconnectPacket(reason)
	_ = c.writePacket(disconnect)

	return c.close(nil)
}

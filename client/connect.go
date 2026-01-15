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
	if c.connected.Load() {
		return ErrAlreadyConnected
	}

	// 如果之前已经连接过（cancel 不为 nil），等待旧的 goroutines 完全退出
	// 这确保在重连时不会与旧的 goroutines 产生竞争
	if c.cancel != nil {
		c.wg.Wait()
	}

	// 重置状态（支持重连）
	c.closeOnce = sync.Once{}
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.pendingAck = make(map[uint16]chan error)
	c.pendingRequests = make(map[uint32]chan *Response)

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

	ctx, cancel := context.WithTimeout(c.ctx, c.options.ConnectTimeout)
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

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

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

	if err := c.writePacket(connect); err != nil {
		conn.Close()
		return NewConnectionError("send CONNECT", err)
	}

	// 等待 CONNACK
	conn.SetReadDeadline(time.Now().Add(c.options.ConnectTimeout))
	pkt, err := packet.ReadPacket(c.reader)
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

	// 保存 sessionPresent 状态（服务端是否恢复了会话）
	c.sessionPresent = connack.SessionPresent

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
	c.nextPacketID.Store(minPacketID)

	// 启用自动重连（只要 Connect 成功一次就启用）
	c.autoReconnect.Store(true)

	// 启动接收协程
	c.wg.Add(1)
	go c.receiveLoop()

	// 启动心跳协程
	c.wg.Add(1)
	go c.keepAliveLoop()

	// 启动发送队列处理协程（如果启用了流控）
	if c.sendQueue != nil {
		c.logger.Debug("Send queue initialized",
			zap.Int("capacity", c.sendQueue.Capacity()))
	}

	// 启动重传协程（用于重传持久化的 QoS1 消息）
	if c.store != nil {
		c.wg.Add(1)
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
		// 即使未连接，也要等待重连协程完成
		c.reconnectWG.Wait()
		return nil
	}

	// 发送 DISCONNECT（忽略错误，因为连接可能已经断开）
	disconnect := packet.NewDisconnectPacket(reason)
	_ = c.writePacket(disconnect)

	err := c.close(nil)

	// 等待重连协程完成
	c.reconnectWG.Wait()

	return err
}

// reconnect 自动重连逻辑（使用指数退避策略）
func (c *Client) reconnect() {
	defer c.reconnectWG.Done()

	c.reconnecting.Store(true)
	defer c.reconnecting.Store(false)

	// 指数退避参数
	initialDelay := 1 * time.Second
	maxDelay := 60 * time.Second
	currentDelay := initialDelay

	// 保存订阅信息以便重连后恢复
	c.subscribedTopicsMu.RLock()
	savedTopics := make([]string, 0, len(c.subscribedTopics))
	for topic := range c.subscribedTopics {
		savedTopics = append(savedTopics, topic)
	}
	c.subscribedTopicsMu.RUnlock()

	c.logger.Info("Starting automatic reconnection",
		zap.Duration("initialDelay", initialDelay),
		zap.Duration("maxDelay", maxDelay))

	for {
		// 检查是否应该停止重连
		if !c.autoReconnect.Load() {
			c.logger.Info("Automatic reconnection disabled, stopping")
			return
		}

		// 等待退避时间
		c.logger.Info("Waiting before reconnection attempt",
			zap.Duration("delay", currentDelay))
		time.Sleep(currentDelay)

		// 如果在等待期间 autoReconnect 被禁用，停止重连
		if !c.autoReconnect.Load() {
			c.logger.Info("Automatic reconnection disabled during backoff, stopping")
			return
		}

		c.logger.Info("Attempting to reconnect...")

		// 临时禁用 autoReconnect，防止 Connect 失败时触发嵌套重连
		c.autoReconnect.Store(false)

		// 尝试重新连接
		err := c.Connect()
		if err != nil {
			c.logger.Warn("Reconnection failed",
				zap.Error(err),
				zap.Duration("nextRetry", currentDelay*2))

			// 重新启用 autoReconnect 以便继续重试
			c.autoReconnect.Store(true)

			// 增加退避时间（指数退避）
			currentDelay *= 2
			if currentDelay > maxDelay {
				currentDelay = maxDelay
			}
			continue
		}

		// 重连成功，Connect() 内部已设置 autoReconnect = true
		c.logger.Info("Reconnection successful",
			zap.Bool("sessionPresent", c.sessionPresent))

		// 根据 sessionPresent 决定是否需要重新订阅/注册
		// sessionPresent=true 表示服务端已恢复会话（订阅和 action 注册都保留了）
		// sessionPresent=false 表示服务端没有旧会话，需要重新订阅/注册
		if c.sessionPresent {
			c.logger.Info("Session restored by server, skipping re-subscribe/re-register")

			// 虽然服务端保留了订阅，但客户端仍需恢复本地订阅记录
			c.subscribedTopicsMu.Lock()
			for _, topic := range savedTopics {
				c.subscribedTopics[topic] = true
			}
			c.subscribedTopicsMu.Unlock()
		} else {
			// 服务端没有会话，需要重新订阅
			if len(savedTopics) > 0 {
				c.logger.Info("Restoring subscriptions",
					zap.Int("count", len(savedTopics)))
				if err := c.Subscribe(savedTopics...); err != nil {
					c.logger.Warn("Failed to restore subscriptions", zap.Error(err))
				} else {
					c.logger.Info("Subscriptions restored successfully")
				}
			}

			// 重新注册已注册的 actions
			c.actionsMu.RLock()
			actions := make([]string, 0, len(c.registeredActions))
			for action := range c.registeredActions {
				actions = append(actions, action)
			}
			c.actionsMu.RUnlock()

			if len(actions) > 0 {
				c.logger.Info("Re-registering actions",
					zap.Int("count", len(actions)))
				if err := c.RegisterMultiple(actions); err != nil {
					c.logger.Warn("Failed to re-register actions", zap.Error(err))
				} else {
					c.logger.Info("Actions re-registered successfully")
				}
			}
		}

		// 重连成功，退出重连循环
		return
	}
}

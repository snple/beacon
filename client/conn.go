package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// Conn 表示单次 TCP 连接
// 负责管理底层网络连接、读写协程、心跳、包确认等连接级别的状态
type Conn struct {
	// 所属的客户端（会话管理器）
	client *Client

	// TCP 连接
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
	connMu sync.RWMutex

	// 连接会话状态（每次连接可能不同）
	clientID       string // 实际使用的客户端 ID（可能由服务器分配）
	keepAlive      uint16 // 实际使用的心跳间隔
	sessionPresent bool   // 服务端是否恢复了会话
	sendWindow     uint16 // 发送窗口大小

	// 发送队列（连接级别，允许离线缓存）
	sendQueue  *sendQueue
	processing atomic.Bool

	// 等待确认的包（连接级别，断开后失效）
	pendingAck   map[nson.Id]chan error
	pendingAckMu sync.Mutex

	// 心跳 RTT
	pingSeq      atomic.Uint32
	lastRTTNanos atomic.Int64
	lastPingSeq  atomic.Uint32
	lastPingSent atomic.Int64

	// 生命周期
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	closed    atomic.Bool
	closeOnce sync.Once
	writeMu   sync.Mutex

	logger *zap.Logger
}

// newConn 创建新的连接对象，从旧连接迁移状态
func newConn(c *Client, oldConn *Conn, netConn net.Conn, clientID string, keepAlive uint16, sessionPresent bool, sendWindow uint16, keepSession bool) *Conn {
	ctx, cancel := context.WithCancel(c.rootCtx)

	conn := &Conn{
		client:         c,
		conn:           netConn,
		reader:         bufio.NewReader(netConn),
		writer:         bufio.NewWriter(netConn),
		clientID:       clientID,
		keepAlive:      keepAlive,
		sessionPresent: sessionPresent,
		sendWindow:     sendWindow,
		pendingAck:     make(map[nson.Id]chan error),
		ctx:            ctx,
		cancel:         cancel,
		logger:         c.logger,
	}

	// 从旧连接迁移或初始化状态
	if oldConn != nil && keepSession {
		sendQueue := newSendQueue(int(sendWindow))

		// 迁移 sendQueue（尽力而为，包括 QoS 0 和 QoS 1 未发送的消息）
		// 从旧队列中取出所有消息，放入新队列
		migratedCount := 0
		for {
			msg, ok := oldConn.sendQueue.tryDequeue()
			if !ok {
				break // 队列已空
			}
			// 尝试放入新队列，如果新队列满了则停止迁移
			if sendQueue.tryEnqueue(msg) {
				migratedCount++
			} else {
				// 新队列已满，剩余消息丢弃
				// QoS 1 的消息会在持久化存储中，后续会被重传机制处理
				c.logger.Debug("New sendQueue full during migration, remaining messages not migrated",
					zap.Int("migrated", migratedCount))
				break
			}
		}

		conn.sendQueue = sendQueue

		c.logger.Debug("Migrated sendQueue from old connection",
			zap.Int("migrated", migratedCount))
	} else {
		// 初始化新的发送队列
		conn.sendQueue = newSendQueue(int(sendWindow))

		// 清空持久化存储
		if !keepSession && c.store != nil {
			if err := c.store.clear(); err != nil {
				c.logger.Warn("Failed to clear persisted messages for clean session", zap.Error(err))
			}
		}
	}

	return conn
}

// start 启动连接的所有协程
func (conn *Conn) start() {
	// 启动接收协程
	conn.wg.Add(1)
	go conn.receiveLoop()

	// 启动心跳协程
	conn.wg.Add(1)
	go conn.keepAliveLoop()

	// 触发消息发送（从 sendQueue 发送消息）
	conn.client.triggerSend()
}

// receiveLoop 接收消息循环
func (conn *Conn) receiveLoop() {
	defer conn.wg.Done()

	for !conn.closed.Load() {
		// 设置读取超时
		timeout := time.Duration(conn.keepAlive) * time.Second * 2
		conn.conn.SetReadDeadline(time.Now().Add(timeout))

		pkt, err := packet.ReadPacket(conn.reader)
		if err != nil {
			if !conn.closed.Load() {
				if errors.Is(err, io.EOF) {
					conn.close(nil)
				} else {
					conn.close(err)
				}
			}
			return
		}

		conn.handlePacket(pkt)
	}
}

// keepAliveLoop 心跳循环
func (conn *Conn) keepAliveLoop() {
	defer conn.wg.Done()

	interval := time.Duration(conn.keepAlive) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-conn.ctx.Done():
			return
		case <-ticker.C:
			if conn.closed.Load() {
				return
			}

			now := time.Now().UnixNano()
			seq := conn.pingSeq.Add(1)
			conn.lastPingSeq.Store(seq)
			conn.lastPingSent.Store(now)

			ping := packet.NewPingPacket(seq, uint64(now))
			if err := conn.writePacket(ping); err != nil {
				conn.close(err)
				return
			}
		}
	}
}

// handlePacket 处理收到的包
func (conn *Conn) handlePacket(pkt packet.Packet) {
	switch p := pkt.(type) {
	case *packet.PublishPacket:
		conn.handlePublish(p)
	case *packet.PubackPacket:
		conn.handlePuback(p)
	case *packet.SubackPacket:
		conn.handleSuback(p)
	case *packet.UnsubackPacket:
		conn.handleUnsuback(p)
	case *packet.PongPacket:
		conn.handlePong(p)
	case *packet.DisconnectPacket:
		conn.close(NewServerDisconnectError(p.ReasonCode.String()))
	case *packet.RequestPacket:
		conn.client.handleRequest(p)
	case *packet.ResponsePacket:
		conn.client.handleResponse(p)
	case *packet.RegackPacket:
		conn.client.handleRegack(p)
	case *packet.UnregackPacket:
		conn.client.handleUnregack(p)
	case *packet.AuthPacket:
		conn.client.handleAuth(p)
	case *packet.TracePacket:
		conn.client.handleTrace(p)
	}
}

// handlePublish 处理 PUBLISH 包
func (conn *Conn) handlePublish(p *packet.PublishPacket) {
	// 调用钩子
	if !conn.client.options.Hooks.callOnPublish(&PublishContext{
		ClientID: conn.clientID,
		Packet:   p,
	}) {
		conn.logger.Debug("Message rejected by hook", zap.String("topic", p.Topic))
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			conn.writePacket(puback)
		}
		return
	}

	msg := &Message{Packet: p}

	// 非阻塞放入消息队列
	select {
	case conn.client.messageQueue <- msg:
		conn.logger.Debug("Enqueued message for polling", zap.String("topic", p.Topic))
		if p.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(p.PacketID, packet.ReasonSuccess)
			conn.writePacket(puback)
		}
	default:
		conn.logger.Warn("Message queue full, message dropped", zap.String("topic", p.Topic))
	}
}

// handlePuback 处理 PUBACK
func (conn *Conn) handlePuback(p *packet.PubackPacket) {
	var err error
	if p.ReasonCode != packet.ReasonSuccess {
		err = NewPublishWarningError(p.ReasonCode.String())
	}

	// 从持久化删除
	if conn.client.store != nil {
		if delErr := conn.client.store.Delete(p.PacketID); delErr != nil {
			conn.logger.Warn("Failed to delete persisted message after ACK",
				zap.String("packetID", p.PacketID.Hex()),
				zap.Error(delErr))
		}
	}

	conn.handleAck(p.PacketID, err)
}

// handleSuback 处理 SUBACK
func (conn *Conn) handleSuback(p *packet.SubackPacket) {
	var err error
	for i, code := range p.ReasonCodes {
		if code != packet.ReasonSuccess && code != packet.ReasonCode(packet.QoS0) &&
			code != packet.ReasonCode(packet.QoS1) {
			err = NewSubscriptionError(i, code.String())
			break
		}
	}
	conn.handleAck(p.PacketID, err)
}

// handleUnsuback 处理 UNSUBACK
func (conn *Conn) handleUnsuback(p *packet.UnsubackPacket) {
	var err error
	for i, code := range p.ReasonCodes {
		if code != packet.ReasonSuccess {
			err = NewUnsubscriptionError(i, code.String())
			break
		}
	}
	conn.handleAck(p.PacketID, err)
}

// handlePong 处理 PONG
func (conn *Conn) handlePong(p *packet.PongPacket) {
	now := time.Now().UnixNano()
	if p.Seq != 0 && p.Seq == conn.lastPingSeq.Load() {
		sent := conn.lastPingSent.Load()
		if sent > 0 && now > sent {
			conn.lastRTTNanos.Store(now - sent)
			return
		}
	}
	if p.Echo > 0 && uint64(now) > p.Echo {
		conn.lastRTTNanos.Store(int64(uint64(now) - p.Echo))
	}
}

// handleAck 处理确认响应
func (conn *Conn) handleAck(packetID nson.Id, err error) {
	conn.pendingAckMu.Lock()
	ch, ok := conn.pendingAck[packetID]
	if ok {
		delete(conn.pendingAck, packetID)
	}
	conn.pendingAckMu.Unlock()

	if ok {
		ch <- err
		close(ch)
	}
}

// writePacket 发送数据包
func (conn *Conn) writePacket(pkt packet.Packet) error {
	if conn.closed.Load() {
		return ErrNotConnected
	}

	conn.writeMu.Lock()
	defer conn.writeMu.Unlock()

	conn.connMu.RLock()
	w := conn.writer
	conn.connMu.RUnlock()

	if w == nil {
		return ErrNotConnected
	}

	if err := packet.WritePacket(w, pkt); err != nil {
		return err
	}
	return w.Flush()
}

// sendMessage 发送消息（从 sendQueue 调用）
func (conn *Conn) sendMessage(msg *Message) error {
	return conn.writePacket(msg.Packet)
}

// waitAck 等待包确认
func (conn *Conn) waitAck(packetID nson.Id, timeout time.Duration) error {
	ch := make(chan error, 1)
	conn.pendingAckMu.Lock()
	conn.pendingAck[packetID] = ch
	conn.pendingAckMu.Unlock()

	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		conn.pendingAckMu.Lock()
		delete(conn.pendingAck, packetID)
		conn.pendingAckMu.Unlock()
		return ErrPublishTimeout
	case <-conn.ctx.Done():
		return ErrNotConnected
	}
}

// close 关闭连接
func (conn *Conn) close(err error) {
	conn.closeOnce.Do(func() {
		conn.closed.Store(true)
		if conn.cancel != nil {
			conn.cancel()
		}

		// 关闭网络连接
		conn.connMu.RLock()
		netConn := conn.conn
		conn.connMu.RUnlock()
		if netConn != nil {
			netConn.Close()
		}

		// 清理等待中的确认
		conn.pendingAckMu.Lock()
		for _, ch := range conn.pendingAck {
			select {
			case ch <- errors.New("conn closed"):
			default:
			}
		}
		conn.pendingAck = make(map[nson.Id]chan error)
		conn.pendingAckMu.Unlock()

		// 通知客户端连接已断开
		conn.client.onConnLost(err)
	})
}

// wait 等待连接关闭
func (conn *Conn) wait() {
	conn.wg.Wait()
}

// IsClosed 检查是否已关闭
func (conn *Conn) IsClosed() bool {
	return conn.closed.Load()
}

// LastRTT 返回最近一次 RTT
func (conn *Conn) LastRTT() time.Duration {
	ns := conn.lastRTTNanos.Load()
	if ns <= 0 {
		return 0
	}
	return time.Duration(ns)
}

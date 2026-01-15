package stream

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/stream/packet"

	"go.uber.org/zap"
)

// Core
type Core struct {
	address     string
	listener    net.Listener
	authHandler AuthHandler
	logger      *zap.Logger

	// 等待连接的客户端 (clientID -> waiting connection)
	waitingMu    sync.RWMutex
	waitingConns map[string][]*WaitingConn

	// 活跃的流会话
	streamsMu sync.RWMutex
	streams   map[uint32]*StreamSession

	streamIDGen atomic.Uint32

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	started atomic.Bool
}

// WaitingConn 等待中的连接
type WaitingConn struct {
	conn      net.Conn
	clientID  string
	timestamp time.Time
	takenCh   chan struct{} // 用于通知连接被取走
	closeCh   chan struct{} // 用于通知连接被清理/过期
}

// AuthHandler 认证处理器
type AuthHandler func(clientID, authMethod string, authData []byte) error

// NewCore 创建 Core
//
// 必填：
// - address（WithAddress）
func NewCore(opts ...CoreOption) (*Core, error) {
	ctx, cancel := context.WithCancel(context.Background())
	// 创建默认的 zap logger
	logger, _ := zap.NewDevelopment()
	b := &Core{
		address:      "",
		authHandler:  func(clientID, authMethod string, authData []byte) error { return nil },
		logger:       logger,
		waitingConns: make(map[string][]*WaitingConn),
		streams:      make(map[uint32]*StreamSession),
		ctx:          ctx,
		cancel:       cancel,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(b)
		}
	}

	if b.logger == nil {
		logger, _ := zap.NewDevelopment()
		b.logger = logger
	}
	if b.address == "" {
		cancel()
		return nil, fmt.Errorf("address is required")
	}
	if b.authHandler == nil {
		b.authHandler = func(clientID, authMethod string, authData []byte) error { return nil }
	}

	return b, nil
}

// Start 启动Core
func (b *Core) Start() error {
	if b.started.Swap(true) {
		return fmt.Errorf("core already started")
	}

	listener, err := net.Listen("tcp", b.address)
	if err != nil {
		b.started.Store(false)
		return fmt.Errorf("failed to listen: %w", err)
	}

	b.listener = listener
	b.address = listener.Addr().String()
	b.logger.Info("Core started", zap.String("address", b.address))

	b.wg.Add(2)
	go b.acceptLoop()
	go b.cleanupLoop()

	return nil
}

// Stop 停止Core
func (b *Core) Stop() error {
	if !b.started.Load() {
		return nil
	}

	b.cancel()
	if b.listener != nil {
		b.listener.Close()
	}

	b.wg.Wait()
	b.logger.Info("Core stopped")
	return nil
}

// GetAddress 获取监听地址
func (b *Core) GetAddress() string {
	return b.address
}

// acceptLoop 接受连接循环
func (b *Core) acceptLoop() {
	defer b.wg.Done()

	for {
		conn, err := b.listener.Accept()
		if err != nil {
			select {
			case <-b.ctx.Done():
				return
			default:
				b.logger.Warn("Failed to accept connection", zap.Error(err))
				continue
			}
		}

		b.wg.Add(1)
		go b.handleConnection(conn)
	}
}

// handleConnection 处理单个连接
func (b *Core) handleConnection(conn net.Conn) {
	defer b.wg.Done()

	var connTaken bool // 连接是否被取走用于流
	defer func() {
		if !connTaken {
			conn.Close()
		}
	}()

	// 读取CONNECT包
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		b.logger.Warn("Failed to read connect packet", zap.Error(err))
		return
	}

	connectPkt, ok := pkt.(*packet.ConnectPacket)
	if !ok {
		b.logger.Warn("Expected CONNECT packet", zap.String("got", fmt.Sprintf("%T", pkt)))
		return
	}

	// 认证
	if err := b.authHandler(connectPkt.ClientID, connectPkt.AuthMethod, connectPkt.AuthData); err != nil {
		ack := &packet.ConnackPacket{
			ReasonCode: packet.NotAuthorized,
			Message:    err.Error(),
			Properties: make(map[string]string),
		}
		packet.WritePacket(conn, ack)
		return
	}

	// 发送CONNACK
	ack := &packet.ConnackPacket{
		ReasonCode: packet.Success,
		ServerID:   "core-1",
		Properties: make(map[string]string),
	}
	if err := packet.WritePacket(conn, ack); err != nil {
		b.logger.Warn("Failed to send connack", zap.Error(err))
		return
	}

	// 开启 TCP KeepAlive
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		keepAliveConfig := net.KeepAliveConfig{
			Enable:   true,
			Idle:     30 * time.Second,
			Interval: 10 * time.Second,
			Count:    3,
		}
		if err := tcpConn.SetKeepAliveConfig(keepAliveConfig); err != nil {
			b.logger.Warn("Failed to set TCP KeepAlive config", zap.Error(err))
		}
	}

	b.logger.Info("Client connected", zap.String("clientID", connectPkt.ClientID))

	// 读取下一个包 - 可能是STREAM_OPEN（发起方）或进入等待（接收方）
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	pkt, err = packet.ReadPacket(conn)
	conn.SetReadDeadline(time.Time{})
	if err != nil {
		b.logger.Debug("Connection closed", zap.String("clientID", connectPkt.ClientID), zap.Error(err))
		return
	}

	switch p := pkt.(type) {
	case *packet.StreamOpenPacket:
		// 发起方：收到STREAM_OPEN
		// handleStreamOpen 仅在成功建立 stream 后接管连接
		connTaken = b.handleStreamOpen(conn, connectPkt.ClientID, p)
	case *packet.StreamReadyPacket:
		// 接收方：收到STREAM_READY表示准备接收流
		connTaken = b.handleAcceptStream(conn, connectPkt.ClientID)
	case *packet.PingPacket:
		// 接收方可能先发送PING来维持连接
		connTaken = b.handleAcceptStreamWithPing(conn, connectPkt.ClientID)
	default:
		b.logger.Warn("Unexpected packet type", zap.String("clientID", connectPkt.ClientID), zap.String("type", fmt.Sprintf("%T", pkt)))
	}
}

// handleAcceptStreamWithPing 处理带PING的AcceptStream
func (b *Core) handleAcceptStreamWithPing(conn net.Conn, clientID string) bool {
	// 回复PONG
	pong := &packet.PongPacket{}
	packet.WritePacket(conn, pong)

	// 读取真正的等待信号（StreamReadyPacket）
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		b.logger.Warn("Failed to read after ping", zap.String("clientID", clientID), zap.Error(err))
		return false
	}

	if _, ok := pkt.(*packet.StreamReadyPacket); ok {
		return b.handleAcceptStream(conn, clientID)
	}
	return false
}

// handleAcceptStream 处理接收方的等待连接
// 返回true表示连接被取走用于流，调用方不应关闭

func (b *Core) handleAcceptStream(conn net.Conn, clientID string) bool {
	// 创建一个 channel 用于通知连接被取走
	takenCh := make(chan struct{})
	closeCh := make(chan struct{})

	waiting := &WaitingConn{
		conn:      conn,
		clientID:  clientID,
		timestamp: time.Now(),
		takenCh:   takenCh,
		closeCh:   closeCh,
	}

	// 注册等待连接
	b.waitingMu.Lock()
	b.waitingConns[clientID] = append(b.waitingConns[clientID], waiting)
	b.waitingMu.Unlock()

	b.logger.Info("Client waiting for stream", zap.String("clientID", clientID))

	// 等待连接被取走或超时
	select {
	case <-takenCh:
		// 连接已被取走用于流
		b.logger.Debug("Connection taken for stream", zap.String("clientID", clientID))
		return true // 连接已被取走，不要关闭
	case <-time.After(10 * time.Minute):
		// 超时
	case <-b.ctx.Done():
		// core 关闭
	case <-closeCh:
		// 已被 cleanup/过期清理
	}

	// 清理等待连接（只移除当前这一条，避免误删其他等待者）
	b.waitingMu.Lock()
	if list, ok := b.waitingConns[clientID]; ok {
		for i := 0; i < len(list); i++ {
			if list[i] == waiting {
				list = append(list[:i], list[i+1:]...)
				break
			}
		}
		if len(list) == 0 {
			delete(b.waitingConns, clientID)
		} else {
			b.waitingConns[clientID] = list
		}
	}
	b.waitingMu.Unlock()

	b.logger.Info("Client disconnected", zap.String("clientID", clientID))
	return false // 正常断开，需要关闭连接
}

// handleStreamOpen 处理发起方的STREAM_OPEN
// 返回 true 表示已接管 sourceConn 的生命周期（建立 stream 并开始转发）
func (b *Core) handleStreamOpen(sourceConn net.Conn, sourceClientID string, openPkt *packet.StreamOpenPacket) bool {
	streamID := b.streamIDGen.Add(1)
	openPkt.StreamID = streamID
	// 防止伪造：以 CONNECT 的 clientID 为准
	openPkt.SourceClientID = sourceClientID
	targetClientID := openPkt.TargetClientID

	b.logger.Info("Handling STREAM_OPEN", zap.Uint32("streamID", streamID),
		zap.String("source", sourceClientID), zap.String("target", targetClientID), zap.String("topic", openPkt.Topic))

	// 查找目标客户端的等待连接
	b.waitingMu.Lock()
	var waiting *WaitingConn
	if list, ok := b.waitingConns[targetClientID]; ok && len(list) > 0 {
		waiting = list[0]
		list = list[1:]
		if len(list) == 0 {
			delete(b.waitingConns, targetClientID)
		} else {
			b.waitingConns[targetClientID] = list
		}
	}
	b.waitingMu.Unlock()

	if waiting == nil {
		// 目标客户端不在等待
		ack := &packet.StreamAckPacket{
			StreamID:   streamID,
			ReasonCode: packet.ClientNotFound,
			Message:    "target client not waiting",
			Properties: make(map[string]string),
		}
		packet.WritePacket(sourceConn, ack)
		return false
	}

	// 通知等待的 goroutine 连接已被取走
	close(waiting.takenCh)

	targetConn := waiting.conn

	// 转发STREAM_OPEN到目标客户端
	if err := packet.WritePacket(targetConn, openPkt); err != nil {
		b.logger.Warn("Failed to forward stream open", zap.Error(err))
		ack := &packet.StreamAckPacket{
			StreamID:   streamID,
			ReasonCode: packet.ProtocolErr,
			Message:    "failed to forward stream open",
			Properties: make(map[string]string),
		}
		packet.WritePacket(sourceConn, ack)
		targetConn.Close()
		return false
	}

	// 读取目标客户端的ACK
	targetConn.SetReadDeadline(time.Now().Add(30 * time.Second))
	pkt, err := packet.ReadPacket(targetConn)
	targetConn.SetReadDeadline(time.Time{})

	if err != nil {
		b.logger.Warn("Failed to read target ack", zap.Error(err))
		ack := &packet.StreamAckPacket{
			StreamID:   streamID,
			ReasonCode: packet.ProtocolErr,
			Message:    "failed to read target ack",
			Properties: make(map[string]string),
		}
		packet.WritePacket(sourceConn, ack)
		targetConn.Close()
		return false
	}

	ackPkt, ok := pkt.(*packet.StreamAckPacket)
	if !ok {
		b.logger.Warn("Expected STREAM_ACK from target", zap.String("got", fmt.Sprintf("%T", pkt)))
		ack := &packet.StreamAckPacket{
			StreamID:   streamID,
			ReasonCode: packet.ProtocolErr,
			Message:    "invalid response from target",
			Properties: make(map[string]string),
		}
		packet.WritePacket(sourceConn, ack)
		targetConn.Close()
		return false
	}

	// 转发ACK到源客户端
	if err := packet.WritePacket(sourceConn, ackPkt); err != nil {
		b.logger.Warn("Failed to forward ack", zap.Error(err))
		targetConn.Close()
		return false
	}

	// 如果ACK成功，建立流并开始转发
	if ackPkt.ReasonCode == packet.Success {
		session := &StreamSession{
			streamID:       streamID,
			topic:          openPkt.Topic,
			sourceClientID: sourceClientID,
			targetClientID: targetClientID,
			sourceConn:     &ClientConn{conn: sourceConn, clientID: sourceClientID},
			targetConn:     &ClientConn{conn: targetConn, clientID: targetClientID},
		}

		b.streamsMu.Lock()
		b.streams[streamID] = session
		b.streamsMu.Unlock()

		b.logger.Info("Stream established", zap.Uint32("streamID", streamID),
			zap.String("source", sourceClientID), zap.String("target", targetClientID))

		// 开始双向转发
		b.wg.Add(1)
		go b.forwardStream(session)
		return true
	}

	// 目标拒绝：关闭被动方连接，发起方连接由外层 defer 关闭
	targetConn.Close()
	return false
}

// forwardStream 双向转发流数据
func (b *Core) forwardStream(session *StreamSession) {
	defer b.wg.Done()
	defer func() {
		session.closed.Store(true)

		b.streamsMu.Lock()
		delete(b.streams, session.streamID)
		b.streamsMu.Unlock()

		session.sourceConn.conn.Close()
		session.targetConn.conn.Close()

		b.logger.Info("Stream closed", zap.Uint32("streamID", session.streamID),
			zap.Uint64("sent", session.bytesSent.Load()), zap.Uint64("received", session.bytesRecv.Load()))
	}()

	// 使用 done channel 在任一方向结束时关闭连接
	done := make(chan struct{})
	closeOnce := sync.Once{}
	closeConns := func() {
		closeOnce.Do(func() {
			close(done)
			// 关闭连接会让另一个方向的 Read/Write 返回错误
			session.sourceConn.conn.Close()
			session.targetConn.conn.Close()
		})
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// 源 -> 目标
	go func() {
		defer wg.Done()
		defer closeConns()
		b.forwardData(session.sourceConn.conn, session.targetConn.conn, &session.bytesSent)
	}()

	// 目标 -> 源
	go func() {
		defer wg.Done()
		defer closeConns()
		b.forwardData(session.targetConn.conn, session.sourceConn.conn, &session.bytesRecv)
	}()

	wg.Wait()
}

// forwardData 单向数据转发
func (b *Core) forwardData(src, dst net.Conn, counter *atomic.Uint64) {
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if err != nil {
			return
		}
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return
			}
			counter.Add(uint64(n))
		}
	}
}

// cleanupLoop 清理过期的等待连接
func (b *Core) cleanupLoop() {
	defer b.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.cleanup()
		case <-b.ctx.Done():
			return
		}
	}
}

// cleanup 清理过期连接
func (b *Core) cleanup() {
	now := time.Now()
	var expired int

	b.waitingMu.Lock()
	for clientID, list := range b.waitingConns {
		// 原地过滤
		kept := list[:0]
		for _, waiting := range list {
			if now.Sub(waiting.timestamp) > 10*time.Minute {
				expired++
				// 唤醒等待者并关闭连接
				select {
				case <-waiting.closeCh:
					// already closed
				default:
					close(waiting.closeCh)
				}
				waiting.conn.Close()
				continue
			}
			kept = append(kept, waiting)
		}
		if len(kept) == 0 {
			delete(b.waitingConns, clientID)
		} else {
			b.waitingConns[clientID] = kept
		}
	}
	b.waitingMu.Unlock()

	if expired > 0 {
		b.logger.Info("Cleaned up expired waiting connections", zap.Int("count", expired))
	}
}

// ClientConn 客户端连接
type ClientConn struct {
	conn     net.Conn
	clientID string
}

// StreamSession 流会话
type StreamSession struct {
	streamID       uint32
	topic          string
	sourceClientID string
	targetClientID string
	sourceConn     *ClientConn
	targetConn     *ClientConn
	closed         atomic.Bool
	bytesSent      atomic.Uint64
	bytesRecv      atomic.Uint64
}

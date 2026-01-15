package stream

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/snple/beacon/stream/packet"

	"go.uber.org/zap"
)

// Client
type Client struct {
	coreAddr   string
	clientID   string
	authMethod string
	authData   []byte
	logger     *zap.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func cloneBytes(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return make(map[string]string)
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// NewClient 创建 Client。
//
// 必填：
// - coreAddr（WithCoreAddr）
// - clientID（WithClientID）
func NewClient(opts ...ClientOption) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	// 创建默认的 zap logger
	logger, _ := zap.NewDevelopment()
	c := &Client{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	if c.logger == nil {
		logger, _ := zap.NewDevelopment()
		c.logger = logger
	}
	if c.coreAddr == "" {
		cancel()
		return nil, fmt.Errorf("coreAddr is required")
	}
	if c.clientID == "" {
		cancel()
		return nil, fmt.Errorf("clientID is required")
	}

	return c, nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

// connectToCore 连接到core并认证
func (c *Client) connectToCore() (net.Conn, error) {
	// 建立TCP连接
	conn, err := net.DialTimeout("tcp", c.coreAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to core: %w", err)
	}

	// 发送CONNECT
	connectPkt := &packet.ConnectPacket{
		ClientID:   c.clientID,
		AuthMethod: c.authMethod,
		AuthData:   c.authData,
		KeepAlive:  60,
		Properties: make(map[string]string),
	}

	if err := packet.WritePacket(conn, connectPkt); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send connect: %w", err)
	}

	// 读取CONNACK
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read connack: %w", err)
	}

	ackPkt, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("expected CONNACK, got %T", pkt)
	}

	if ackPkt.ReasonCode != packet.Success {
		conn.Close()
		return nil, fmt.Errorf("connection rejected: %s", ackPkt.Message)
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
			c.logger.Warn("Failed to set TCP KeepAlive config", zap.Error(err))
		}
	}

	return conn, nil
}

// OpenStream 发起流连接（主动方）
func (c *Client) OpenStream(targetClientID, topic string, metadata []byte, properties map[string]string) (*Stream, error) {
	// 连接core
	conn, err := c.connectToCore()
	if err != nil {
		return nil, err
	}

	// 发送STREAM_OPEN
	openPkt := &packet.StreamOpenPacket{
		StreamID:       0, // core会分配
		Topic:          topic,
		SourceClientID: c.clientID,
		TargetClientID: targetClientID,
		Metadata:       metadata,
		Properties:     properties,
	}

	if err := packet.WritePacket(conn, openPkt); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send stream open: %w", err)
	}

	// 等待STREAM_ACK
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	conn.SetReadDeadline(time.Time{})

	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read stream ack: %w", err)
	}

	ackPkt, ok := pkt.(*packet.StreamAckPacket)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("expected STREAM_ACK, got %T", pkt)
	}

	if ackPkt.ReasonCode != packet.Success {
		conn.Close()
		return nil, fmt.Errorf("stream rejected: %s", ackPkt.Message)
	}

	// 创建流
	ctx, cancel := context.WithCancel(c.ctx)
	stream := &Stream{
		id:             ackPkt.StreamID,
		topic:          topic,
		metadata:       cloneBytes(metadata),
		properties:     cloneStringMap(properties),
		sourceClientID: c.clientID,
		targetClientID: targetClientID,
		conn:           conn,
		ctx:            ctx,
		cancel:         cancel,
		logger:         c.logger,
	}

	c.logger.Info("Stream opened", zap.Uint32("streamID", stream.id), zap.String("target", targetClientID), zap.String("topic", topic))

	return stream, nil
}

// AcceptStream 等待接收流连接（被动方）
// 返回一个已建立的流，如果客户端关闭则返回 nil
func (c *Client) AcceptStream() (*Stream, error) {
	for {
		select {
		case <-c.ctx.Done():
			return nil, c.ctx.Err()
		default:
		}

		// 连接core
		conn, err := c.connectToCore()
		if err != nil {
			c.logger.Warn("Failed to connect for accept", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		// 发送准备信号（使用 STREAM_READY 表示准备接收流）
		readyPkt := &packet.StreamReadyPacket{
			Properties: make(map[string]string),
		}

		if err := packet.WritePacket(conn, readyPkt); err != nil {
			c.logger.Warn("Failed to send ready signal", zap.Error(err))
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		c.logger.Info("Waiting for stream")

		// 等待STREAM_OPEN
		stream, err := c.waitForStreamOpen(conn)
		if err != nil {
			c.logger.Debug("Wait interrupted", zap.Error(err))
			conn.Close()
			continue
		}

		// 成功接收到流，返回
		return stream, nil
	}
}

// waitForStreamOpen 等待STREAM_OPEN
func (c *Client) waitForStreamOpen(conn net.Conn) (*Stream, error) {
	// 读取STREAM_OPEN（设置较长的超时，因为可能需要等待发起方）
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
		pkt, err := packet.ReadPacket(conn)
		conn.SetReadDeadline(time.Time{})

		if err != nil {
			return nil, err
		}

		switch p := pkt.(type) {
		case *packet.StreamOpenPacket:
			// 收到流请求，处理
			return c.handleStreamOpen(conn, p)

		default:
			c.logger.Warn("Unexpected packet while waiting", zap.String("type", fmt.Sprintf("%T", pkt)))
		}
	}
}

// handleStreamOpen 处理收到的STREAM_OPEN
func (c *Client) handleStreamOpen(conn net.Conn, openPkt *packet.StreamOpenPacket) (*Stream, error) {
	streamID := openPkt.StreamID
	topic := openPkt.Topic

	c.logger.Info("Received stream open", zap.Uint32("streamID", streamID), zap.String("topic", topic), zap.String("from", openPkt.SourceClientID))

	// 创建流
	ctx, cancel := context.WithCancel(c.ctx)
	stream := &Stream{
		id:             streamID,
		topic:          topic,
		metadata:       cloneBytes(openPkt.Metadata),
		properties:     cloneStringMap(openPkt.Properties),
		sourceClientID: openPkt.SourceClientID,
		targetClientID: c.clientID,
		conn:           conn,
		ctx:            ctx,
		cancel:         cancel,
		logger:         c.logger,
	}

	// 发送ACK
	ackPkt := &packet.StreamAckPacket{
		StreamID:   streamID,
		ReasonCode: packet.Success,
		Properties: make(map[string]string),
	}

	if err := packet.WritePacket(conn, ackPkt); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to send ack: %w", err)
	}

	// 返回流供调用方使用
	return stream, nil
}

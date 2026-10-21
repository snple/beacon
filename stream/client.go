package stream

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/snple/beacon/stream/packet"

	"go.uber.org/zap"
)

// Client 是流中继协议的客户端，用于主动 OpenStream 或被动 AcceptStream。
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

// ErrAuthRejected 表示 core 明确拒绝了认证：重试没有意义，应立即返回。
var ErrAuthRejected = errors.New("stream: authentication rejected")

// controlWriteTimeout 限制握手/控制包的写入时间，避免对端不读数据时永久阻塞。
const controlWriteTimeout = 10 * time.Second

func writeControl(conn net.Conn, pkt packet.Packet) error {
	if err := conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout)); err != nil {
		return err
	}
	defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()

	return packet.WritePacket(conn, pkt)
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

	if err := writeControl(conn, connectPkt); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send connect: %w", err)
	}

	// 读取CONNACK
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	_ = conn.SetReadDeadline(time.Time{})

	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to read connack: %w", err)
	}

	ackPkt, ok := pkt.(*packet.ConnackPacket)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("expected CONNACK, got %T", pkt)
	}

	if ackPkt.ReasonCode != packet.Success {
		_ = conn.Close()
		return nil, fmt.Errorf("%w: %s", ErrAuthRejected, ackPkt.Message)
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

	if err := writeControl(conn, openPkt); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to send stream open: %w", err)
	}

	// 等待STREAM_ACK
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	pkt, err := packet.ReadPacket(conn)
	_ = conn.SetReadDeadline(time.Time{})

	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to read stream ack: %w", err)
	}

	ackPkt, ok := pkt.(*packet.StreamAckPacket)
	if !ok {
		_ = conn.Close()
		return nil, fmt.Errorf("expected STREAM_ACK, got %T", pkt)
	}

	if ackPkt.ReasonCode != packet.Success {
		_ = conn.Close()
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
			// 认证被拒绝是确定性的，重试只会无限刷日志。
			if errors.Is(err, ErrAuthRejected) {
				return nil, err
			}
			c.logger.Warn("Failed to connect for accept", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		// 发送准备信号（使用 STREAM_READY 表示准备接收流）
		readyPkt := &packet.StreamReadyPacket{
			Properties: make(map[string]string),
		}

		if err := writeControl(conn, readyPkt); err != nil {
			c.logger.Warn("Failed to send ready signal", zap.Error(err))
			_ = conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		c.logger.Info("Waiting for stream")

		// 等待STREAM_OPEN
		stream, err := c.waitForStreamOpen(conn)
		if err != nil {
			c.logger.Debug("Wait interrupted", zap.Error(err))
			_ = conn.Close()
			// 避免 core 立即断开时形成紧凑重连循环。
			select {
			case <-c.ctx.Done():
				return nil, c.ctx.Err()
			case <-time.After(time.Second):
			}
			continue
		}

		// 成功接收到流，返回
		return stream, nil
	}
}

// waitForStreamOpen 等待STREAM_OPEN
// 通过 goroutine 读取 + select c.ctx.Done()，使 Client.Close() 能中断阻塞读
// （否则关闭后 accept goroutine 最多再阻塞 10 分钟，并反复重建连接）。
func (c *Client) waitForStreamOpen(conn net.Conn) (*Stream, error) {
	type result struct {
		stream *Stream
		err    error
	}
	resultCh := make(chan result, 1)

	go func() {
		// 读取STREAM_OPEN（设置较长的超时，因为可能需要等待发起方）
		for {
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Minute))
			pkt, err := packet.ReadPacket(conn)
			_ = conn.SetReadDeadline(time.Time{})

			if err != nil {
				resultCh <- result{nil, err}
				return
			}

			switch p := pkt.(type) {
			case *packet.StreamOpenPacket:
				// 收到流请求，处理
				stream, err := c.handleStreamOpen(conn, p)
				resultCh <- result{stream, err}
				return

			default:
				c.logger.Warn("Unexpected packet while waiting", zap.String("type", fmt.Sprintf("%T", pkt)))
			}
		}
	}()

	select {
	case res := <-resultCh:
		return res.stream, res.err
	case <-c.ctx.Done():
		// 客户端已关闭：关闭连接以中断读取 goroutine，避免其长时间阻塞
		_ = conn.Close()
		return nil, c.ctx.Err()
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

	if err := writeControl(conn, ackPkt); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to send ack: %w", err)
	}

	// 返回流供调用方使用
	return stream, nil
}

package stream

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/stream/packet"

	"go.uber.org/zap"
)

// Stream 表示一个已建立的流连接
type Stream struct {
	id             uint32
	topic          string
	metadata       []byte
	properties     map[string]string
	sourceClientID string
	targetClientID string
	conn           net.Conn
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *zap.Logger

	// 读缓冲：存储部分读取的数据包
	readBuf    []byte
	readOffset int

	// 写序列号
	writeSeq atomic.Uint32
}

// ID 返回流ID
func (s *Stream) ID() uint32 {
	return s.id
}

// Topic 返回流主题
func (s *Stream) Topic() string {
	return s.topic
}

// Metadata 返回对端在 STREAM_OPEN 中携带的元数据（主动方为自己传入的 metadata）
func (s *Stream) Metadata() []byte {
	if len(s.metadata) == 0 {
		return nil
	}
	out := make([]byte, len(s.metadata))
	copy(out, s.metadata)
	return out
}

// Properties 返回对端在 STREAM_OPEN 中携带的属性（主动方为自己传入的 properties）
func (s *Stream) Properties() map[string]string {
	if len(s.properties) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(s.properties))
	for k, v := range s.properties {
		out[k] = v
	}
	return out
}

// SourceClientID 返回源客户端ID
func (s *Stream) SourceClientID() string {
	return s.sourceClientID
}

// TargetClientID 返回目标客户端ID
func (s *Stream) TargetClientID() string {
	return s.targetClientID
}

// WritePacket 发送一个数据包
func (s *Stream) WritePacket(pkt *packet.StreamDataPacket) error {
	pkt.StreamID = s.id
	pkt.SequenceID = s.writeSeq.Add(1)
	return packet.WritePacket(s.conn, pkt)
}

// ReadPacket 接收一个数据包
// 返回数据包，如果连接关闭返回 io.EOF
func (s *Stream) ReadPacket() (*packet.StreamDataPacket, error) {
	pkt, err := packet.ReadPacket(s.conn)
	if err != nil {
		return nil, err
	}

	dataPkt, ok := pkt.(*packet.StreamDataPacket)
	if !ok {
		return nil, fmt.Errorf("unexpected packet type: %T", pkt)
	}

	return dataPkt, nil
}

// Write 实现 io.Writer 接口，将数据作为一个数据包发送
// 注意：每次 Write 调用都会发送一个完整的数据包，保持消息边界
func (s *Stream) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	pkt := &packet.StreamDataPacket{
		StreamID:   s.id,
		Flags:      0,
		Data:       p,
		Properties: make(map[string]string),
	}

	if err := s.WritePacket(pkt); err != nil {
		return 0, err
	}

	return len(p), nil
}

// Read 实现 io.Reader 接口，从数据包中读取数据
// 注意：如果 p 太小无法装下整个数据包，会分多次读取
func (s *Stream) Read(p []byte) (int, error) {
	// 如果缓冲区为空，读取下一个数据包
	if len(s.readBuf) == 0 || s.readOffset >= len(s.readBuf) {
		pkt, err := s.ReadPacket()
		if err != nil {
			return 0, err
		}

		s.readBuf = pkt.Data
		s.readOffset = 0
	}

	// 从缓冲区复制数据到 p
	n := copy(p, s.readBuf[s.readOffset:])
	s.readOffset += n

	// 如果已经读完当前包，清空缓冲区
	if s.readOffset >= len(s.readBuf) {
		s.readBuf = nil
		s.readOffset = 0
	}

	return n, nil
}

// Close 关闭流
func (s *Stream) Close() error {
	s.cancel()
	return s.conn.Close()
}

// LocalAddr 返回本地网络地址
func (s *Stream) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

// RemoteAddr 返回远程网络地址
func (s *Stream) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

// SetDeadline 设置读写超时
func (s *Stream) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

// SetReadDeadline 设置读超时
func (s *Stream) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}

// SetWriteDeadline 设置写超时
func (s *Stream) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

// Context 返回流的context
func (s *Stream) Context() context.Context {
	return s.ctx
}

// Conn 返回底层连接（用于特殊情况）
func (s *Stream) Conn() net.Conn {
	return s.conn
}

var _ net.Conn = (*Stream)(nil)

package client

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// conn 表示单次网络连接。
type conn struct {
	client *Client
	nc     net.Conn

	writeMu      sync.Mutex
	writeTimeout time.Duration
	// readMaxPacketSize 限制本端可接收的单包大小；writeMaxPacketSize 限制发送。
	// 两者相互独立：接收上限小并不意味只能发送小包。
	readMaxPacketSize  uint32
	writeMaxPacketSize uint32
	// readTimeout 是应用层存活窗口：超过该时长没有收到任何包就认为连接
	// 已死（半开连接、服务端挂死）。0 表示不检测。
	readTimeout time.Duration

	pendingMu sync.Mutex
	pending   map[uint32]chan packet.Packet

	closed    atomic.Bool
	closeOnce sync.Once
	done      chan struct{}

	errMu sync.Mutex
	err   error

	connack *packet.ConnackPacket
}

func newConn(c *Client, nc net.Conn) *conn {
	writeMax := c.opts.MaxSendPacketSize
	if writeMax == 0 {
		// CONNACK 之前的发送上限未知；按协议硬上限编码 CONNECT，
		// 由服务端按它自己的 MaxPacketSize 决定是否接受。
		writeMax = packet.MaxPacketSize
	}
	return &conn{
		client:             c,
		nc:                 nc,
		writeTimeout:       c.opts.WriteTimeout,
		readMaxPacketSize:  c.opts.MaxRecvPacketSize,
		writeMaxPacketSize: writeMax,
		pending:            make(map[uint32]chan packet.Packet),
		done:               make(chan struct{}),
	}
}

func (cn *conn) register(id uint32, ch chan packet.Packet) {
	cn.pendingMu.Lock()
	cn.pending[id] = ch
	cn.pendingMu.Unlock()
}

func (cn *conn) unregister(id uint32) {
	cn.pendingMu.Lock()
	delete(cn.pending, id)
	cn.pendingMu.Unlock()
}

func (cn *conn) write(pkt packet.Packet) error {
	cn.writeMu.Lock()
	defer cn.writeMu.Unlock()

	if cn.writeTimeout > 0 {
		_ = cn.nc.SetWriteDeadline(time.Now().Add(cn.writeTimeout))
	}
	if _, err := packet.WritePacketWithSize(cn.nc, pkt, cn.writeMaxPacketSize); err != nil {
		return err
	}
	if cn.writeTimeout > 0 {
		_ = cn.nc.SetWriteDeadline(time.Time{})
	}
	return nil
}

// Err 返回连接结束原因。
func (cn *conn) Err() error {
	cn.errMu.Lock()
	defer cn.errMu.Unlock()
	return cn.err
}

func (cn *conn) setErr(err error) {
	cn.errMu.Lock()
	if cn.err == nil {
		cn.err = err
	}
	cn.errMu.Unlock()
}

// Done 在连接关闭时关闭。
func (cn *conn) Done() <-chan struct{} { return cn.done }

func (cn *conn) close(err error) {
	cn.closeOnce.Do(func() {
		cn.closed.Store(true)
		cn.setErr(err)
		if cn.nc != nil {
			_ = cn.nc.Close()
		}
		close(cn.done)

		// 唤醒全部等待者
		cn.pendingMu.Lock()
		for id, ch := range cn.pending {
			close(ch)
			delete(cn.pending, id)
		}
		cn.pendingMu.Unlock()
	})
}

// readLoop 读取响应并分发到等待者。
func (cn *conn) readLoop() {
	var err error
	for {
		if cn.closed.Load() {
			return
		}

		if cn.readTimeout > 0 {
			_ = cn.nc.SetReadDeadline(time.Now().Add(cn.readTimeout))
		}

		pkt, _, rerr := packet.ReadPacketWithSize(cn.nc, cn.readMaxPacketSize)
		if rerr != nil {
			var ne net.Error
			if errors.As(rerr, &ne) && ne.Timeout() {
				err = NewConnectionError("read", ErrKeepAliveTimeout)
			} else if errors.Is(rerr, io.EOF) {
				err = ErrNotConnected
			} else {
				err = NewConnectionError("read", rerr)
			}
			break
		}

		switch p := pkt.(type) {
		case *packet.PongPacket:
			// 心跳响应，无需处理
		case *packet.DisconnectPacket:
			err = reasonToError(p.ReasonCode)
			if err == nil {
				err = ErrNotConnected
			}
			cn.client.logger.Debug("server disconnected",
				zap.String("reason", p.ReasonCode.String()))
			cn.close(err)
			return
		default:
			id := requestIDOf(pkt)
			if id == 0 {
				continue
			}
			cn.pendingMu.Lock()
			ch, ok := cn.pending[id]
			delete(cn.pending, id)
			cn.pendingMu.Unlock()
			if ok {
				ch <- pkt
			}
		}
	}

	cn.close(err)
}

// requestIDOf 返回响应包关联的请求 ID。
func requestIDOf(pkt packet.Packet) uint32 {
	switch p := pkt.(type) {
	case *packet.PushOkPacket:
		return p.RequestID
	case *packet.PullOkPacket:
		return p.RequestID
	case *packet.CommitOkPacket:
		return p.RequestID
	case *packet.DeclareRespPacket:
		return p.RequestID
	case *packet.ConnackPacket:
		return 0
	default:
		return 0
	}
}

// pingLoop 周期性发送 PING，维持服务端读超时。
func (cn *conn) pingLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var seq uint32
	for {
		select {
		case <-cn.done:
			return
		case <-ticker.C:
			seq++
			if err := cn.write(packet.NewPingPacket(seq, uint64(time.Now().UnixNano()))); err != nil {
				cn.close(NewConnectionError("ping", err))
				return
			}
		}
	}
}

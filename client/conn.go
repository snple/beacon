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

	writeMu       sync.Mutex
	writeTimeout  time.Duration
	maxPacketSize uint32

	pendingMu sync.Mutex
	pending   map[uint64]chan packet.Packet

	closed    atomic.Bool
	closeOnce sync.Once
	done      chan struct{}

	errMu sync.Mutex
	err   error

	connack *packet.ConnackPacket
}

func newConn(c *Client, nc net.Conn) *conn {
	return &conn{
		client:        c,
		nc:            nc,
		writeTimeout:  c.opts.WriteTimeout,
		maxPacketSize: c.opts.MaxPacketSize,
		pending:       make(map[uint64]chan packet.Packet),
		done:          make(chan struct{}),
	}
}

func (cn *conn) register(id uint64, ch chan packet.Packet) {
	cn.pendingMu.Lock()
	cn.pending[id] = ch
	cn.pendingMu.Unlock()
}

func (cn *conn) unregister(id uint64) {
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
	if _, err := packet.WritePacketWithSize(cn.nc, pkt, cn.maxPacketSize); err != nil {
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

		pkt, _, rerr := packet.ReadPacketWithSize(cn.nc, cn.maxPacketSize)
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
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
func requestIDOf(pkt packet.Packet) uint64 {
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

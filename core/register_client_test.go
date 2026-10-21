package core

import (
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/snple/beacon/packet"
)

// recordingConn 记录写入次数的 net.Conn，用于断言持锁路径不写网络。
type recordingConn struct {
	mu     sync.Mutex
	writes int
	closed bool
}

func (c *recordingConn) Read([]byte) (int, error) { return 0, io.EOF }

func (c *recordingConn) Write(b []byte) (int, error) {
	c.mu.Lock()
	c.writes++
	c.mu.Unlock()
	return len(b), nil
}

func (c *recordingConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *recordingConn) LocalAddr() net.Addr              { return testAddr("local") }
func (c *recordingConn) RemoteAddr() net.Addr             { return testAddr("remote") }
func (c *recordingConn) SetDeadline(time.Time) error      { return nil }
func (c *recordingConn) SetReadDeadline(time.Time) error  { return nil }
func (c *recordingConn) SetWriteDeadline(time.Time) error { return nil }
func (c *recordingConn) writeCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.writes
}
func (c *recordingConn) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// TestRegisterClient_DoesNotWriteOldConn 验证会话接管时 registerClient
// 不在持有 clientsMu 的情况下写旧连接（旧连接只做 detach）。
func TestRegisterClient_DoesNotWriteOldConn(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	oldConn := &recordingConn{}
	connectOld := packet.NewConnectPacket()
	connectOld.ClientID = "takeover"
	connectOld.SessionTimeout = 60
	oldClient := newClient(oldConn, connectOld, c)

	c.clientsMu.Lock()
	c.clients["takeover"] = oldClient
	c.clientsMu.Unlock()

	newConn, peer := net.Pipe()
	defer peer.Close()
	defer newConn.Close()

	connectNew := packet.NewConnectPacket()
	connectNew.ClientID = "takeover"
	connectNew.SessionTimeout = 60

	client, sessionPresent, _, oldConns := c.registerClient(newConn, connectNew)

	if client == nil {
		t.Fatal("registerClient returned nil client")
	}
	if !sessionPresent {
		t.Fatal("expected session to be resumed")
	}
	if got := oldConn.writeCount(); got != 0 {
		t.Fatalf("registerClient wrote %d times to the old conn while holding clientsMu", got)
	}
	// 新连接已替换 client.conn，oldConns 中应恰好是旧连接。
	if len(oldConns) != 1 || oldConns[0].conn != oldConn {
		t.Fatalf("expected old conn to be detached, got %d conns", len(oldConns))
	}

	// 模拟 handleConn 在锁外关闭旧连接。
	for _, oc := range oldConns {
		oc.closeLocal(packet.ReasonSessionTakenOver)
	}
	if !oldConn.isClosed() {
		t.Fatal("old conn should be closed outside the lock")
	}
	if oldConn.writeCount() != 0 {
		t.Fatal("closeLocal must not write DISCONNECT")
	}
}

// TestRegisterClient_RejectedAfterStop 验证停机后到达的注册会被拒绝，
// 避免连接在 Stop 的客户端快照之后注册而拖住停机。
func TestRegisterClient_RejectedAfterStop(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	conn, peer := net.Pipe()
	defer peer.Close()
	defer conn.Close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "late-registration"

	client, sessionPresent, _, oldConns := c.registerClient(conn, connect)
	if client != nil || sessionPresent || len(oldConns) != 0 {
		t.Fatalf("expected registration to be rejected, got client=%v present=%v oldConns=%d",
			client, sessionPresent, len(oldConns))
	}
}

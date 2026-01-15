package stream

import (
	"io"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestStreamAsIOReadWriter 演示 Stream 实现了 io.ReadWriter 接口
// 可以像普通的 io 流一样使用 Read/Write
func TestStreamAsIOReadWriter(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	core, err := NewCore(
		WithAddress("127.0.0.1:0"),
		WithCoreLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	if err := core.Start(); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}
	defer core.Stop()

	clientA, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientA"),
		WithAuth("", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	clientB, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientB"),
		WithAuth("", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientB: %v", err)
	}
	defer clientB.Close()

	// ClientB 等待并使用 io.Copy 转发
	go func() {
		stream, err := clientB.AcceptStream()
		if err != nil {
			return
		}
		defer stream.Close()

		// 使用 io.Copy 实现 echo，因为 Stream 实现了 io.ReadWriter
		// 注意：这里会一直复制直到读取到 EOF
		buf := make([]byte, 1024)
		for {
			n, err := stream.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return
			}
			if n > 0 {
				stream.Write(buf[:n])
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// ClientA 使用 Write 发送数据
	stream, err := clientA.OpenStream("clientB", "io-test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// 使用 Write 方法发送数据（会自动包装成数据包）
	testMsg := []byte("Hello via io.Writer!")
	n, err := stream.Write(testMsg)
	if err != nil {
		t.Fatalf("Failed to write: %v", err)
	}
	if n != len(testMsg) {
		t.Fatalf("Write returned %d, expected %d", n, len(testMsg))
	}

	// 使用 Read 方法读取响应（会自动从数据包中提取）
	buf := make([]byte, 1024)
	n, err = stream.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read: %v", err)
	}

	received := string(buf[:n])
	if received != string(testMsg) {
		t.Errorf("Echo failed: got %q, want %q", received, string(testMsg))
	}

	t.Logf("Successfully echoed via io.ReadWriter: %s", received)
}

// TestStreamAsNetConn 演示 Stream 实现了 net.Conn 接口
func TestStreamAsNetConn(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	core, err := NewCore(
		WithAddress("127.0.0.1:0"),
		WithCoreLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create core: %v", err)
	}
	if err := core.Start(); err != nil {
		t.Fatalf("Failed to start core: %v", err)
	}
	defer core.Stop()

	clientA, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientA"),
		WithAuth("", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	clientB, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientB"),
		WithAuth("", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientB: %v", err)
	}
	defer clientB.Close()

	go func() {
		stream, _ := clientB.AcceptStream()
		if stream != nil {
			defer stream.Close()
			buf := make([]byte, 1024)
			n, _ := stream.Read(buf)
			stream.Write(buf[:n])
		}
	}()

	time.Sleep(500 * time.Millisecond)

	stream, err := clientA.OpenStream("clientB", "conn-test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// 验证 Stream 实现了 net.Conn 接口
	var conn io.ReadWriteCloser = stream
	_ = conn

	// 可以使用 SetDeadline 等方法
	deadline := time.Now().Add(5 * time.Second)
	if err := stream.SetDeadline(deadline); err != nil {
		t.Fatalf("Failed to set deadline: %v", err)
	}

	// 正常使用
	stream.Write([]byte("test"))
	buf := make([]byte, 1024)
	n, _ := stream.Read(buf)

	t.Logf("Received via net.Conn interface: %s", string(buf[:n]))
}

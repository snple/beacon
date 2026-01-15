package stream

import (
	"io"
	"sync"
	"testing"
	"time"

	"github.com/snple/beacon/stream/packet"

	"go.uber.org/zap"
)

func TestBasicStreamFlow(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// 启动core
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

	t.Logf("Core started at %s", core.GetAddress())

	// 创建客户端
	clientA, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientA"),
		WithAuth("tokenA", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	clientB, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientB"),
		WithAuth("tokenB", nil),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientB: %v", err)
	}
	defer clientB.Close()

	// ClientB等待接收流
	var wg sync.WaitGroup
	receivedData := make([]byte, 0)
	var receiveMu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()

		stream, err := clientB.AcceptStream()
		if err != nil {
			t.Logf("AcceptStream error: %v", err)
			return
		}
		defer stream.Close()

		t.Logf("ClientB received stream: %d from %s", stream.ID(), stream.SourceClientID())

		// 读取数据包
		for {
			pkt, err := stream.ReadPacket()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Logf("ReadPacket error: %v", err)
				return
			}

			receiveMu.Lock()
			receivedData = append(receivedData, pkt.Data...)
			receiveMu.Unlock()
		}
	}()

	// 等待clientB准备好
	time.Sleep(500 * time.Millisecond)

	// ClientA发起流
	stream, err := clientA.OpenStream("clientB", "test-topic", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	t.Logf("ClientA opened stream: %d", stream.ID())

	// 发送数据包
	testData := []byte("Hello from ClientA!")
	pkt := &packet.StreamDataPacket{
		Data:       testData,
		Properties: make(map[string]string),
	}
	if err := stream.WritePacket(pkt); err != nil {
		t.Fatalf("Failed to write packet: %v", err)
	}

	// 关闭写端
	stream.Close()

	// 等待接收完成
	time.Sleep(500 * time.Millisecond)

	// 验证
	receiveMu.Lock()
	received := string(receivedData)
	receiveMu.Unlock()

	if received != string(testData) {
		t.Errorf("Data mismatch: got %q, want %q", received, string(testData))
	}

	t.Log("Test passed!")
}

func TestBidirectionalStream(t *testing.T) {
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
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	clientB, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientB"),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientB: %v", err)
	}
	defer clientB.Close()

	// ClientB等待并echo
	go func() {
		for {
			stream, err := clientB.AcceptStream()
			if err != nil {
				return
			}

			t.Logf("ClientB: Echo stream %d", stream.ID())

			// Echo 数据包
			for {
				pkt, err := stream.ReadPacket()
				if err != nil {
					stream.Close()
					break
				}

				if err := stream.WritePacket(pkt); err != nil {
					stream.Close()
					break
				}
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// ClientA发起并测试echo
	stream, err := clientA.OpenStream("clientB", "echo", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	// 发送数据包
	testMsg := "Echo test message"
	pkt := &packet.StreamDataPacket{
		Data:       []byte(testMsg),
		Properties: make(map[string]string),
	}
	if err := stream.WritePacket(pkt); err != nil {
		t.Fatalf("Failed to write packet: %v", err)
	}

	// 读取echo数据包
	respPkt, err := stream.ReadPacket()
	if err != nil {
		t.Fatalf("Failed to read packet: %v", err)
	}

	received := string(respPkt.Data)
	if received != testMsg {
		t.Errorf("Echo failed: got %q, want %q", received, testMsg)
	}

	t.Log("Bidirectional test passed!")
}

func TestStreamRejection(t *testing.T) {
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
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	// 不启动clientB

	// ClientA尝试连接不存在的clientB
	_, err = clientA.OpenStream("clientB", "test", nil, nil)
	if err == nil {
		t.Fatal("Expected error when target not available")
	}

	t.Logf("Got expected error: %v", err)
}

func TestMultipleStreams(t *testing.T) {
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
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientA: %v", err)
	}
	defer clientA.Close()

	clientB, err := NewClient(
		WithCoreAddr(core.GetAddress()),
		WithClientID("clientB"),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create clientB: %v", err)
	}
	defer clientB.Close()

	// ClientB启动多个accept goroutines
	for i := 0; i < 3; i++ {
		go func(id int) {
			for {
				stream, err := clientB.AcceptStream()
				if err != nil {
					return
				}

				t.Logf("ClientB handler %d: stream %d", id, stream.ID())

				pkt, _ := stream.ReadPacket()
				stream.WritePacket(pkt)
				stream.Close()
			}
		}(i)
	}

	time.Sleep(1 * time.Second)

	// ClientA创建多个流
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			stream, err := clientA.OpenStream("clientB", "multi", nil, nil)
			if err != nil {
				t.Logf("Stream %d failed: %v", id, err)
				return
			}
			defer stream.Close()

			msg := []byte("Message " + string(rune('0'+id)))
			pkt := &packet.StreamDataPacket{
				Data:       msg,
				Properties: make(map[string]string),
			}
			stream.WritePacket(pkt)

			respPkt, _ := stream.ReadPacket()
			t.Logf("Stream %d received: %s", id, string(respPkt.Data))
		}(i)
	}

	wg.Wait()
	t.Log("Multiple streams test passed!")
}

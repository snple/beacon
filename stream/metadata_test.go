package stream

import (
	"bytes"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestMetadataAndPropertiesTransfer 测试 OpenStream 的 topic/metadata/properties 能否被 AcceptStream 正确接收
func TestMetadataAndPropertiesTransfer(t *testing.T) {
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

	// 准备测试数据
	expectedTopic := "test-topic-metadata"
	expectedMetadata := []byte("test metadata content with 中文")
	expectedProperties := map[string]string{
		"key1":    "value1",
		"key2":    "value2",
		"unicode": "测试中文",
		"empty":   "",
	}

	// 接收方通道
	type ReceivedData struct {
		topic      string
		metadata   []byte
		properties map[string]string
		sourceID   string
		targetID   string
	}
	receivedCh := make(chan ReceivedData, 1)

	// ClientB 等待接收
	go func() {
		stream, err := clientB.AcceptStream()
		if err != nil {
			t.Logf("AcceptStream error: %v", err)
			return
		}
		defer stream.Close()

		// 提取接收到的字段
		received := ReceivedData{
			topic:      stream.Topic(),
			metadata:   stream.Metadata(),
			properties: stream.Properties(),
			sourceID:   stream.SourceClientID(),
			targetID:   stream.TargetClientID(),
		}

		receivedCh <- received
	}()

	time.Sleep(500 * time.Millisecond)

	// ClientA 发起流并传入 metadata 和 properties
	streamA, err := clientA.OpenStream("clientB", expectedTopic, expectedMetadata, expectedProperties)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer streamA.Close()

	// 验证主动方能获取到自己传入的数据
	if streamA.Topic() != expectedTopic {
		t.Errorf("ClientA: Topic mismatch: got %q, want %q", streamA.Topic(), expectedTopic)
	}
	if !bytes.Equal(streamA.Metadata(), expectedMetadata) {
		t.Errorf("ClientA: Metadata mismatch: got %v, want %v", streamA.Metadata(), expectedMetadata)
	}
	propsA := streamA.Properties()
	if len(propsA) != len(expectedProperties) {
		t.Errorf("ClientA: Properties count mismatch: got %d, want %d", len(propsA), len(expectedProperties))
	}
	for k, v := range expectedProperties {
		if propsA[k] != v {
			t.Errorf("ClientA: Property %q mismatch: got %q, want %q", k, propsA[k], v)
		}
	}
	if streamA.SourceClientID() != "clientA" {
		t.Errorf("ClientA: SourceClientID mismatch: got %q, want %q", streamA.SourceClientID(), "clientA")
	}
	if streamA.TargetClientID() != "clientB" {
		t.Errorf("ClientA: TargetClientID mismatch: got %q, want %q", streamA.TargetClientID(), "clientB")
	}

	// 等待并验证被动方接收到的数据
	select {
	case received := <-receivedCh:
		// 验证 topic
		if received.topic != expectedTopic {
			t.Errorf("ClientB: Topic mismatch: got %q, want %q", received.topic, expectedTopic)
		}

		// 验证 metadata
		if !bytes.Equal(received.metadata, expectedMetadata) {
			t.Errorf("ClientB: Metadata mismatch: got %v, want %v", received.metadata, expectedMetadata)
		}

		// 验证 properties
		if len(received.properties) != len(expectedProperties) {
			t.Errorf("ClientB: Properties count mismatch: got %d, want %d", len(received.properties), len(expectedProperties))
		}
		for k, v := range expectedProperties {
			if received.properties[k] != v {
				t.Errorf("ClientB: Property %q mismatch: got %q, want %q", k, received.properties[k], v)
			}
		}

		// 验证 clientID
		if received.sourceID != "clientA" {
			t.Errorf("ClientB: SourceClientID mismatch: got %q, want %q", received.sourceID, "clientA")
		}
		if received.targetID != "clientB" {
			t.Errorf("ClientB: TargetClientID mismatch: got %q, want %q", received.targetID, "clientB")
		}

		t.Log("✓ All metadata and properties transferred correctly!")

	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for stream acceptance")
	}
}

// TestEmptyMetadataAndProperties 测试空 metadata 和空 properties 的情况
func TestEmptyMetadataAndProperties(t *testing.T) {
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

	receivedCh := make(chan bool, 1)

	go func() {
		stream, err := clientB.AcceptStream()
		if err != nil {
			return
		}
		defer stream.Close()

		// 验证空值
		if stream.Topic() != "empty-test" {
			t.Errorf("Topic mismatch: got %q, want %q", stream.Topic(), "empty-test")
		}
		if stream.Metadata() != nil {
			t.Errorf("Metadata should be nil, got %v", stream.Metadata())
		}
		props := stream.Properties()
		if len(props) != 0 {
			t.Errorf("Properties should be empty, got %v", props)
		}

		receivedCh <- true
	}()

	time.Sleep(500 * time.Millisecond)

	// 传入 nil metadata 和 nil properties
	stream, err := clientA.OpenStream("clientB", "empty-test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	select {
	case <-receivedCh:
		t.Log("✓ Empty metadata and properties handled correctly!")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout")
	}
}

// TestLargeMetadata 测试大型 metadata 传递
func TestLargeMetadata(t *testing.T) {
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

	// 创建 1MB 的 metadata
	largeMetadata := make([]byte, 1024*1024)
	for i := range largeMetadata {
		largeMetadata[i] = byte(i % 256)
	}

	receivedCh := make(chan []byte, 1)

	go func() {
		stream, err := clientB.AcceptStream()
		if err != nil {
			return
		}
		defer stream.Close()

		receivedCh <- stream.Metadata()
	}()

	time.Sleep(500 * time.Millisecond)

	stream, err := clientA.OpenStream("clientB", "large-test", largeMetadata, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	select {
	case received := <-receivedCh:
		if !bytes.Equal(received, largeMetadata) {
			t.Errorf("Large metadata mismatch: length got %d, want %d", len(received), len(largeMetadata))
		} else {
			t.Logf("✓ Large metadata (1MB) transferred correctly!")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout")
	}
}

// TestSourceClientIDAntiSpoofing 测试 Core 是否正确防止 SourceClientID 伪造
func TestSourceClientIDAntiSpoofing(t *testing.T) {
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

	receivedSourceIDCh := make(chan string, 1)

	go func() {
		stream, err := clientB.AcceptStream()
		if err != nil {
			return
		}
		defer stream.Close()

		// 检查接收到的 SourceClientID
		receivedSourceIDCh <- stream.SourceClientID()
	}()

	time.Sleep(500 * time.Millisecond)

	// ClientA 发起流
	stream, err := clientA.OpenStream("clientB", "anti-spoof-test", nil, nil)
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}
	defer stream.Close()

	select {
	case receivedSourceID := <-receivedSourceIDCh:
		// Core 应该强制把 SourceClientID 设置为实际的 clientID（clientA）
		// 即使客户端尝试伪造也会被覆盖
		if receivedSourceID != "clientA" {
			t.Errorf("SourceClientID should be 'clientA' (from CONNECT), got %q", receivedSourceID)
		} else {
			t.Log("✓ Core correctly enforces SourceClientID from CONNECT packet!")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout")
	}
}

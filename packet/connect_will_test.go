package packet

import (
	"bytes"
	"testing"
)

// TestWillMessageEncoding 测试遗嘱消息编码的正确性
func TestWillMessageEncoding(t *testing.T) {
	// 创建带遗嘱的 CONNECT 包
	pkt := NewConnectPacket()
	pkt.ClientID = "test-will"
	pkt.Will = true
	pkt.WillPacket = NewPublishPacket("will/topic", []byte("will payload"))
	pkt.WillPacket.QoS = QoS1
	pkt.WillPacket.Retain = true

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	data := buf.Bytes()
	t.Logf("编码后的数据长度: %d", len(data))
	t.Logf("编码后的数据 (hex): %x", data)

	// 解码
	reader := bytes.NewReader(data)
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(reader); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(reader, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证遗嘱消息
	if decoded.WillPacket == nil {
		t.Fatal("WillPacket 为 nil")
	}

	if decoded.WillPacket.QoS != pkt.WillPacket.QoS {
		t.Errorf("QoS 不匹配: 期望 %v, 得到 %v", pkt.WillPacket.QoS, decoded.WillPacket.QoS)
	}

	if decoded.WillPacket.Retain != pkt.WillPacket.Retain {
		t.Errorf("Retain 不匹配: 期望 %v, 得到 %v", pkt.WillPacket.Retain, decoded.WillPacket.Retain)
	}

	if decoded.WillPacket.Topic != pkt.WillPacket.Topic {
		t.Errorf("Topic 不匹配: 期望 %s, 得到 %s", pkt.WillPacket.Topic, decoded.WillPacket.Topic)
	}

	if !bytes.Equal(decoded.WillPacket.Payload, pkt.WillPacket.Payload) {
		t.Errorf("Payload 不匹配: 期望 %v, 得到 %v", pkt.WillPacket.Payload, decoded.WillPacket.Payload)
	}

	t.Logf("✓ 遗嘱消息编码/解码正确")
}

// TestWillMessageWithDifferentQoS 测试不同 QoS 级别的遗嘱消息
func TestWillMessageWithDifferentQoS(t *testing.T) {
	testCases := []struct {
		name   string
		qos    QoS
		retain bool
	}{
		{"QoS0-NoRetain", QoS0, false},
		{"QoS0-Retain", QoS0, true},
		{"QoS1-NoRetain", QoS1, false},
		{"QoS1-Retain", QoS1, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pkt := NewConnectPacket()
			pkt.ClientID = "test-qos"
			pkt.Will = true
			pkt.WillPacket = NewPublishPacket("test/topic", []byte("test payload"))
			pkt.WillPacket.QoS = tc.qos
			pkt.WillPacket.Retain = tc.retain

			// 编码
			var buf bytes.Buffer
			if err := pkt.Encode(&buf); err != nil {
				t.Fatalf("编码失败: %v", err)
			}

			// 解码
			reader := bytes.NewReader(buf.Bytes())
			decoded := &ConnectPacket{}
			header := FixedHeader{}
			if err := header.Decode(reader); err != nil {
				t.Fatalf("解码固定头部失败: %v", err)
			}

			if err := decoded.Decode(reader, header); err != nil {
				t.Fatalf("解码失败: %v", err)
			}

			// 验证
			if decoded.WillPacket.QoS != tc.qos {
				t.Errorf("QoS 不匹配: 期望 %v, 得到 %v", tc.qos, decoded.WillPacket.QoS)
			}
			if decoded.WillPacket.Retain != tc.retain {
				t.Errorf("Retain 不匹配: 期望 %v, 得到 %v", tc.retain, decoded.WillPacket.Retain)
			}
		})
	}
}

// TestWillMessageWithEmptyPayload 测试空载荷的遗嘱消息
func TestWillMessageWithEmptyPayload(t *testing.T) {
	pkt := NewConnectPacket()
	pkt.ClientID = "test-empty-payload"
	pkt.Will = true
	pkt.WillPacket = NewPublishPacket("test/topic", []byte{})
	pkt.WillPacket.QoS = QoS0

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	reader := bytes.NewReader(buf.Bytes())
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(reader); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(reader, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if decoded.WillPacket == nil {
		t.Fatal("WillPacket 为 nil")
	}
	if len(decoded.WillPacket.Payload) != 0 {
		t.Errorf("空载荷应该保持为空，但得到: %v", decoded.WillPacket.Payload)
	}
}

// TestWillMessageWithLargePayload 测试大载荷的遗嘱消息
func TestWillMessageWithLargePayload(t *testing.T) {
	// 创建一个较大的载荷 (1KB)
	largePayload := make([]byte, 1024)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}

	pkt := NewConnectPacket()
	pkt.ClientID = "test-large-payload"
	pkt.Will = true
	pkt.WillPacket = NewPublishPacket("test/topic", largePayload)
	pkt.WillPacket.QoS = QoS1

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	reader := bytes.NewReader(buf.Bytes())
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(reader); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(reader, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if !bytes.Equal(decoded.WillPacket.Payload, largePayload) {
		t.Errorf("大载荷不匹配，期望长度 %d, 得到长度 %d",
			len(largePayload), len(decoded.WillPacket.Payload))
	}
}

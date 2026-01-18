package packet

import (
	"bytes"
	"testing"
)

func TestAuthPacket(t *testing.T) {
	// 创建 AUTH 包
	auth := NewAuthPacket(ReasonContinueAuth)
	auth.Properties.AuthMethod = "oauth2"
	auth.Properties.AuthData = []byte("test-auth-data")
	auth.Properties.UserProperties = map[string]string{
		"client_type": "mobile",
		"version":     "1.0",
	}

	// 编码
	var buf bytes.Buffer
	if err := WritePacket(&buf, auth); err != nil {
		t.Fatalf("Failed to encode AUTH packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("Failed to decode AUTH packet: %v", err)
	}

	authDecoded, ok := decoded.(*AuthPacket)
	if !ok {
		t.Fatalf("Expected *AuthPacket, got %T", decoded)
	}

	// 验证
	if authDecoded.ReasonCode != ReasonContinueAuth {
		t.Errorf("Expected reason code %v, got %v", ReasonContinueAuth, authDecoded.ReasonCode)
	}
	if authDecoded.Properties.AuthMethod != "oauth2" {
		t.Errorf("Expected auth method 'oauth2', got '%s'", authDecoded.Properties.AuthMethod)
	}
	if !bytes.Equal(authDecoded.Properties.AuthData, []byte("test-auth-data")) {
		t.Errorf("Auth data mismatch")
	}
	if authDecoded.Properties.UserProperties["client_type"] != "mobile" {
		t.Errorf("User property mismatch")
	}
}

func TestTracePacket(t *testing.T) {
	// 创建 TRACE 包
	trace := NewTracePacket("trace-123", "publish", "client-001")
	trace.Topic = "sensors/temperature"
	trace.Timestamp = 1609459200000 // 2021-01-01 00:00:00 UTC
	trace.Details = map[string]string{
		"qos":     "1",
		"size":    "256",
		"latency": "15ms",
		"core_id": "core-01",
	}

	// 编码
	var buf bytes.Buffer
	if err := WritePacket(&buf, trace); err != nil {
		t.Fatalf("Failed to encode TRACE packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("Failed to decode TRACE packet: %v", err)
	}

	traceDecoded, ok := decoded.(*TracePacket)
	if !ok {
		t.Fatalf("Expected *TracePacket, got %T", decoded)
	}

	// 验证
	if traceDecoded.TraceID != "trace-123" {
		t.Errorf("Expected trace ID 'trace-123', got '%s'", traceDecoded.TraceID)
	}
	if traceDecoded.Event != "publish" {
		t.Errorf("Expected event 'publish', got '%s'", traceDecoded.Event)
	}
	if traceDecoded.ClientID != "client-001" {
		t.Errorf("Expected client ID 'client-001', got '%s'", traceDecoded.ClientID)
	}
	if traceDecoded.Topic != "sensors/temperature" {
		t.Errorf("Expected topic 'sensors/temperature', got '%s'", traceDecoded.Topic)
	}
	if traceDecoded.Timestamp != 1609459200000 {
		t.Errorf("Expected timestamp 1609459200000, got %d", traceDecoded.Timestamp)
	}
	if len(traceDecoded.Details) != 4 {
		t.Errorf("Expected 4 details, got %d", len(traceDecoded.Details))
	}
	if traceDecoded.Details["qos"] != "1" {
		t.Errorf("Detail 'qos' mismatch")
	}
	if traceDecoded.Details["latency"] != "15ms" {
		t.Errorf("Detail 'latency' mismatch")
	}
}

func TestTracePacketEmptyDetails(t *testing.T) {
	// 创建没有详情的 TRACE 包
	trace := NewTracePacket("trace-456", "subscribe", "client-002")
	trace.Timestamp = 1609459200000

	// 编码
	var buf bytes.Buffer
	if err := WritePacket(&buf, trace); err != nil {
		t.Fatalf("Failed to encode TRACE packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("Failed to decode TRACE packet: %v", err)
	}

	traceDecoded, ok := decoded.(*TracePacket)
	if !ok {
		t.Fatalf("Expected *TracePacket, got %T", decoded)
	}

	// 验证
	if traceDecoded.TraceID != "trace-456" {
		t.Errorf("Expected trace ID 'trace-456', got '%s'", traceDecoded.TraceID)
	}
	if len(traceDecoded.Details) != 0 {
		t.Errorf("Expected 0 details, got %d", len(traceDecoded.Details))
	}
}

func TestAuthPacketMinimal(t *testing.T) {
	// 创建最小的 AUTH 包
	auth := NewAuthPacket(ReasonSuccess)

	// 编码
	var buf bytes.Buffer
	if err := WritePacket(&buf, auth); err != nil {
		t.Fatalf("Failed to encode AUTH packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("Failed to decode AUTH packet: %v", err)
	}

	authDecoded, ok := decoded.(*AuthPacket)
	if !ok {
		t.Fatalf("Expected *AuthPacket, got %T", decoded)
	}

	// 验证
	if authDecoded.ReasonCode != ReasonSuccess {
		t.Errorf("Expected reason code %v, got %v", ReasonSuccess, authDecoded.ReasonCode)
	}
}

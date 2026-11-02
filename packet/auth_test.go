package packet

import (
	"bytes"
	"encoding/binary"
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
	if err := WritePacket(&buf, auth, 0); err != nil {
		t.Fatalf("Failed to encode AUTH packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf, 0)
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
	if err := WritePacket(&buf, trace, 0); err != nil {
		t.Fatalf("Failed to encode TRACE packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf, 0)
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
	if err := WritePacket(&buf, trace, 0); err != nil {
		t.Fatalf("Failed to encode TRACE packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf, 0)
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
	if err := WritePacket(&buf, auth, 0); err != nil {
		t.Fatalf("Failed to encode AUTH packet: %v", err)
	}

	// 解码
	decoded, err := ReadPacket(&buf, 0)
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

// TestAuthPropertiesWireFormat 校验 AUTH 属性与其它包使用相同帧格式：
// [Len(4B)][Bitmask(2B)][按位序的值]，以便复用通用属性解码。
func TestAuthPropertiesWireFormat(t *testing.T) {
	auth := NewAuthPacket(ReasonContinueAuth)
	auth.Properties.AuthMethod = "m"

	var buf bytes.Buffer
	if err := WritePacket(&buf, auth, 0); err != nil {
		t.Fatalf("Failed to encode AUTH packet: %v", err)
	}

	body := buf.Bytes()[5:] // 跳过 5 字节固定头
	if len(body) < 7 {
		t.Fatalf("AUTH body too short: %d", len(body))
	}
	if body[0] != byte(ReasonContinueAuth) {
		t.Fatalf("reason code = %d, want %d", body[0], ReasonContinueAuth)
	}

	propLen := binary.BigEndian.Uint32(body[1:5])
	if int(propLen) != len(body)-5 {
		t.Fatalf("property length = %d, want %d", propLen, len(body)-5)
	}

	flags := binary.BigEndian.Uint16(body[5:7])
	if flags&authPropAuthMethod == 0 {
		t.Fatalf("auth method flag not set: 0x%04x", flags)
	}
	if flags&^uint16(authPropAuthMethod|authPropAuthData|authPropUserProperties) != 0 {
		t.Fatalf("unexpected flag bits: 0x%04x", flags)
	}
}

// TestAuthPropertiesRejectsTrailingBytes 校验属性块内部的多余字节会被拒绝。
func TestAuthPropertiesRejectsTrailingBytes(t *testing.T) {
	var buf bytes.Buffer
	_ = EncodeUint32(&buf, 3) // 声明 3 字节属性体
	_ = EncodeUint16(&buf, 0) // 位掩码：无任何属性
	buf.WriteByte(0xFF)       // 多余字节

	p := NewAuthProperties()
	if err := p.Decode(&buf); err != ErrMalformedPacket {
		t.Fatalf("Decode error = %v, want ErrMalformedPacket", err)
	}
}

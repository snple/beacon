package packet

import (
	"bytes"
	"testing"
)

func TestConnectPacket_EncodeDecodeBasic(t *testing.T) {
	// 创建基本的 CONNECT 包
	pkt := NewConnectPacket()
	pkt.ClientID = "test-client-123"
	pkt.KeepAlive = 60

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(&buf); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(&buf, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if decoded.ProtocolName != pkt.ProtocolName {
		t.Errorf("协议名称不匹配: 期望 %s, 得到 %s", pkt.ProtocolName, decoded.ProtocolName)
	}
	if decoded.ProtocolVersion != pkt.ProtocolVersion {
		t.Errorf("协议版本不匹配: 期望 %d, 得到 %d", pkt.ProtocolVersion, decoded.ProtocolVersion)
	}
	if decoded.ClientID != pkt.ClientID {
		t.Errorf("ClientID 不匹配: 期望 %s, 得到 %s", pkt.ClientID, decoded.ClientID)
	}
	if decoded.KeepAlive != pkt.KeepAlive {
		t.Errorf("KeepAlive 不匹配: 期望 %d, 得到 %d", pkt.KeepAlive, decoded.KeepAlive)
	}
	if decoded.KeepSession != pkt.KeepSession {
		t.Errorf("KeepSession 标志不匹配")
	}
}

func TestConnectPacket_EncodeDecodeWithCleanSession(t *testing.T) {
	// 创建带 CleanSession 标志的 CONNECT 包
	pkt := NewConnectPacket()
	pkt.ClientID = "test-client-456"
	pkt.KeepSession = true
	pkt.KeepAlive = 120

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(&buf); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(&buf, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if !decoded.KeepSession {
		t.Error("KeepSession 标志应该为 true")
	}
	if decoded.ClientID != pkt.ClientID {
		t.Errorf("ClientID 不匹配: 期望 %s, 得到 %s", pkt.ClientID, decoded.ClientID)
	}
	if decoded.KeepAlive != pkt.KeepAlive {
		t.Errorf("KeepAlive 不匹配: 期望 %d, 得到 %d", pkt.KeepAlive, decoded.KeepAlive)
	}
}

func TestConnectPacket_EncodeDecodeWithWill(t *testing.T) {
	// 创建带遗嘱消息的 CONNECT 包
	pkt := NewConnectPacket()
	pkt.ClientID = "test-client-with-will"
	pkt.Will = true
	pkt.WillPacket = NewPublishPacket("client/status", []byte("offline"))
	pkt.WillPacket.QoS = QoS1
	pkt.WillPacket.Retain = true
	pkt.WillPacket.Properties.ContentType = "text/plain"

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(&buf); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(&buf, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if !decoded.Will {
		t.Error("Will 标志应该为 true")
	}
	if decoded.WillPacket == nil {
		t.Fatal("WillPacket 不应该为 nil")
	}
	if decoded.WillPacket.QoS != QoS1 {
		t.Errorf("WillPacket.QoS 不匹配: 期望 %v, 得到 %v", QoS1, decoded.WillPacket.QoS)
	}
	if !decoded.WillPacket.Retain {
		t.Error("WillPacket.Retain 应该为 true")
	}
	if decoded.WillPacket.Topic != pkt.WillPacket.Topic {
		t.Errorf("WillPacket.Topic 不匹配: 期望 %s, 得到 %s", pkt.WillPacket.Topic, decoded.WillPacket.Topic)
	}
	if !bytes.Equal(decoded.WillPacket.Payload, pkt.WillPacket.Payload) {
		t.Errorf("WillPacket.Payload 不匹配: 期望 %v, 得到 %v", pkt.WillPacket.Payload, decoded.WillPacket.Payload)
	}
	if decoded.WillPacket.Properties.ContentType != pkt.WillPacket.Properties.ContentType {
		t.Errorf("WillPacket.Properties.ContentType 不匹配: 期望 %s, 得到 %s",
			pkt.WillPacket.Properties.ContentType, decoded.WillPacket.Properties.ContentType)
	}
}

func TestConnectPacket_EncodeDecodeWithProperties(t *testing.T) {
	// 创建带属性的 CONNECT 包
	pkt := NewConnectPacket()
	pkt.ClientID = "test-client-props"

	// 设置一些属性
	sessionExpiry := uint32(3600)
	pkt.Properties.SessionExpiry = &sessionExpiry
	pkt.Properties.AuthMethod = "SCRAM-SHA-256"
	pkt.Properties.AuthData = []byte("auth-data")
	pkt.Properties.TraceID = "trace-123"
	pkt.Properties.UserProperties["key1"] = "value1"
	pkt.Properties.UserProperties["key2"] = "value2"

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	decoded := &ConnectPacket{}
	header := FixedHeader{}
	if err := header.Decode(&buf); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(&buf, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证属性
	if decoded.Properties.SessionExpiry == nil || *decoded.Properties.SessionExpiry != sessionExpiry {
		t.Errorf("SessionExpiry 不匹配")
	}
	if decoded.Properties.AuthMethod != pkt.Properties.AuthMethod {
		t.Errorf("AuthMethod 不匹配: 期望 %s, 得到 %s",
			pkt.Properties.AuthMethod, decoded.Properties.AuthMethod)
	}
	if !bytes.Equal(decoded.Properties.AuthData, pkt.Properties.AuthData) {
		t.Errorf("AuthData 不匹配")
	}
	if decoded.Properties.TraceID != pkt.Properties.TraceID {
		t.Errorf("TraceID 不匹配: 期望 %s, 得到 %s",
			pkt.Properties.TraceID, decoded.Properties.TraceID)
	}
	if len(decoded.Properties.UserProperties) != 2 {
		t.Errorf("UserProperties 数量不匹配: 期望 2, 得到 %d", len(decoded.Properties.UserProperties))
	}
	if decoded.Properties.UserProperties["key1"] != "value1" {
		t.Error("UserProperties key1 不匹配")
	}
	if decoded.Properties.UserProperties["key2"] != "value2" {
		t.Error("UserProperties key2 不匹配")
	}
}

func TestConnackPacket_EncodeDecode(t *testing.T) {
	// 创建 CONNACK 包
	pkt := NewConnackPacket(ReasonSuccess)
	pkt.SessionPresent = true

	maxPacketSize := uint32(1024000)
	pkt.Properties.MaxPacketSize = &maxPacketSize
	pkt.Properties.ClientID = "assigned-client-789"

	// 编码
	var buf bytes.Buffer
	if err := pkt.Encode(&buf); err != nil {
		t.Fatalf("编码失败: %v", err)
	}

	// 解码
	decoded := &ConnackPacket{}
	header := FixedHeader{}
	if err := header.Decode(&buf); err != nil {
		t.Fatalf("解码固定头部失败: %v", err)
	}

	if err := decoded.Decode(&buf, header); err != nil {
		t.Fatalf("解码失败: %v", err)
	}

	// 验证
	if !decoded.SessionPresent {
		t.Error("SessionPresent 应该为 true")
	}
	if decoded.ReasonCode != ReasonSuccess {
		t.Errorf("ReasonCode 不匹配: 期望 %v, 得到 %v", ReasonSuccess, decoded.ReasonCode)
	}
	if decoded.Properties.MaxPacketSize == nil || *decoded.Properties.MaxPacketSize != maxPacketSize {
		t.Error("MaxPacketSize 不匹配")
	}
	if decoded.Properties.ClientID != pkt.Properties.ClientID {
		t.Errorf("ClientID 不匹配: 期望 %s, 得到 %s",
			pkt.Properties.ClientID, decoded.Properties.ClientID)
	}
}

func TestConnackPacket_ErrorReasonCode(t *testing.T) {
	// 测试错误原因码
	errorCodes := []ReasonCode{
		ReasonNotAuthorized,
		ReasonServerUnavailable,
		ReasonClientIDNotValid,
	}

	for _, code := range errorCodes {
		pkt := NewConnackPacket(code)
		pkt.SessionPresent = false

		// 编码
		var buf bytes.Buffer
		if err := pkt.Encode(&buf); err != nil {
			t.Fatalf("编码失败 (code=%v): %v", code, err)
		}

		// 解码
		decoded := &ConnackPacket{}
		header := FixedHeader{}
		if err := header.Decode(&buf); err != nil {
			t.Fatalf("解码固定头部失败 (code=%v): %v", code, err)
		}

		if err := decoded.Decode(&buf, header); err != nil {
			t.Fatalf("解码失败 (code=%v): %v", code, err)
		}

		// 验证
		if decoded.ReasonCode != code {
			t.Errorf("ReasonCode 不匹配: 期望 %v, 得到 %v", code, decoded.ReasonCode)
		}
		if decoded.SessionPresent {
			t.Error("错误响应的 SessionPresent 应该为 false")
		}
	}
}

func TestConnectPacket_Type(t *testing.T) {
	pkt := NewConnectPacket()
	if pkt.Type() != CONNECT {
		t.Errorf("Type() 返回值不正确: 期望 %v, 得到 %v", CONNECT, pkt.Type())
	}
}

func TestConnackPacket_Type(t *testing.T) {
	pkt := NewConnackPacket(ReasonSuccess)
	if pkt.Type() != CONNACK {
		t.Errorf("Type() 返回值不正确: 期望 %v, 得到 %v", CONNACK, pkt.Type())
	}
}

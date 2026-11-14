package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

// 响应大小预算必须与实际空响应编码一致，否则会多发超过客户端上限的包。
func TestPushOkPropertiesFitClientLimit(t *testing.T) {
	cn := &conn{maxPacketSize: 40}
	resp := packet.NewPushOkPacket(1)
	resp.Properties = map[string]string{packet.PropertyError: "not authorized"}
	cn.fitPushOkProperties(resp)
	if resp.Properties != nil {
		t.Fatalf("oversized error properties were kept: %+v", resp.Properties)
	}
}

func TestResponseOverheadMatchesEncoding(t *testing.T) {
	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	frame, err := packet.EncodePacket(connack, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != connackBaseSize {
		t.Fatalf("connack base size = %d, encoded size = %d", connackBaseSize, len(frame))
	}
	connack.AssignedClientID = "c1"
	connack.Properties = map[string]string{"region": "cn"}
	frame, err = packet.EncodePacket(connack, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != connackEncodedSize("c1", connack.Properties) {
		t.Fatalf("connack encoded size = %d, helper = %d", len(frame), connackEncodedSize("c1", connack.Properties))
	}

	declare := packet.NewDeclareRespPacket(1)
	frame, err = packet.EncodePacket(declare, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != declareRespOverhead {
		t.Fatalf("declare response overhead = %d, encoded size = %d", declareRespOverhead, len(frame))
	}

	pull := packet.NewPullOkPacket(1)
	frame, err = packet.EncodePacket(pull, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(frame) != pullResponseOverhead {
		t.Fatalf("pull response overhead = %d, encoded size = %d", pullResponseOverhead, len(frame))
	}
}

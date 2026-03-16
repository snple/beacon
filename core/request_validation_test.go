package core

import (
	"testing"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

func TestCoreHandleResponse_RejectsMissingTargetClientID(t *testing.T) {
	cl := &Client{
		ID: "handler",
		core: &Core{
			logger: zap.NewNop(),
		},
	}

	err := cl.handleResponse(&packet.ResponsePacket{
		RequestID:      1,
		TargetClientID: "",
		ReasonCode:     packet.ReasonSuccess,
		Properties:     packet.NewResponseProperties(),
	})
	if err != packet.ErrMalformedPacket {
		t.Fatalf("expected ErrMalformedPacket, got %v", err)
	}
}

func TestCoreHandleResponse_RejectsZeroRequestID(t *testing.T) {
	cl := &Client{
		ID: "handler",
		core: &Core{
			logger: zap.NewNop(),
		},
	}

	err := cl.handleResponse(&packet.ResponsePacket{
		RequestID:      0,
		TargetClientID: "requester",
		ReasonCode:     packet.ReasonSuccess,
		Properties:     packet.NewResponseProperties(),
	})
	if err != packet.ErrMalformedPacket {
		t.Fatalf("expected ErrMalformedPacket, got %v", err)
	}
}

package client

import (
	"strings"
	"testing"
)

func TestOptionsDefaults(t *testing.T) {
	opts := NewClientOptions()
	if opts.KeepAlive == 0 || opts.MaxPacketSize == 0 || opts.ConnectTimeout == 0 {
		t.Fatalf("defaults not applied: %+v", opts)
	}
	if opts.DefaultMaxRecords == 0 || opts.DefaultWaitMs == 0 {
		t.Fatalf("pull defaults not applied: %+v", opts)
	}
	if !opts.AutoReconnect {
		t.Fatal("auto reconnect should default to true")
	}
}

func TestOptionsValidate(t *testing.T) {
	o := NewClientOptions().WithClientID("ok")
	if err := o.Validate(); err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}

	o = NewClientOptions().WithClientID("bad\x01id")
	if err := o.Validate(); err == nil {
		t.Fatal("expected error for control character in clientID")
	}

	long := strings.Repeat("a", 300)
	o = NewClientOptions().WithClientID(long)
	if err := o.Validate(); err == nil {
		t.Fatal("expected error for over-long clientID")
	}
}

func TestReasonToError(t *testing.T) {
	if err := reasonToError(0x00); err != nil {
		t.Fatalf("success should map to nil, got %v", err)
	}
	if err := reasonToError(0x10); err != nil {
		t.Fatalf("no-match is not an error, got %v", err)
	}
	if err := reasonToError(0x8E); err != ErrSessionTakenOver {
		t.Fatalf("session taken over: %v", err)
	}
	if err := reasonToError(0xA2); err != ErrKicked {
		t.Fatalf("kicked: %v", err)
	}
	if err := reasonToError(0xA1); err != ErrCommitRequired {
		t.Fatalf("commit required: %v", err)
	}
	if err := reasonToError(0x87); err == nil {
		t.Fatal("not authorized should be an error")
	}
}

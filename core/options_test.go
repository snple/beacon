package core

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestCoreOptionsBuilder 测试 CoreOptions Builder 模式
func TestCoreOptionsBuilder(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	opts := NewCoreOptions().
		WithMaxClients(5000).
		WithMaxPacketSize(2097152).
		WithConnectTimeout(30).
		WithKeepAlive(120).
		WithMaxSessionTimeout(7200).
		WithReceiveWindow(500).
		WithDefaultMessageExpiry(48 * time.Hour).
		WithExpiredCheckInterval(300 * time.Second).
		WithRetransmitInterval(60 * time.Second).
		WithRetainEnabled(false).
		WithRequestQueueSize(200).
		WithLogger(logger)

	if opts.MaxClients != 5000 {
		t.Errorf("expected maxClients 5000, got %d", opts.MaxClients)
	}
	if opts.MaxPacketSize != 2097152 {
		t.Errorf("expected maxPacketSize 2097152, got %d", opts.MaxPacketSize)
	}
	if opts.ConnectTimeout != 30 {
		t.Errorf("expected connectTimeout 30, got %d", opts.ConnectTimeout)
	}
	if opts.KeepAlive != 120 {
		t.Errorf("expected keepAlive 120, got %d", opts.KeepAlive)
	}
	if opts.MaxSessionTimeout != 7200 {
		t.Errorf("expected maxSessionTimeout 7200, got %d", opts.MaxSessionTimeout)
	}
	if opts.ReceiveWindow != 500 {
		t.Errorf("expected receiveWindow 500, got %d", opts.ReceiveWindow)
	}
	if opts.DefaultMessageExpiry != 48*time.Hour {
		t.Errorf("expected defaultMessageExpiry 48h, got %v", opts.DefaultMessageExpiry)
	}
	if opts.ExpiredCheckInterval != 300*time.Second {
		t.Errorf("expected expiredCheckInterval 300s, got %v", opts.ExpiredCheckInterval)
	}
	if opts.RetransmitInterval != 60*time.Second {
		t.Errorf("expected retransmitInterval 60s, got %v", opts.RetransmitInterval)
	}
	if opts.RetainEnabled {
		t.Error("expected retainEnabled false")
	}
	if opts.RequestQueueSize != 200 {
		t.Errorf("expected requestQueueSize 200, got %d", opts.RequestQueueSize)
	}
	if opts.Logger != logger {
		t.Error("expected logger to be set")
	}
}

// TestNewWithOptions 测试使用 Builder 模式创建 Core
func TestNewWithOptions(t *testing.T) {
	opts := NewCoreOptions().
		WithMaxClients(100)

	core, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("failed to create Core: %v", err)
	}

	if core == nil {
		t.Fatal("Core should not be nil")
	}

	if core.options.MaxClients != 100 {
		t.Errorf("expected maxClients 100, got %d", core.options.MaxClients)
	}
}

// TestNewWithOptionsNil 测试传入 nil 选项
func TestNewWithOptionsNil(t *testing.T) {
	core, err := NewWithOptions(nil)
	if err != nil {
		t.Fatalf("failed to create Core with nil options: %v", err)
	}

	if core == nil {
		t.Fatal("Core should not be nil")
	}
}

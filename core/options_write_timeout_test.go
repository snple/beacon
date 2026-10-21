package core

import (
	"testing"
	"time"
)

func TestCoreOptions_WriteTimeoutAndOfflineSessionDefaults(t *testing.T) {
	opts := NewCoreOptions()
	if opts.WriteTimeout != 10*time.Second {
		t.Fatalf("expected default writeTimeout 10s, got %v", opts.WriteTimeout)
	}
	if opts.MaxOfflineSessions != 10000 {
		t.Fatalf("expected default maxOfflineSessions 10000, got %d", opts.MaxOfflineSessions)
	}

	// 0 表示显式关闭限制，必须能通过校验。
	opts.WithWriteTimeout(0).WithMaxOfflineSessions(0)
	if err := opts.Validate(); err != nil {
		t.Fatalf("expected zero values to be valid, got %v", err)
	}

	if err := NewCoreOptions().WithWriteTimeout(-time.Second).Validate(); err == nil {
		t.Fatal("expected negative writeTimeout to be rejected")
	}
	if err := NewCoreOptions().WithMaxOfflineSessions(-1).Validate(); err == nil {
		t.Fatal("expected negative maxOfflineSessions to be rejected")
	}
}

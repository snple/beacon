package client

import (
	"testing"
	"time"
)

func TestClientOptions_WriteTimeoutDefault(t *testing.T) {
	opts := NewClientOptions()
	if opts.WriteTimeout != 10*time.Second {
		t.Fatalf("expected default writeTimeout 10s, got %v", opts.WriteTimeout)
	}

	opts.WithWriteTimeout(0)
	if opts.WriteTimeout != 0 {
		t.Fatalf("expected writeTimeout to be disabled, got %v", opts.WriteTimeout)
	}
}

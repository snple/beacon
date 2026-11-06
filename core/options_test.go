package core

import (
	"testing"

	"github.com/snple/beacon/packet"
)

func TestCoreOptionsChannelLength(t *testing.T) {
	def := NewCoreOptions()
	def.applyDefaults()
	if def.MaxChannelLength != packet.MinChannelLength {
		t.Fatalf("default maxChannelLength=%d, want %d", def.MaxChannelLength, packet.MinChannelLength)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("default options invalid: %v", err)
	}

	valid := []int{
		packet.MinChannelLength,
		(packet.MinChannelLength + packet.MaxChannelLength) / 2,
		packet.MaxChannelLength,
	}
	for _, n := range valid {
		o := NewCoreOptions().WithMaxChannelLength(n)
		o.applyDefaults()
		if o.MaxChannelLength != n {
			t.Fatalf("maxChannelLength=%d, want %d", o.MaxChannelLength, n)
		}
		if err := o.Validate(); err != nil {
			t.Fatalf("maxChannelLength=%d should be valid: %v", n, err)
		}
	}

	invalid := []int{packet.MinChannelLength - 1, packet.MaxChannelLength + 1}
	for _, n := range invalid {
		o := NewCoreOptions().WithMaxChannelLength(n)
		o.applyDefaults()
		if err := o.Validate(); err == nil {
			t.Fatalf("maxChannelLength=%d should be rejected", n)
		}
	}

	// 0 表示使用默认值。
	o := NewCoreOptions().WithMaxChannelLength(0)
	o.applyDefaults()
	if o.MaxChannelLength != packet.MinChannelLength {
		t.Fatalf("zero maxChannelLength should fall back to default, got %d", o.MaxChannelLength)
	}
}

func TestCoreOptionsMaxBatch(t *testing.T) {
	def := NewCoreOptions()
	def.applyDefaults()
	if def.MaxBatch != 16 {
		t.Fatalf("default maxBatch=%d, want 16", def.MaxBatch)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("default options invalid: %v", err)
	}

	for _, n := range []uint32{1, 16, packet.MaxPushBatch} {
		o := NewCoreOptions().WithMaxBatch(n)
		o.applyDefaults()
		if o.MaxBatch != n {
			t.Fatalf("maxBatch=%d, want %d", o.MaxBatch, n)
		}
		if err := o.Validate(); err != nil {
			t.Fatalf("maxBatch=%d should be valid: %v", n, err)
		}
	}

	o := NewCoreOptions().WithMaxBatch(packet.MaxPushBatch + 1)
	o.applyDefaults()
	if err := o.Validate(); err == nil {
		t.Fatalf("maxBatch=%d should be rejected", packet.MaxPushBatch+1)
	}
}

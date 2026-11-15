package client

import (
	"errors"
	"strings"
	"testing"

	"github.com/snple/beacon/packet"
)

func TestOptionsDefaults(t *testing.T) {
	opts := NewClientOptions()
	if opts.KeepAlive == 0 || opts.MaxRecvPacketSize == 0 || opts.ConnectTimeout == 0 {
		t.Fatalf("defaults not applied: %+v", opts)
	}
	if opts.DefaultPullWaitMs == 0 {
		t.Fatalf("pull defaults not applied: %+v", opts)
	}
	if !opts.AutoReconnect {
		t.Fatal("auto reconnect should default to true")
	}
}

func TestOptionsValidate(t *testing.T) {
	// 空 ClientID 合法：由服务端分配。
	if err := NewClientOptions().Validate(); err != nil {
		t.Fatalf("empty clientID should be allowed: %v", err)
	}

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

	o = NewClientOptions().WithChannels(Filter("a/#"))
	if err := o.Validate(); err == nil {
		t.Fatal("expected error for invalid channel filter")
	}

	o = NewClientOptions().WithReconnectBackoff(2, 1)
	if err := o.Validate(); err == nil {
		t.Fatal("expected error for inverted reconnect backoff")
	}
}

func TestReasonToError(t *testing.T) {
	if err := reasonToError(0x00); err != nil {
		t.Fatalf("success should map to nil, got %v", err)
	}
	if err := reasonToError(0x10); err != nil {
		t.Fatalf("no-match is not an error, got %v", err)
	}

	cases := []struct {
		reason packet.ReasonCode
		want   error
	}{
		{packet.ReasonSessionTakenOver, ErrSessionTakenOver},
		{packet.ReasonKicked, ErrKicked},
		{packet.ReasonCommitRequired, ErrCommitRequired},
		{packet.ReasonExpired, ErrExpired},
		{packet.ReasonNotAuthorized, ErrNotAuthorized},
		{packet.ReasonPacketTooLarge, ErrPacketTooLarge},
		{packet.ReasonQuotaExceeded, ErrQuotaExceeded},
		{packet.ReasonConsumerBacklog, ErrConsumerBacklog},
		{packet.ReasonTargetNotFound, ErrTargetNotFound},
		{packet.ReasonChannelInvalid, ErrChannelInvalid},
		{packet.ReasonChannelFilterInvalid, ErrChannelInvalid},
	}
	for _, tc := range cases {
		err := reasonToError(tc.reason)
		if !errors.Is(err, tc.want) {
			t.Fatalf("reason %s: got %v, want %v", tc.reason, err, tc.want)
		}
	}

	err := reasonToErrorWithDetail(packet.ReasonNotAuthorized, "channel denied")
	var se *ServerError
	if !errors.As(err, &se) || se.Detail != "channel denied" {
		t.Fatalf("detail not preserved: %#v", err)
	}
}

func TestOptionsChannelsReplaceAndAppend(t *testing.T) {
	o := NewClientOptions().
		WithChannels(Filter("a/**")).
		AddChannels(TargetOnly("b/1"), TargetOnly("b/2"))
	if len(o.Channels) != 3 {
		t.Fatalf("channels: %+v", o.Channels)
	}
	if o.Channels[0].TargetOnly || !o.Channels[1].TargetOnly || !o.Channels[2].TargetOnly {
		t.Fatalf("target-only flags: %+v", o.Channels)
	}

	// WithChannels 是替换语义。
	o.WithChannels(Filter("c/**"))
	if len(o.Channels) != 1 || o.Channels[0].Filter != "c/**" {
		t.Fatalf("WithChannels should replace: %+v", o.Channels)
	}
}

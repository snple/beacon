package core

import (
	"errors"
	"testing"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
)

// ============================================================================
// MessageHandler 测试
// ============================================================================

func TestMessageHandlerFunc_OnPublish(t *testing.T) {
	tests := []struct {
		name        string
		publishFunc func(ctx *PublishContext) error
		expectErr   bool
	}{
		{
			name:        "nil function allows publish",
			publishFunc: nil,
			expectErr:   false,
		},
		{
			name: "function returns nil allows publish",
			publishFunc: func(ctx *PublishContext) error {
				return nil
			},
			expectErr: false,
		},
		{
			name: "function returns error rejects publish",
			publishFunc: func(ctx *PublishContext) error {
				return errors.New("publish rejected")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &MessageHandlerFunc{
				PublishFunc: tt.publishFunc,
			}
			ctx := &PublishContext{
				ClientID: "test-client",
				Packet:   packet.NewPublishPacket("test/topic", []byte("test")),
			}
			err := handler.OnPublish(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("OnPublish() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestMessageHandlerFunc_OnDeliver(t *testing.T) {
	tests := []struct {
		name         string
		deliverFunc  func(ctx *DeliverContext) bool
		expectResult bool
	}{
		{
			name:         "nil function allows delivery",
			deliverFunc:  nil,
			expectResult: true,
		},
		{
			name: "function returns true allows delivery",
			deliverFunc: func(ctx *DeliverContext) bool {
				return true
			},
			expectResult: true,
		},
		{
			name: "function returns false blocks delivery",
			deliverFunc: func(ctx *DeliverContext) bool {
				return false
			},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &MessageHandlerFunc{
				DeliverFunc: tt.deliverFunc,
			}
			ctx := &DeliverContext{
				ClientID: "test-client",
				Packet:   packet.NewPublishPacket("test/topic", []byte("test")),
			}
			result := handler.OnDeliver(ctx)
			if result != tt.expectResult {
				t.Errorf("OnDeliver() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

// ============================================================================
// ConnectionHandler 测试
// ============================================================================

func TestConnectionHandlerFunc_OnConnect(t *testing.T) {
	called := false
	handler := &ConnectionHandlerFunc{
		ConnectFunc: func(ctx *ConnectContext) {
			called = true
			if ctx.ClientID != "test-client" {
				t.Errorf("ClientID = %q, want %q", ctx.ClientID, "test-client")
			}
		},
	}

	connectPacket := packet.NewConnectPacket()
	connectPacket.ClientID = "test-client"
	ctx := &ConnectContext{
		ClientID:   "test-client",
		RemoteAddr: "127.0.0.1:12345",
		Packet:     connectPacket,
	}
	handler.OnConnect(ctx)

	if !called {
		t.Error("OnConnect was not called")
	}
}

func TestConnectionHandlerFunc_OnConnect_NilFunc(t *testing.T) {
	handler := &ConnectionHandlerFunc{}
	ctx := &ConnectContext{ClientID: "test-client", Packet: packet.NewConnectPacket()}
	// Should not panic
	handler.OnConnect(ctx)
}

func TestConnectionHandlerFunc_OnDisconnect(t *testing.T) {
	called := false
	handler := &ConnectionHandlerFunc{
		DisconnectFunc: func(ctx *DisconnectContext) {
			called = true
			if ctx.ClientID != "test-client" {
				t.Errorf("ClientID = %q, want %q", ctx.ClientID, "test-client")
			}
		},
	}

	ctx := &DisconnectContext{
		ClientID: "test-client",
		Packet:   packet.NewDisconnectPacket(packet.ReasonNormalDisconnect),
		Duration: 3600,
	}
	handler.OnDisconnect(ctx)

	if !called {
		t.Error("OnDisconnect was not called")
	}
}

// ============================================================================
// SubscriptionHandler 测试
// ============================================================================

func TestSubscriptionHandlerFunc_OnSubscribe(t *testing.T) {
	tests := []struct {
		name          string
		subscribeFunc func(ctx *SubscribeContext) error
		expectErr     bool
	}{
		{
			name:          "nil function allows subscribe",
			subscribeFunc: nil,
			expectErr:     false,
		},
		{
			name: "function returns nil allows subscribe",
			subscribeFunc: func(ctx *SubscribeContext) error {
				return nil
			},
			expectErr: false,
		},
		{
			name: "function returns error rejects subscribe",
			subscribeFunc: func(ctx *SubscribeContext) error {
				return errors.New("subscribe rejected")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &SubscriptionHandlerFunc{
				SubscribeFunc: tt.subscribeFunc,
			}
			subPacket := packet.NewSubscribePacket(nson.NewId())
			subPacket.Subscriptions = []packet.Subscription{
				{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS1}},
			}
			ctx := &SubscribeContext{
				ClientID:     "test-client",
				Packet:       subPacket,
				Subscription: &subPacket.Subscriptions[0],
			}
			err := handler.OnSubscribe(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("OnSubscribe() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestSubscriptionHandlerFunc_OnUnsubscribe(t *testing.T) {
	called := false
	handler := &SubscriptionHandlerFunc{
		UnsubscribeFunc: func(ctx *UnsubscribeContext) {
			called = true
			if ctx.ClientID != "test-client" {
				t.Errorf("ClientID = %q, want %q", ctx.ClientID, "test-client")
			}
			if ctx.Topic != "test/topic" {
				t.Errorf("Topic = %q, want %q", ctx.Topic, "test/topic")
			}
		},
	}

	unsubPacket := packet.NewUnsubscribePacket(nson.NewId())
	unsubPacket.Topics = []string{"test/topic"}
	ctx := &UnsubscribeContext{
		ClientID: "test-client",
		Packet:   unsubPacket,
		Topic:    "test/topic",
	}
	handler.OnUnsubscribe(ctx)

	if !called {
		t.Error("OnUnsubscribe was not called")
	}
}

// ============================================================================
// AuthHandler 测试
// ============================================================================

func TestAuthHandlerFunc_OnConnect(t *testing.T) {
	tests := []struct {
		name        string
		connectFunc func(ctx *AuthConnectContext) error
		expectErr   bool
	}{
		{
			name:        "nil function allows connection",
			connectFunc: nil,
			expectErr:   false,
		},
		{
			name: "valid credentials allows connection",
			connectFunc: func(ctx *AuthConnectContext) error {
				if ctx.ClientID == "valid-client" {
					return nil
				}
				return errors.New("invalid client")
			},
			expectErr: false,
		},
		{
			name: "invalid credentials rejects connection",
			connectFunc: func(ctx *AuthConnectContext) error {
				return errors.New("authentication failed")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &AuthHandlerFunc{
				ConnectFunc: tt.connectFunc,
			}
			connectPacket := packet.NewConnectPacket()
			connectPacket.ClientID = "valid-client"
			ctx := &AuthConnectContext{
				ClientID:   "valid-client",
				RemoteAddr: "127.0.0.1:12345",
				Packet:     connectPacket,
			}
			err := handler.OnConnect(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("OnConnect() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestAuthHandlerFunc_OnAuth(t *testing.T) {
	tests := []struct {
		name           string
		authFunc       func(ctx *AuthContext) (bool, []byte, error)
		expectContinue bool
		expectData     []byte
		expectErr      bool
	}{
		{
			name:           "nil function returns not supported error",
			authFunc:       nil,
			expectContinue: false,
			expectData:     nil,
			expectErr:      true,
		},
		{
			name: "continue authentication",
			authFunc: func(ctx *AuthContext) (bool, []byte, error) {
				return true, []byte("challenge"), nil
			},
			expectContinue: true,
			expectData:     []byte("challenge"),
			expectErr:      false,
		},
		{
			name: "authentication complete",
			authFunc: func(ctx *AuthContext) (bool, []byte, error) {
				return false, []byte("success"), nil
			},
			expectContinue: false,
			expectData:     []byte("success"),
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &AuthHandlerFunc{
				AuthFunc: tt.authFunc,
			}
			authPacket := packet.NewAuthPacket(packet.ReasonContinueAuth)
			authPacket.Properties = packet.NewAuthProperties()
			authPacket.Properties.AuthMethod = "SCRAM-SHA-256"
			authPacket.Properties.AuthData = []byte("auth-data")
			ctx := &AuthContext{
				ClientID:   "test-client",
				RemoteAddr: "127.0.0.1:12345",
				Packet:     authPacket,
			}
			continued, data, err := handler.OnAuth(ctx)
			if continued != tt.expectContinue {
				t.Errorf("OnAuth() continued = %v, want %v", continued, tt.expectContinue)
			}
			if string(data) != string(tt.expectData) {
				t.Errorf("OnAuth() data = %v, want %v", data, tt.expectData)
			}
			if (err != nil) != tt.expectErr {
				t.Errorf("OnAuth() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

// ============================================================================
// TraceHandler 测试
// ============================================================================

func TestTraceHandlerFunc(t *testing.T) {
	called := false
	var capturedCtx *TraceContext

	handler := TraceHandlerFunc(func(ctx *TraceContext) {
		called = true
		capturedCtx = ctx
	})

	tracePacket := packet.NewTracePacket("trace-123", "message.published", "test-client")
	tracePacket.Topic = "test/topic"
	tracePacket.Timestamp = 1234567890
	tracePacket.Details = map[string]string{"key": "value"}
	ctx := &TraceContext{
		ClientID: "test-client",
		Packet:   tracePacket,
	}
	handler.OnTrace(ctx)

	if !called {
		t.Error("OnTrace was not called")
	}
	if capturedCtx.Packet.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want %q", capturedCtx.Packet.TraceID, "trace-123")
	}
	if capturedCtx.Packet.Event != "message.published" {
		t.Errorf("Event = %q, want %q", capturedCtx.Packet.Event, "message.published")
	}
}

// ============================================================================
// RequestHandler 测试
// ============================================================================

func TestRequestHandlerFunc_OnRequest(t *testing.T) {
	tests := []struct {
		name        string
		requestFunc func(ctx *RequestContext) error
		expectErr   bool
	}{
		{
			name:        "nil function allows request",
			requestFunc: nil,
			expectErr:   false,
		},
		{
			name: "function returns nil allows request",
			requestFunc: func(ctx *RequestContext) error {
				return nil
			},
			expectErr: false,
		},
		{
			name: "function returns error rejects request",
			requestFunc: func(ctx *RequestContext) error {
				return errors.New("request rejected")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &RequestHandlerFunc{
				RequestFunc: tt.requestFunc,
			}
			reqPacket := packet.NewRequestPacket(123, "user.get", []byte("{}"))
			ctx := &RequestContext{
				ClientID: "test-client",
				Packet:   reqPacket,
			}
			err := handler.OnRequest(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("OnRequest() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestRequestHandlerFunc_OnResponse(t *testing.T) {
	called := false
	var capturedCtx *ResponseContext

	handler := &RequestHandlerFunc{
		ResponseFunc: func(ctx *ResponseContext) {
			called = true
			capturedCtx = ctx
		},
	}

	respPacket := packet.NewResponsePacket(123, "test-client", packet.ReasonSuccess, []byte(`{"id": "123"}`))
	ctx := &ResponseContext{
		ClientID: "test-client",
		Packet:   respPacket,
	}
	handler.OnResponse(ctx)

	if !called {
		t.Error("OnResponse was not called")
	}
	if capturedCtx.ClientID != "test-client" {
		t.Errorf("ClientID = %q, want %q", capturedCtx.ClientID, "test-client")
	}
}

func TestRequestHandlerFunc_OnResponse_NilFunc(t *testing.T) {
	handler := &RequestHandlerFunc{}
	respPacket := packet.NewResponsePacket(123, "test-client", packet.ReasonSuccess, nil)
	ctx := &ResponseContext{ClientID: "test-client", Packet: respPacket}
	// Should not panic
	handler.OnResponse(ctx)
}

// ============================================================================
// Hooks 集成测试
// ============================================================================

func TestHooks_CallOnPublish(t *testing.T) {
	tests := []struct {
		name      string
		hooks     *Hooks
		expectErr bool
	}{
		{
			name:      "nil MessageHandler allows publish",
			hooks:     &Hooks{},
			expectErr: false,
		},
		{
			name: "MessageHandler returns nil allows publish",
			hooks: &Hooks{
				MessageHandler: &MessageHandlerFunc{
					PublishFunc: func(ctx *PublishContext) error {
						return nil
					},
				},
			},
			expectErr: false,
		},
		{
			name: "MessageHandler returns error rejects publish",
			hooks: &Hooks{
				MessageHandler: &MessageHandlerFunc{
					PublishFunc: func(ctx *PublishContext) error {
						return errors.New("rejected")
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &PublishContext{
				ClientID: "test-client",
				Packet:   packet.NewPublishPacket("test/topic", []byte("test")),
			}
			err := tt.hooks.callOnPublish(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("callOnPublish() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestHooks_CallOnDeliver(t *testing.T) {
	tests := []struct {
		name         string
		hooks        *Hooks
		expectResult bool
	}{
		{
			name:         "nil MessageHandler allows delivery",
			hooks:        &Hooks{},
			expectResult: true,
		},
		{
			name: "MessageHandler returns true allows delivery",
			hooks: &Hooks{
				MessageHandler: &MessageHandlerFunc{
					DeliverFunc: func(ctx *DeliverContext) bool {
						return true
					},
				},
			},
			expectResult: true,
		},
		{
			name: "MessageHandler returns false blocks delivery",
			hooks: &Hooks{
				MessageHandler: &MessageHandlerFunc{
					DeliverFunc: func(ctx *DeliverContext) bool {
						return false
					},
				},
			},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &DeliverContext{
				ClientID: "test-client",
				Packet:   packet.NewPublishPacket("test/topic", []byte("test")),
			}
			result := tt.hooks.callOnDeliver(ctx)
			if result != tt.expectResult {
				t.Errorf("callOnDeliver() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

func TestHooks_CallOnConnect(t *testing.T) {
	called := false
	hooks := &Hooks{
		ConnectionHandler: &ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				called = true
			},
		},
	}

	ctx := &ConnectContext{ClientID: "test-client", Packet: packet.NewConnectPacket()}
	hooks.callOnConnect(ctx)

	if !called {
		t.Error("callOnConnect did not call handler")
	}
}

func TestHooks_CallOnDisconnect(t *testing.T) {
	called := false
	hooks := &Hooks{
		ConnectionHandler: &ConnectionHandlerFunc{
			DisconnectFunc: func(ctx *DisconnectContext) {
				called = true
			},
		},
	}

	ctx := &DisconnectContext{ClientID: "test-client", Packet: packet.NewDisconnectPacket(packet.ReasonNormalDisconnect)}
	hooks.callOnDisconnect(ctx)

	if !called {
		t.Error("callOnDisconnect did not call handler")
	}
}

func TestHooks_CallOnSubscribe(t *testing.T) {
	tests := []struct {
		name      string
		hooks     *Hooks
		expectErr bool
	}{
		{
			name:      "nil SubscriptionHandler allows subscribe",
			hooks:     &Hooks{},
			expectErr: false,
		},
		{
			name: "SubscriptionHandler returns error rejects subscribe",
			hooks: &Hooks{
				SubscriptionHandler: &SubscriptionHandlerFunc{
					SubscribeFunc: func(ctx *SubscribeContext) error {
						return errors.New("rejected")
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subPacket := packet.NewSubscribePacket(nson.NewId())
			subPacket.Subscriptions = []packet.Subscription{
				{Topic: "test/topic", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
			}
			ctx := &SubscribeContext{
				ClientID:     "test-client",
				Packet:       subPacket,
				Subscription: &subPacket.Subscriptions[0],
			}
			err := tt.hooks.callOnSubscribe(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("callOnSubscribe() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestHooks_CallOnUnsubscribe(t *testing.T) {
	called := false
	hooks := &Hooks{
		SubscriptionHandler: &SubscriptionHandlerFunc{
			UnsubscribeFunc: func(ctx *UnsubscribeContext) {
				called = true
			},
		},
	}

	unsubPacket := packet.NewUnsubscribePacket(nson.NewId())
	unsubPacket.Topics = []string{"test/topic"}
	ctx := &UnsubscribeContext{
		ClientID: "test-client",
		Packet:   unsubPacket,
		Topic:    "test/topic",
	}
	hooks.callOnUnsubscribe(ctx)

	if !called {
		t.Error("callOnUnsubscribe did not call handler")
	}
}

func TestHooks_CallOnAuth(t *testing.T) {
	tests := []struct {
		name           string
		hooks          *Hooks
		expectContinue bool
		expectErr      bool
	}{
		{
			name:           "nil AuthHandler returns error",
			hooks:          &Hooks{},
			expectContinue: false,
			expectErr:      true,
		},
		{
			name: "AuthHandler continues auth",
			hooks: &Hooks{
				AuthHandler: &AuthHandlerFunc{
					AuthFunc: func(ctx *AuthContext) (bool, []byte, error) {
						return true, nil, nil
					},
				},
			},
			expectContinue: true,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPacket := packet.NewAuthPacket(packet.ReasonContinueAuth)
			ctx := &AuthContext{
				ClientID:   "test-client",
				RemoteAddr: "127.0.0.1:12345",
				Packet:     authPacket,
			}
			continued, _, err := tt.hooks.callOnAuth(ctx)
			if continued != tt.expectContinue {
				t.Errorf("callOnAuth() continued = %v, want %v", continued, tt.expectContinue)
			}
			if (err != nil) != tt.expectErr {
				t.Errorf("callOnAuth() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestHooks_CallOnTrace(t *testing.T) {
	called := false
	hooks := &Hooks{
		TraceHandler: TraceHandlerFunc(func(ctx *TraceContext) {
			called = true
		}),
	}

	tracePacket := packet.NewTracePacket("trace-123", "test.event", "test-client")
	ctx := &TraceContext{
		ClientID: "test-client",
		Packet:   tracePacket,
	}
	hooks.callOnTrace(ctx)

	if !called {
		t.Error("callOnTrace did not call handler")
	}
}

func TestHooks_CallOnRequest(t *testing.T) {
	tests := []struct {
		name      string
		hooks     *Hooks
		expectErr bool
	}{
		{
			name:      "nil RequestHandler allows request",
			hooks:     &Hooks{},
			expectErr: false,
		},
		{
			name: "RequestHandler returns error rejects request",
			hooks: &Hooks{
				RequestHandler: &RequestHandlerFunc{
					RequestFunc: func(ctx *RequestContext) error {
						return errors.New("rejected")
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqPacket := packet.NewRequestPacket(123, "user.get", []byte("{}"))
			ctx := &RequestContext{
				ClientID: "test-client",
				Packet:   reqPacket,
			}
			err := tt.hooks.callOnRequest(ctx)
			if (err != nil) != tt.expectErr {
				t.Errorf("callOnRequest() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

func TestHooks_CallOnResponse(t *testing.T) {
	called := false
	hooks := &Hooks{
		RequestHandler: &RequestHandlerFunc{
			ResponseFunc: func(ctx *ResponseContext) {
				called = true
			},
		},
	}

	respPacket := packet.NewResponsePacket(123, "test-client", packet.ReasonSuccess, nil)
	ctx := &ResponseContext{
		ClientID: "test-client",
		Packet:   respPacket,
	}
	hooks.callOnResponse(ctx)

	if !called {
		t.Error("callOnResponse did not call handler")
	}
}

func TestHooks_CallOnResponse_NilHandler(t *testing.T) {
	hooks := &Hooks{}
	respPacket := packet.NewResponsePacket(123, "test-client", packet.ReasonSuccess, nil)
	ctx := &ResponseContext{ClientID: "test-client", Packet: respPacket}
	// Should not panic
	hooks.callOnResponse(ctx)
}

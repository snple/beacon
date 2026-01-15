package client

import (
	"testing"

	"github.com/snple/beacon/packet"
)

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

	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	connack.SessionPresent = false
	ctx := &ConnectContext{
		ClientID:       "test-client",
		SessionPresent: true,
		Packet:         connack,
	}
	handler.OnConnect(ctx)

	if !called {
		t.Error("OnConnect was not called")
	}
}

func TestConnectionHandlerFunc_OnConnect_NilFunc(t *testing.T) {
	handler := &ConnectionHandlerFunc{}
	ctx := &ConnectContext{ClientID: "test-client"}
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
		Err:      nil,
	}
	handler.OnDisconnect(ctx)

	if !called {
		t.Error("OnDisconnect was not called")
	}
}

// ============================================================================
// AuthHandler 测试
// ============================================================================

func TestAuthHandlerFunc(t *testing.T) {
	tests := []struct {
		name           string
		handler        AuthHandlerFunc
		expectContinue bool
		expectErr      bool
	}{
		{
			name: "continue authentication",
			handler: AuthHandlerFunc(func(ctx *AuthContext) (bool, []byte, error) {
				return true, []byte("challenge"), nil
			}),
			expectContinue: true,
			expectErr:      false,
		},
		{
			name: "authentication complete",
			handler: AuthHandlerFunc(func(ctx *AuthContext) (bool, []byte, error) {
				return false, nil, nil
			}),
			expectContinue: false,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authPkt := packet.NewAuthPacket(packet.ReasonSuccess)
			authPkt.Properties = packet.NewAuthProperties()
			authPkt.Properties.AuthData = []byte("auth-data")
			ctx := &AuthContext{
				ClientID: "test-client",
				Packet:   authPkt,
			}
			continued, _, err := tt.handler.OnAuth(ctx)
			if continued != tt.expectContinue {
				t.Errorf("OnAuth() continued = %v, want %v", continued, tt.expectContinue)
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

	tracePkt := packet.NewTracePacket("trace-123", "message.published", "test-client")
	tracePkt.Topic = "test/topic"
	tracePkt.Timestamp = 1234567890
	tracePkt.Details = map[string]string{"key": "value"}
	ctx := &TraceContext{
		ClientID: "test-client",
		Packet:   tracePkt,
	}
	handler.OnTrace(ctx)

	if !called {
		t.Error("OnTrace was not called")
	}
	if capturedCtx.Packet.TraceID != "trace-123" {
		t.Errorf("TraceID = %q, want %q", capturedCtx.Packet.TraceID, "trace-123")
	}
}

// ============================================================================
// MessageHandler 测试
// ============================================================================

func TestMessageHandlerFunc_OnPublish(t *testing.T) {
	tests := []struct {
		name         string
		handler      MessageHandlerFunc
		expectResult bool
	}{
		{
			name: "allow publish",
			handler: MessageHandlerFunc(func(ctx *PublishContext) bool {
				return true
			}),
			expectResult: true,
		},
		{
			name: "reject publish",
			handler: MessageHandlerFunc(func(ctx *PublishContext) bool {
				return false
			}),
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubPkt := packet.NewPublishPacket("test/topic", []byte("test"))
			ctx := &PublishContext{
				ClientID: "test-client",
				Packet:   pubPkt,
			}
			result := tt.handler.OnPublish(ctx)
			if result != tt.expectResult {
				t.Errorf("OnPublish() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

// ============================================================================
// RequestHandler 测试
// ============================================================================

func TestRequestHandlerFunc_OnRequest(t *testing.T) {
	tests := []struct {
		name         string
		handler      RequestHandlerFunc
		expectResult bool
	}{
		{
			name: "allow request",
			handler: RequestHandlerFunc(func(ctx *RequestContext) bool {
				return true
			}),
			expectResult: true,
		},
		{
			name: "reject request",
			handler: RequestHandlerFunc(func(ctx *RequestContext) bool {
				return false
			}),
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqPkt := packet.NewRequestPacket(1, "test.action", nil)
			ctx := &RequestContext{
				ClientID: "test-client",
				Packet:   reqPkt,
			}
			result := tt.handler.OnRequest(ctx)
			if result != tt.expectResult {
				t.Errorf("OnRequest() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

// ============================================================================
// Hooks 集成测试
// ============================================================================

func TestHooks_CallOnConnect(t *testing.T) {
	called := false
	hooks := &Hooks{
		ConnectionHandler: &ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				called = true
			},
		},
	}

	ctx := &ConnectContext{ClientID: "test-client"}
	hooks.callOnConnect(ctx)

	if !called {
		t.Error("callOnConnect did not call handler")
	}
}

func TestHooks_CallOnConnect_NilHandler(t *testing.T) {
	hooks := &Hooks{}
	ctx := &ConnectContext{ClientID: "test-client"}
	// Should not panic
	hooks.callOnConnect(ctx)
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

	ctx := &DisconnectContext{ClientID: "test-client"}
	hooks.callOnDisconnect(ctx)

	if !called {
		t.Error("callOnDisconnect did not call handler")
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
			name:           "nil AuthHandler returns default",
			hooks:          &Hooks{},
			expectContinue: false,
			expectErr:      false,
		},
		{
			name: "AuthHandler continues auth",
			hooks: &Hooks{
				AuthHandler: AuthHandlerFunc(func(ctx *AuthContext) (bool, []byte, error) {
					return true, nil, nil
				}),
			},
			expectContinue: true,
			expectErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &AuthContext{}
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

	tracePkt := packet.NewTracePacket("trace-123", "test.event", "test-client")
	ctx := &TraceContext{ClientID: "test-client", Packet: tracePkt}
	hooks.callOnTrace(ctx)

	if !called {
		t.Error("callOnTrace did not call handler")
	}
}

func TestHooks_CallOnTrace_NilHandler(t *testing.T) {
	hooks := &Hooks{}
	tracePkt := packet.NewTracePacket("trace-123", "test.event", "test-client")
	ctx := &TraceContext{ClientID: "test-client", Packet: tracePkt}
	// Should not panic
	hooks.callOnTrace(ctx)
}

func TestHooks_CallOnPublish(t *testing.T) {
	tests := []struct {
		name         string
		hooks        *Hooks
		expectResult bool
	}{
		{
			name:         "nil MessageHandler allows publish",
			hooks:        &Hooks{},
			expectResult: true,
		},
		{
			name: "MessageHandler returns true allows publish",
			hooks: &Hooks{
				MessageHandler: MessageHandlerFunc(func(ctx *PublishContext) bool {
					return true
				}),
			},
			expectResult: true,
		},
		{
			name: "MessageHandler returns false blocks publish",
			hooks: &Hooks{
				MessageHandler: MessageHandlerFunc(func(ctx *PublishContext) bool {
					return false
				}),
			},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pubPkt := packet.NewPublishPacket("test/topic", nil)
			ctx := &PublishContext{ClientID: "test-client", Packet: pubPkt}
			result := tt.hooks.callOnPublish(ctx)
			if result != tt.expectResult {
				t.Errorf("callOnPublish() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

func TestHooks_CallOnRequest(t *testing.T) {
	tests := []struct {
		name         string
		hooks        *Hooks
		expectResult bool
	}{
		{
			name:         "nil RequestHandler allows request",
			hooks:        &Hooks{},
			expectResult: true,
		},
		{
			name: "RequestHandler returns true allows request",
			hooks: &Hooks{
				RequestHandler: RequestHandlerFunc(func(ctx *RequestContext) bool {
					return true
				}),
			},
			expectResult: true,
		},
		{
			name: "RequestHandler returns false blocks request",
			hooks: &Hooks{
				RequestHandler: RequestHandlerFunc(func(ctx *RequestContext) bool {
					return false
				}),
			},
			expectResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqPkt := packet.NewRequestPacket(1, "test.action", nil)
			ctx := &RequestContext{ClientID: "test-client", Packet: reqPkt}
			result := tt.hooks.callOnRequest(ctx)
			if result != tt.expectResult {
				t.Errorf("callOnRequest() = %v, want %v", result, tt.expectResult)
			}
		})
	}
}

// ============================================================================
// Hooks 执行验证测试 - 确保 hooks 真的被调用
// ============================================================================

func TestHooksExecution_ContextPassedCorrectly(t *testing.T) {
	// 验证上下文数据正确传递
	var receivedClientID string
	var receivedSessionPresent bool

	hooks := &Hooks{
		ConnectionHandler: &ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				receivedClientID = ctx.ClientID
				receivedSessionPresent = ctx.SessionPresent
			},
		},
	}

	connack := packet.NewConnackPacket(packet.ReasonSuccess)
	connack.SessionPresent = true
	ctx := &ConnectContext{
		ClientID:       "my-client-123",
		SessionPresent: true,
		Packet:         connack,
	}
	hooks.callOnConnect(ctx)

	if receivedClientID != "my-client-123" {
		t.Errorf("ClientID not passed correctly: got %q, want %q", receivedClientID, "my-client-123")
	}
	if !receivedSessionPresent {
		t.Error("SessionPresent not passed correctly")
	}
}

func TestHooksExecution_MultipleHandlers(t *testing.T) {
	// 验证所有不同类型的 hooks 可以独立工作
	connectCalled := false
	disconnectCalled := false
	traceCalled := false
	publishCalled := false
	requestCalled := false

	hooks := &Hooks{
		ConnectionHandler: &ConnectionHandlerFunc{
			ConnectFunc: func(ctx *ConnectContext) {
				connectCalled = true
			},
			DisconnectFunc: func(ctx *DisconnectContext) {
				disconnectCalled = true
			},
		},
		TraceHandler: TraceHandlerFunc(func(ctx *TraceContext) {
			traceCalled = true
		}),
		MessageHandler: MessageHandlerFunc(func(ctx *PublishContext) bool {
			publishCalled = true
			return true
		}),
		RequestHandler: RequestHandlerFunc(func(ctx *RequestContext) bool {
			requestCalled = true
			return true
		}),
	}

	// 调用所有 hooks
	hooks.callOnConnect(&ConnectContext{})
	hooks.callOnDisconnect(&DisconnectContext{})
	tracePkt := packet.NewTracePacket("trace-1", "test", "test-client")
	hooks.callOnTrace(&TraceContext{Packet: tracePkt})
	pubPkt := packet.NewPublishPacket("test", nil)
	hooks.callOnPublish(&PublishContext{Packet: pubPkt})
	reqPkt := packet.NewRequestPacket(1, "test", nil)
	hooks.callOnRequest(&RequestContext{Packet: reqPkt})

	// 验证所有 hooks 都被调用
	if !connectCalled {
		t.Error("OnConnect was not called")
	}
	if !disconnectCalled {
		t.Error("OnDisconnect was not called")
	}
	if !traceCalled {
		t.Error("OnTrace was not called")
	}
	if !publishCalled {
		t.Error("OnPublish was not called")
	}
	if !requestCalled {
		t.Error("OnRequest was not called")
	}
}

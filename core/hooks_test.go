package core

import (
	"testing"

	"go.uber.org/zap"
)

// 钩子 panic 必须被隔离，不能带崩整个进程。
func TestHookPanicRecovered(t *testing.T) {
	push := &Hooks{
		Logger: zap.NewNop(),
		Message: MessageHandlerFunc{PushFn: func(*PushContext) error {
			panic("boom")
		}},
	}
	if err := push.callOnPush(&PushContext{}); err == nil {
		t.Fatal("expected error from panicking OnPush")
	}

	deliver := &Hooks{
		Logger: zap.NewNop(),
		Message: MessageHandlerFunc{DeliverFn: func(*DeliverContext) bool {
			panic("boom")
		}},
	}
	if deliver.callOnDeliver(&DeliverContext{}) {
		t.Fatal("panicking OnDeliver should reject the message")
	}

	auth := &Hooks{
		Logger: zap.NewNop(),
		Auth: AuthHandlerFunc(func(*AuthContext) error {
			panic("boom")
		}),
	}
	if err := auth.callAuth(&AuthContext{}); err == nil {
		t.Fatal("expected error from panicking auth hook")
	}
}

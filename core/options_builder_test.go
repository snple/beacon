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
		WithAddress(":1883").
		WithMaxClients(5000).
		WithMaxPacketSize(2097152).
		WithConnectTimeout(30).
		WithKeepAlive(120).
		WithMaxSessionExpiry(7200).
		WithReceiveWindow(500).
		WithDefaultMessageExpiry(48 * time.Hour).
		WithExpiredCheckInterval(300 * time.Second).
		WithRetransmitInterval(60 * time.Second).
		WithRetainEnabled(false).
		WithRequestQueueSize(200).
		WithLogger(logger)

	if opts.Address != ":1883" {
		t.Errorf("expected address :1883, got %s", opts.Address)
	}
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
	if opts.MaxSessionExpiry != 7200 {
		t.Errorf("expected maxSessionExpiry 7200, got %d", opts.MaxSessionExpiry)
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

// TestCoreOptionsDefaults 测试 CoreOptions 默认值
func TestCoreOptionsDefaults(t *testing.T) {
	opts := NewCoreOptions()

	if opts.Address != ":3883" {
		t.Errorf("expected default address :3883, got %s", opts.Address)
	}
	if opts.MaxClients != 10000 {
		t.Errorf("expected default maxClients 10000, got %d", opts.MaxClients)
	}
	if opts.MaxPacketSize != 1048576 {
		t.Errorf("expected default maxPacketSize 1048576, got %d", opts.MaxPacketSize)
	}
	if opts.ConnectTimeout != 10 {
		t.Errorf("expected default connectTimeout 10, got %d", opts.ConnectTimeout)
	}
	if opts.KeepAlive != 60 {
		t.Errorf("expected default keepAlive 60, got %d", opts.KeepAlive)
	}
	if opts.MaxSessionExpiry != 3600 {
		t.Errorf("expected default maxSessionExpiry 3600, got %d", opts.MaxSessionExpiry)
	}
	if opts.ReceiveWindow != 1000 {
		t.Errorf("expected default receiveWindow 1000, got %d", opts.ReceiveWindow)
	}
	if opts.DefaultMessageExpiry != 24*time.Hour {
		t.Errorf("expected default defaultMessageExpiry 24h, got %v", opts.DefaultMessageExpiry)
	}
	if opts.ExpiredCheckInterval != 180*time.Second {
		t.Errorf("expected default expiredCheckInterval 180s, got %v", opts.ExpiredCheckInterval)
	}
	if opts.RetransmitInterval != 30*time.Second {
		t.Errorf("expected default retransmitInterval 30s, got %v", opts.RetransmitInterval)
	}
	if !opts.RetainEnabled {
		t.Error("expected default retainEnabled true")
	}
	if opts.RequestQueueSize != 100 {
		t.Errorf("expected default requestQueueSize 100, got %d", opts.RequestQueueSize)
	}
}

// TestCoreOptionsValidate 测试 CoreOptions 验证
func TestCoreOptionsValidate(t *testing.T) {
	opts := NewCoreOptions().
		WithAddress(":8883").
		WithMaxClients(2000)

	if err := opts.Validate(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if opts.Address != ":8883" {
		t.Errorf("expected address :8883, got %s", opts.Address)
	}
	if opts.MaxClients != 2000 {
		t.Errorf("expected maxClients 2000, got %d", opts.MaxClients)
	}
}

// TestCoreOptionsWithStorage 测试存储配置
func TestCoreOptionsWithStorage(t *testing.T) {
	opts := NewCoreOptions().
		WithStorageDir("/tmp/queen-data").
		WithStorageEnabled(true)

	if !opts.StorageConfig.Enabled {
		t.Error("expected storage enabled")
	}
	if opts.StorageConfig.DataDir != "/tmp/queen-data" {
		t.Errorf("expected dataDir /tmp/queen-data, got %s", opts.StorageConfig.DataDir)
	}
}

// TestCoreOptionsWithHooks 测试钩子设置
func TestCoreOptionsWithHooks(t *testing.T) {
	authCalled := false
	authHandler := &AuthHandlerFunc{
		AuthFunc: func(ctx *AuthContext) (bool, []byte, error) {
			authCalled = true
			return true, nil, nil
		},
	}

	opts := NewCoreOptions().
		WithAuthHandler(authHandler)

	if opts.Hooks.AuthHandler == nil {
		t.Error("expected AuthHandler to be set")
	}

	// 调用测试
	_, _, _ = opts.Hooks.AuthHandler.OnAuth(&AuthContext{})
	if !authCalled {
		t.Error("expected AuthHandler to be called")
	}
}

// TestCoreOptionsWithMessageHandler 测试消息处理钩子
func TestCoreOptionsWithMessageHandler(t *testing.T) {
	msgCalled := false
	msgHandler := &MessageHandlerFunc{
		PublishFunc: func(ctx *PublishContext) error {
			msgCalled = true
			return nil
		},
	}

	opts := NewCoreOptions().
		WithMessageHandler(msgHandler)

	if opts.Hooks.MessageHandler == nil {
		t.Error("expected MessageHandler to be set")
	}

	// 调用测试
	_ = opts.Hooks.MessageHandler.OnPublish(&PublishContext{})
	if !msgCalled {
		t.Error("expected MessageHandler to be called")
	}
}

// TestCoreOptionsChaining 测试链式调用
func TestCoreOptionsChaining(t *testing.T) {
	opts := NewCoreOptions().
		WithAddress(":1883").
		WithMaxClients(100).
		WithKeepAlive(30).
		WithRetainEnabled(true).
		WithStorageEnabled(false)

	// 验证所有值都正确设置
	if opts.Address != ":1883" {
		t.Error("chain failed for Address")
	}
	if opts.MaxClients != 100 {
		t.Error("chain failed for MaxClients")
	}
	if opts.KeepAlive != 30 {
		t.Error("chain failed for KeepAlive")
	}
	if !opts.RetainEnabled {
		t.Error("chain failed for RetainEnabled")
	}
	if opts.StorageConfig.Enabled {
		t.Error("chain failed for StorageConfig.Enabled")
	}
}

// TestNewWithOptions 测试使用 Builder 模式创建 Core
func TestNewWithOptions(t *testing.T) {
	opts := NewCoreOptions().
		WithAddress(":13883").
		WithMaxClients(100)

	Core, err := NewWithOptions(opts)
	if err != nil {
		t.Fatalf("failed to create Core: %v", err)
	}

	if Core == nil {
		t.Fatal("Core should not be nil")
	}

	if Core.options.Address != ":13883" {
		t.Errorf("expected address :13883, got %s", Core.options.Address)
	}
	if Core.options.MaxClients != 100 {
		t.Errorf("expected maxClients 100, got %d", Core.options.MaxClients)
	}
}

// TestNewWithOptionsNil 测试传入 nil 选项
func TestNewWithOptionsNil(t *testing.T) {
	Core, err := NewWithOptions(nil)
	if err != nil {
		t.Fatalf("failed to create Core with nil options: %v", err)
	}

	if Core == nil {
		t.Fatal("Core should not be nil")
	}

	// 应使用默认配置
	if Core.options.Address != ":3883" {
		t.Errorf("expected default address :3883, got %s", Core.options.Address)
	}
}

// TestCoreOptionsWithTLS 测试 TLS 配置
func TestCoreOptionsWithTLS(t *testing.T) {
	// 不实际创建 TLS 配置，只测试设置功能
	opts := NewCoreOptions().
		WithTLS(nil)

	if opts.TLSConfig != nil {
		t.Error("expected TLSConfig to be nil")
	}
}

// TestCoreOptionsWithConnectionHandler 测试连接处理钩子
func TestCoreOptionsWithConnectionHandler(t *testing.T) {
	connectCalled := false
	connHandler := &ConnectionHandlerFunc{
		ConnectFunc: func(ctx *ConnectContext) {
			connectCalled = true
		},
	}

	opts := NewCoreOptions().
		WithConnectionHandler(connHandler)

	if opts.Hooks.ConnectionHandler == nil {
		t.Error("expected ConnectionHandler to be set")
	}

	// 调用测试
	opts.Hooks.ConnectionHandler.OnConnect(&ConnectContext{})
	if !connectCalled {
		t.Error("expected ConnectionHandler to be called")
	}
}

// TestCoreOptionsWithSubscriptionHandler 测试订阅处理钩子
func TestCoreOptionsWithSubscriptionHandler(t *testing.T) {
	subCalled := false
	subHandler := &SubscriptionHandlerFunc{
		SubscribeFunc: func(ctx *SubscribeContext) error {
			subCalled = true
			return nil
		},
	}

	opts := NewCoreOptions().
		WithSubscriptionHandler(subHandler)

	if opts.Hooks.SubscriptionHandler == nil {
		t.Error("expected SubscriptionHandler to be set")
	}

	// 调用测试
	_ = opts.Hooks.SubscriptionHandler.OnSubscribe(&SubscribeContext{})
	if !subCalled {
		t.Error("expected SubscriptionHandler to be called")
	}
}

// TestCoreOptionsWithTraceHandler 测试追踪处理钩子
func TestCoreOptionsWithTraceHandler(t *testing.T) {
	traceCalled := false
	traceHandler := TraceHandlerFunc(func(ctx *TraceContext) {
		traceCalled = true
	})

	opts := NewCoreOptions().
		WithTraceHandler(traceHandler)

	if opts.Hooks.TraceHandler == nil {
		t.Error("expected TraceHandler to be set")
	}

	// 调用测试
	opts.Hooks.TraceHandler.OnTrace(&TraceContext{})
	if !traceCalled {
		t.Error("expected TraceHandler to be called")
	}
}

// TestCoreOptionsWithStorageConfig 测试完整存储配置
func TestCoreOptionsWithStorageConfig(t *testing.T) {
	storageConfig := StorageConfig{
		Enabled:    true,
		DataDir:    "/custom/path",
		SyncWrites: true,
	}

	opts := NewCoreOptions().
		WithStorage(storageConfig)

	if !opts.StorageConfig.Enabled {
		t.Error("expected storage enabled")
	}
	if opts.StorageConfig.DataDir != "/custom/path" {
		t.Errorf("expected dataDir /custom/path, got %s", opts.StorageConfig.DataDir)
	}
	if !opts.StorageConfig.SyncWrites {
		t.Error("expected syncWrites true")
	}
}

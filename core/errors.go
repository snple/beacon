package core

import (
	"errors"
	"fmt"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
)

// 通用错误
var (
	// ErrCoreNotRunning core 未运行
	ErrCoreNotRunning = errors.New("core not running")

	// ErrCoreAlreadyRunning core 已在运行
	ErrCoreAlreadyRunning = errors.New("core already running")

	// ErrCoreStopped core 已停止（同一实例不可重启：context 已取消、存储已关闭）
	ErrCoreStopped = errors.New("core already stopped")
)

// 队列相关错误（复用共享 mq 包的定义，保持 API 兼容）
var (
	ErrQueueEmpty       = mq.ErrQueueEmpty
	ErrCorruptedMessage = mq.ErrCorruptedMessage
)

// 发送队列错误（复用共享 mq 包的定义，保持 API 兼容）
var (
	ErrSendQueueFull         = mq.ErrSendQueueFull
	ErrMessageAlreadyInQueue = mq.ErrMessageAlreadyInQueue
)

// 客户端相关错误
var (
	// ErrClientNotFound 客户端未找到
	ErrClientNotFound = errors.New("client not found")

	// ErrClientClosed 客户端已关闭
	ErrClientClosed = errors.New("client is closed")

	// ErrClientNotAvailable 客户端不可用
	ErrClientNotAvailable = errors.New("client is not available")

	// ErrSourceClientIDEmpty 来源客户端 ID 为空
	ErrSourceClientIDEmpty = errors.New("source client ID cannot be empty")
)

// 协议相关错误
var (
	// ErrInvalidConnectPacket 第一个包不是 CONNECT
	ErrInvalidConnectPacket = errors.New("first packet is not CONNECT")

	// ErrUnsupportedProtocol 不支持的协议版本
	ErrUnsupportedProtocol = errors.New("unsupported protocol version")
)

// 内部错误类型
var (
	ErrInvalidMessage       = errors.New("invalid message")
	ErrInvalidRetainMessage = errors.New("invalid retain message")
	ErrDeliveryRejected     = errors.New("delivery rejected by OnDeliver hook")
	ErrMessageTooLarge      = errors.New("message too large, dropped")
	ErrClientQueueFull      = errors.New("client queue quota exceeded")
	ErrInvalidTopic         = errors.New("invalid topic name")
	ErrQuotaExceeded        = errors.New("quota exceeded")
	ErrInvalidQoS           = errors.New("invalid qos")
)

// 管理面（事件流）错误
var (
	// ErrInvalidEventType 表示 EventOptions.Types 中包含未知的事件类型。
	ErrInvalidEventType = errors.New("invalid event type")
	// ErrNilContext 表示 SubscribeEvents 收到 nil context（无法自动关闭订阅）。
	ErrNilContext = errors.New("nil context")
)

// ClientNotFoundError 客户端未找到错误（携带 clientID）
type ClientNotFoundError struct {
	ClientID string
}

func (e *ClientNotFoundError) Error() string {
	return fmt.Sprintf("client not found: %s", e.ClientID)
}

func (e *ClientNotFoundError) Is(target error) bool {
	_, ok := target.(*ClientNotFoundError)
	return ok
}

// NewClientNotFoundError 创建客户端未找到错误
func NewClientNotFoundError(clientID string) error {
	return &ClientNotFoundError{ClientID: clientID}
}

// ClientClosedError 客户端已关闭错误（携带 clientID）
type ClientClosedError struct {
	ClientID string
}

func (e *ClientClosedError) Error() string {
	return fmt.Sprintf("client is closed: %s", e.ClientID)
}

func (e *ClientClosedError) Is(target error) bool {
	_, ok := target.(*ClientClosedError)
	return ok
}

// NewClientClosedError 创建客户端已关闭错误
func NewClientClosedError(clientID string) error {
	return &ClientClosedError{ClientID: clientID}
}

// ClientNotAvailableError 客户端不可用错误（携带 clientID）
type ClientNotAvailableError struct {
	ClientID string
}

func (e *ClientNotAvailableError) Error() string {
	return fmt.Sprintf("target client is not available: %s", e.ClientID)
}

func (e *ClientNotAvailableError) Is(target error) bool {
	_, ok := target.(*ClientNotAvailableError)
	return ok
}

// NewClientNotAvailableError 创建客户端不可用错误
func NewClientNotAvailableError(clientID string) error {
	return &ClientNotAvailableError{ClientID: clientID}
}

// AuthError 允许认证器指定返回给客户端的 CONNACK 原因码。
//
// 默认情况下任何认证失败都会返回 ReasonNotAuthorized，无法区分
// "凭证错误"与"被封禁""服务器忙"等语义。认证器可以返回 *AuthError
// 来指定原因码，例如 packet.ReasonBanned。
type AuthError struct {
	Reason packet.ReasonCode // 返回给客户端的 CONNACK 原因码
	Err    error             // 可选，仅用于服务端日志，不下发
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return "authentication failed: " + e.Err.Error()
	}
	return "authentication failed: " + e.Reason.String()
}

func (e *AuthError) Unwrap() error { return e.Err }

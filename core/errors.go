package core

import (
	"errors"
	"fmt"
	"time"
)

// 通用错误
var (
	// ErrCoreNotRunning core 未运行
	ErrCoreNotRunning = errors.New("core not running")

	// ErrCoreAlreadyRunning core 已在运行
	ErrCoreAlreadyRunning = errors.New("core already running")
)

// 轮询相关错误
var (
	// ErrPollTimeout 轮询超时
	ErrPollTimeout = errors.New("poll timeout")
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

// 请求响应相关错误
var (
	// ErrRequestCoreNotSet 请求的 core 引用未设置
	ErrRequestCoreNotSet = errors.New("core request core not set")

	// ErrResultNil 响应结果为空
	ErrResultNil = errors.New("result cannot be nil")

	// ErrResponseHandlerClosed 响应处理器已关闭
	ErrResponseHandlerClosed = errors.New("response handler closed")

	// ErrActionNotFound action 未找到
	ErrActionNotFound = errors.New("action not found")

	// ErrNoAvailableHandler 没有可用的处理器
	ErrNoAvailableHandler = errors.New("no available handler")
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
)

// ClientNotFoundError 客户端未找到错误（携带 clientID）
type ClientNotFoundError struct {
	ClientID string
}

func (e *ClientNotFoundError) Error() string {
	return fmt.Sprintf("client not found: %s", e.ClientID)
}

func (e *ClientNotFoundError) Is(target error) bool {
	return target == ErrClientNotFound
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
	return target == ErrClientClosed
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
	return target == ErrClientNotAvailable
}

// NewClientNotAvailableError 创建客户端不可用错误
func NewClientNotAvailableError(clientID string) error {
	return &ClientNotAvailableError{ClientID: clientID}
}

// ActionNotFoundError action 未找到错误（携带 action 名称）
type ActionNotFoundError struct {
	Action string
}

func (e *ActionNotFoundError) Error() string {
	return fmt.Sprintf("action not found: %s", e.Action)
}

func (e *ActionNotFoundError) Is(target error) bool {
	return target == ErrActionNotFound
}

// NewActionNotFoundError 创建 action 未找到错误
func NewActionNotFoundError(action string) error {
	return &ActionNotFoundError{Action: action}
}

// NoAvailableHandlerError 没有可用处理器错误（携带 action 名称）
type NoAvailableHandlerError struct {
	Action string
}

func (e *NoAvailableHandlerError) Error() string {
	return fmt.Sprintf("no available handler for action: %s", e.Action)
}

func (e *NoAvailableHandlerError) Is(target error) bool {
	return target == ErrNoAvailableHandler
}

// NewNoAvailableHandlerError 创建没有可用处理器错误
func NewNoAvailableHandlerError(action string) error {
	return &NoAvailableHandlerError{Action: action}
}

// PollTimeoutError 轮询超时错误（携带超时时间）
type PollTimeoutError struct {
	Timeout time.Duration
}

func (e *PollTimeoutError) Error() string {
	return fmt.Sprintf("poll timeout after %v", e.Timeout)
}

func (e *PollTimeoutError) Is(target error) bool {
	return target == ErrPollTimeout
}

// NewPollTimeoutError 创建轮询超时错误
func NewPollTimeoutError(timeout time.Duration) error {
	return &PollTimeoutError{Timeout: timeout}
}

// RequestTimeoutError 请求超时错误
type RequestTimeoutError struct {
	Timeout time.Duration // time.Duration 或 string
}

func (e *RequestTimeoutError) Error() string {
	return fmt.Sprintf("request timeout after %v", e.Timeout)
}

// NewRequestTimeoutError 创建请求超时错误
func NewRequestTimeoutError(timeout time.Duration) error {
	return &RequestTimeoutError{Timeout: timeout}
}

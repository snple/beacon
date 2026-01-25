package client

import (
	"errors"
	"fmt"
	"time"
)

// 连接相关错误
var (
	// ErrAlreadyConnected 已经连接
	ErrAlreadyConnected = errors.New("already connected")

	// ErrNotConnected 未连接
	ErrNotConnected = errors.New("not connected")

	// ErrClientClosed 客户端已关闭
	ErrClientClosed = errors.New("client closed")
)

// 请求响应相关错误
var (
	// ErrRequestClientNotSet 请求的 client 引用未设置
	ErrRequestClientNotSet = errors.New("request client not set")

	// ErrRequestNil 请求为空
	ErrRequestNil = errors.New("request cannot be nil")

	// ErrResultNil 响应结果为空
	ErrResultNil = errors.New("result cannot be nil")

	ErrActionEmpty = errors.New("action cannot be empty")
)

// 订阅/注册相关错误
var (
	// ErrTopicsEmpty 主题列表为空
	ErrTopicsEmpty = errors.New("topics cannot be empty")

	// ErrActionsEmpty actions 列表为空
	ErrActionsEmpty = errors.New("actions cannot be empty")

	// ErrWillTopicRequired 遗嘱主题是必需的
	ErrWillTopicRequired = errors.New("will topic is required")
)

// 超时相关错误
var (
	// ErrPollTimeout 轮询超时
	ErrPollTimeout = errors.New("poll timeout")

	// ErrPublishTimeout 发布超时
	ErrPublishTimeout = errors.New("publish timeout")

	// ErrSubscribeTimeout 订阅超时
	ErrSubscribeTimeout = errors.New("subscribe timeout")

	// ErrUnsubscribeTimeout 取消订阅超时
	ErrUnsubscribeTimeout = errors.New("unsubscribe timeout")
)

// 队列相关错误
var (
	// 队列相关错误
	ErrQueueEmpty = errors.New("queue is empty")
)

// 内部错误类型
var (
	ErrInvalidMessage       = errors.New("invalid message")
	ErrInvalidRetainMessage = errors.New("invalid retain message")
	ErrPacketTooLarge       = errors.New("packet size exceeds client maxPacketSize")
)

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
	Timeout time.Duration
}

func (e *RequestTimeoutError) Error() string {
	return fmt.Sprintf("request timeout after %v", e.Timeout)
}

func (e *RequestTimeoutError) Is(target error) bool {
	_, ok := target.(*RequestTimeoutError)
	return ok
}

// NewRequestTimeoutError 创建请求超时错误
func NewRequestTimeoutError(timeout time.Duration) error {
	return &RequestTimeoutError{Timeout: timeout}
}

// ConnectionError 连接错误（带详细原因）
type ConnectionError struct {
	Op  string // 操作：connect, send CONNECT, read CONNACK 等
	Err error  // 底层错误
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to %s: %v", e.Op, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// NewConnectionError 创建连接错误
func NewConnectionError(op string, err error) error {
	return &ConnectionError{Op: op, Err: err}
}

// ConnectionRefusedError 连接被拒绝错误
type ConnectionRefusedError struct {
	Reason string
}

func (e *ConnectionRefusedError) Error() string {
	return fmt.Sprintf("connection refused: %s", e.Reason)
}

// NewConnectionRefusedError 创建连接被拒绝错误
func NewConnectionRefusedError(reason string) error {
	return &ConnectionRefusedError{Reason: reason}
}

// UnexpectedPacketError 意外的包类型错误
type UnexpectedPacketError struct {
	Expected string
	Got      string
}

func (e *UnexpectedPacketError) Error() string {
	return fmt.Sprintf("expected %s, got %s", e.Expected, e.Got)
}

// NewUnexpectedPacketError 创建意外包类型错误
func NewUnexpectedPacketError(expected, got string) error {
	return &UnexpectedPacketError{Expected: expected, Got: got}
}

// SubscriptionError 订阅错误
type SubscriptionError struct {
	Index  int
	Reason string
}

func (e *SubscriptionError) Error() string {
	return fmt.Sprintf("subscription %d failed: %s", e.Index, e.Reason)
}

// NewSubscriptionError 创建订阅错误
func NewSubscriptionError(index int, reason string) error {
	return &SubscriptionError{Index: index, Reason: reason}
}

// UnsubscriptionError 取消订阅错误
type UnsubscriptionError struct {
	Index  int
	Reason string
}

func (e *UnsubscriptionError) Error() string {
	return fmt.Sprintf("unsubscription %d failed: %s", e.Index, e.Reason)
}

// NewUnsubscriptionError 创建取消订阅错误
func NewUnsubscriptionError(index int, reason string) error {
	return &UnsubscriptionError{Index: index, Reason: reason}
}

// PublishWarningError 发布警告错误（QoS1 ACK 返回非 Success）
type PublishWarningError struct {
	Reason string
}

func (e *PublishWarningError) Error() string {
	return fmt.Sprintf("publish warning: %s", e.Reason)
}

// NewPublishWarningError 创建发布警告错误
func NewPublishWarningError(reason string) error {
	return &PublishWarningError{Reason: reason}
}

// ServerDisconnectError 服务器断开连接错误
type ServerDisconnectError struct {
	Reason string
}

func (e *ServerDisconnectError) Error() string {
	return fmt.Sprintf("server disconnect: %s", e.Reason)
}

// NewServerDisconnectError 创建服务器断开连接错误
func NewServerDisconnectError(reason string) error {
	return &ServerDisconnectError{Reason: reason}
}

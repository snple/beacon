package client

import (
	"errors"
	"fmt"
	"time"

	"github.com/snple/beacon/internal/mq"
	"github.com/snple/beacon/packet"
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

// 订阅/注册相关错误
var (
	// ErrTopicsEmpty 主题列表为空
	ErrTopicsEmpty = errors.New("topics cannot be empty")

	// ErrWillTopicRequired 遗嘱主题是必需的
	ErrWillTopicRequired = errors.New("will topic is required")

	// ErrWillTopicInvalid 遗嘱主题非法（不允许通配符或空字符）
	ErrWillTopicInvalid = errors.New("will topic is invalid")

	// ErrWillQoSInvalid 遗嘱 QoS 非法（仅支持 QoS0/QoS1）
	ErrWillQoSInvalid = errors.New("will QoS is invalid")
)

// 超时相关错误
var (
	// ErrPollTimeout 轮询超时
	ErrPollTimeout = errors.New("poll timeout")

	// ErrPublishTimeout 发布超时
	ErrPublishTimeout = errors.New("publish timeout")

	// ErrNoPendingAck 消息没有待确认的手动 ACK（AutoAck=true 或 QoS0 消息）
	ErrNoPendingAck = errors.New("message has no pending manual ack")

	// ErrSubscribeTimeout 订阅超时
	ErrSubscribeTimeout = errors.New("subscribe timeout")

	// ErrUnsubscribeTimeout 取消订阅超时
	ErrUnsubscribeTimeout = errors.New("unsubscribe timeout")
)

// 队列相关错误
var (
	// 队列相关错误（复用共享 mq 包的定义，保持 API 兼容）
	ErrQueueEmpty       = mq.ErrQueueEmpty
	ErrCorruptedMessage = mq.ErrCorruptedMessage

	// ErrMessageTooLarge 消息超过允许的最大大小，应丢弃
	ErrMessageTooLarge = errors.New("message too large, dropped")

	// ErrQueueFull 本地出站队列已达配额上限，消息未被接受
	ErrQueueFull = errors.New("outbound queue is full")
)

// 发送队列错误（复用共享 mq 包的定义，保持 API 兼容）
var (
	ErrSendQueueFull         = mq.ErrSendQueueFull
	ErrMessageAlreadyInQueue = mq.ErrMessageAlreadyInQueue
)

// 内部错误类型
var (
	ErrInvalidMessage       = errors.New("invalid message")
	ErrInvalidRetainMessage = errors.New("invalid retain message")
	ErrPacketTooLarge       = errors.New("packet size exceeds client maxPacketSize")
	ErrClientIDInvalid      = errors.New("client ID is invalid")
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

	// Codes 是 SUBACK 返回的逐条原因码，与 SUBSCRIBE 请求里的订阅顺序一一对应。
	// 调用方可以据此判断哪些订阅被授予、哪些被拒绝（Codes 为空表示服务端未返回详情）。
	Codes []packet.ReasonCode
}

func (e *SubscriptionError) Error() string {
	return fmt.Sprintf("subscription %d failed: %s", e.Index, e.Reason)
}

// Granted 报告第 i 个订阅是否被服务端授予。超出 Codes 范围时按授予处理
// （服务端未报告即视为没有拒绝）。
func (e *SubscriptionError) Granted(i int) bool {
	return i >= len(e.Codes) || e.Codes[i].IsSubackSuccess()
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

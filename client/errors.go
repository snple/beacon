package client

import (
	"errors"
	"fmt"

	"github.com/snple/beacon/packet"
)

var (
	// ErrNotConnected 客户端未连接
	ErrNotConnected = errors.New("client not connected")
	// ErrClientClosed 客户端已关闭
	ErrClientClosed = errors.New("client is closed")
	// ErrAlreadyConnected 客户端已有活动连接
	ErrAlreadyConnected = errors.New("client already connected")
	// ErrTimeout 请求超时
	ErrTimeout = errors.New("request timeout")
	// ErrPollTimeout 轮询超时
	ErrPollTimeout = errors.New("poll timeout")
	// ErrSessionTakenOver 会话被同 ClientID 的其他连接接管，不应自动重连
	ErrSessionTakenOver = errors.New("session taken over by another connection")
	// ErrKicked 被服务端踢下线
	ErrKicked = errors.New("kicked by server")
	// ErrCommitRequired 服务端要求先确认再拉取
	ErrCommitRequired = errors.New("commit required before pulling more")
	// ErrRateLimited 服务端限流（例如已有在途 PULL）
	ErrRateLimited = errors.New("request rate limited")
	// ErrTargetNotSubscribed 定向投递的目标存在，但没有声明匹配该 channel 的过滤器
	ErrTargetNotSubscribed = errors.New("target has not subscribed to the channel")
	// ErrKeepAliveTimeout 超过存活窗口没有收到任何包（半开连接或服务端无响应）
	ErrKeepAliveTimeout = errors.New("keepalive timeout")
	// ErrExpired 消息已过期，服务端拒绝或丢弃
	ErrExpired = errors.New("message expired")
)

// ConnectionError 表示连接层错误。
type ConnectionError struct {
	Op  string
	Err error
}

func (e *ConnectionError) Error() string { return fmt.Sprintf("%s: %v", e.Op, e.Err) }
func (e *ConnectionError) Unwrap() error { return e.Err }

// NewConnectionError 构造连接错误。
func NewConnectionError(op string, err error) *ConnectionError {
	return &ConnectionError{Op: op, Err: err}
}

// ServerError 表示服务端返回了失败原因码。
type ServerError struct {
	Reason packet.ReasonCode
	Detail string
}

func (e *ServerError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("server rejected: %s: %s", e.Reason.String(), e.Detail)
	}
	return fmt.Sprintf("server rejected: %s", e.Reason.String())
}

// NewServerError 构造服务端错误。
func NewServerError(reason packet.ReasonCode) *ServerError {
	return &ServerError{Reason: reason}
}

// NewServerErrorWithDetail 构造带错误细节的服务端错误。
func NewServerErrorWithDetail(reason packet.ReasonCode, detail string) *ServerError {
	return &ServerError{Reason: reason, Detail: detail}
}

// reasonToError 把原因码映射为错误（成功返回 nil）。
func reasonToError(reason packet.ReasonCode) error {
	return reasonToErrorWithDetail(reason, "")
}

// reasonToErrorWithDetail 把原因码和可选细节映射为错误。
func reasonToErrorWithDetail(reason packet.ReasonCode, detail string) error {
	if !reason.IsError() {
		return nil
	}
	switch reason {
	case packet.ReasonSessionTakenOver:
		return ErrSessionTakenOver
	case packet.ReasonKicked:
		return ErrKicked
	case packet.ReasonCommitRequired:
		return ErrCommitRequired
	case packet.ReasonRateLimited:
		return ErrRateLimited
	case packet.ReasonExpired:
		return ErrExpired
	default:
		return NewServerErrorWithDetail(reason, detail)
	}
}

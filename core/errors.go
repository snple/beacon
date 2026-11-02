package core

import (
	"errors"
	"fmt"

	"github.com/snple/beacon/packet"
)

var (
	// ErrCoreNotRunning core 未运行
	ErrCoreNotRunning = errors.New("core not running")
	// ErrCoreAlreadyRunning core 已在运行
	ErrCoreAlreadyRunning = errors.New("core already running")
	// ErrCoreStopped core 已停止（同一实例不可重启）
	ErrCoreStopped = errors.New("core already stopped")

	// ErrClientNotFound 客户端未找到
	ErrClientNotFound = errors.New("client not found")
	// ErrClientNotConnected 客户端当前没有活动连接
	ErrClientNotConnected = errors.New("client is not connected")

	// ErrInvalidConnectPacket 第一个包不是 CONNECT
	ErrInvalidConnectPacket = errors.New("first packet is not CONNECT")
	// ErrUnsupportedProtocol 不支持的协议版本
	ErrUnsupportedProtocol = errors.New("unsupported protocol version")

	// ErrSessionTakenOver 会话已被新连接接管
	ErrSessionTakenOver = errors.New("session taken over")

	// ErrTargetNotFound 定向投递的目标没有会话
	ErrTargetNotFound = errors.New("target client not found")

	// ErrNotAuthorized 未通过 ACL
	ErrNotAuthorized = errors.New("not authorized")

	// ErrInvalidChannel channel 名不合法
	ErrInvalidChannel = errors.New("invalid channel")

	// ErrRecordTooLarge 单条记录超过上限
	ErrRecordTooLarge = errors.New("record too large")

	// ErrBatchTooLarge 单次 PUSH 超过上限
	ErrBatchTooLarge = errors.New("batch too large")
)

// ClientNotFoundError 客户端不存在
type ClientNotFoundError struct{ ClientID string }

func (e *ClientNotFoundError) Error() string {
	return fmt.Sprintf("client %q not found", e.ClientID)
}

func (e *ClientNotFoundError) Is(target error) bool { return target == ErrClientNotFound }

// AuthError 认证失败，可指定 CONNACK 原因码
type AuthError struct {
	Reason packet.ReasonCode
	Err    error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason.String()
}

func (e *AuthError) Unwrap() error { return e.Err }

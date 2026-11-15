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

	// ErrMaxClients 在线连接数已达上限
	ErrMaxClients = errors.New("max clients reached")
	// ErrMaxSessions 会话总数已达上限
	ErrMaxSessions = errors.New("max sessions reached")

	// ErrInvalidConnectPacket 第一个包不是 CONNECT
	ErrInvalidConnectPacket = errors.New("first packet is not CONNECT")
	// ErrUnsupportedProtocol 不支持的协议版本
	ErrUnsupportedProtocol = errors.New("unsupported protocol version")

	// ErrSessionTakenOver 会话已被新连接接管
	ErrSessionTakenOver = errors.New("session taken over")

	// ErrTargetNotFound 定向投递的目标没有会话
	ErrTargetNotFound = errors.New("target client not found")
	// ErrTargetNotSubscribed 定向投递的目标没有声明匹配该 channel 的过滤器
	ErrTargetNotSubscribed = errors.New("target has not subscribed to the channel")

	// ErrNotAuthorized 未通过 ACL
	ErrNotAuthorized = errors.New("not authorized")

	// ErrInvalidChannel channel 名不合法
	ErrInvalidChannel = errors.New("invalid channel")

	// ErrRecordTooLarge 单条记录超过上限
	ErrRecordTooLarge = errors.New("record too large")
)

// ClientNotFoundError 客户端不存在
type ClientNotFoundError struct{ ClientID string }

func (e *ClientNotFoundError) Error() string {
	return fmt.Sprintf("client %q not found", e.ClientID)
}

func (e *ClientNotFoundError) Is(target error) bool { return target == ErrClientNotFound }

// HookError 让钩子指定拒绝原因码，并可选择把 Detail 回给客户端。
//
// Detail 为空时不会向客户端泄露钩子内部错误；Err 只用于本地日志与
// errors.Is/As 判断。Reason 不是失败码时按 NotAuthorized 处理。
type HookError struct {
	// Reason 是回给客户端的失败原因码；不是失败码时按 NotAuthorized 处理。
	Reason packet.ReasonCode
	// Detail 是回给客户端的诊断信息；应避免包含内部实现细节。
	Detail string
	// Err 是仅用于本地日志与 errors.Is/As 判断的原始错误。
	Err error
}

func (e *HookError) Error() string {
	switch {
	case e.Detail != "":
		return e.Detail
	case e.Err != nil:
		return e.Err.Error()
	case e.Reason != 0:
		return e.Reason.String()
	default:
		return "hook rejected"
	}
}

func (e *HookError) Unwrap() error { return e.Err }

// AuthError 认证失败，可指定 CONNACK 原因码。等价于 HookError。
type AuthError = HookError

// hookRejection 解析钩子返回的拒绝错误。
//
// 只有显式设置 HookError.Detail 才会回传诊断信息，普通 error 一律按
// NotAuthorized 且不带细节处理，避免泄露服务端内部错误。
func hookRejection(err error) (packet.ReasonCode, string) {
	var he *HookError
	if errors.As(err, &he) {
		reason := he.Reason
		if !reason.IsError() {
			// 钩子返回了成功码：拒绝必须带失败码，否则发布者会误以为成功。
			reason = packet.ReasonNotAuthorized
		}
		return reason, he.Detail
	}
	return packet.ReasonNotAuthorized, ""
}

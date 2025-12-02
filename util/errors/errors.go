// Package errors 提供 Beacon 的标准错误定义
package errors

import "errors"

// 标准错误定义
var (
	// 数据库和存储相关错误
	ErrInvalidDatabase = errors.New("database is nil or invalid")
	ErrStorageNotFound = errors.New("storage not found")
	ErrStorageClosed   = errors.New("storage has been closed")

	// Node 相关错误
	ErrNodeNotFound      = errors.New("node not found")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrNodeInvalidID     = errors.New("node ID is invalid")
	ErrNodeInvalidName   = errors.New("node name is invalid")
	ErrNodeOffline       = errors.New("node is offline")

	// Wire 相关错误
	ErrWireNotFound      = errors.New("wire not found")
	ErrWireAlreadyExists = errors.New("wire already exists")
	ErrWireInvalidID     = errors.New("wire ID is invalid")
	ErrWireInvalidName   = errors.New("wire name is invalid")

	// Pin 相关错误
	ErrPinNotFound      = errors.New("pin not found")
	ErrPinAlreadyExists = errors.New("pin already exists")
	ErrPinInvalidID     = errors.New("pin ID is invalid")
	ErrPinInvalidName   = errors.New("pin name is invalid")
	ErrPinReadOnly      = errors.New("pin is read-only")
	ErrPinWriteOnly     = errors.New("pin is write-only")
	ErrPinInvalidType   = errors.New("pin type is invalid")

	// 参数错误
	ErrInvalidArgument = errors.New("invalid argument")
	ErrEmptyID         = errors.New("ID cannot be empty")
	ErrEmptyName       = errors.New("name cannot be empty")
	ErrInvalidValue    = errors.New("invalid value")

	// 配置错误
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrConfigNotFound    = errors.New("configuration not found")
	ErrConfigParseFailed = errors.New("failed to parse configuration")

	// Secret 错误
	ErrSecretNotFound = errors.New("secret not found")
	ErrSecretInvalid  = errors.New("secret is invalid")
	ErrSecretMismatch = errors.New("secret does not match")
	ErrSecretTooShort = errors.New("secret is too short")
	ErrSecretTooLong  = errors.New("secret is too long")
	ErrSecretRequired = errors.New("secret is required")
	ErrSecretExpired  = errors.New("secret has expired")
	ErrSecretNotSet   = errors.New("secret is not set")
	ErrUnauthorized   = errors.New("unauthorized")

	// 同步错误
	ErrSyncFailed     = errors.New("synchronization failed")
	ErrSyncInProgress = errors.New("synchronization in progress")
	ErrSyncTimeout    = errors.New("synchronization timeout")

	// 连接错误
	ErrConnectionFailed = errors.New("connection failed")
	ErrConnectionClosed = errors.New("connection closed")
	ErrConnectionLost   = errors.New("connection lost")

	// 编解码错误
	ErrEncodeFailed = errors.New("encode failed")
	ErrDecodeFailed = errors.New("decode failed")
	ErrInvalidData  = errors.New("invalid data")

	// 操作错误
	ErrOperationFailed   = errors.New("operation failed")
	ErrOperationTimeout  = errors.New("operation timeout")
	ErrOperationCanceled = errors.New("operation canceled")
	ErrNotImplemented    = errors.New("not implemented")
	ErrNotSupported      = errors.New("not supported")
)

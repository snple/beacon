// Package logger 提供统一的日志记录功能
package logger

import (
	"go.uber.org/zap"
)

// Logger 全局日志实例
var Logger *zap.Logger

// InitLogger 初始化全局日志实例
//
// 参数：
//   - development: 是否为开发模式
//
// 返回：
//   - error: 初始化失败时返回错误
func InitLogger(development bool) error {
	var err error
	if development {
		Logger, err = zap.NewDevelopment()
	} else {
		Logger, err = zap.NewProduction()
	}
	return err
}

// Debug 记录 Debug 级别日志
func Debug(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Debug(msg, fields...)
	}
}

// Info 记录 Info 级别日志
func Info(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Info(msg, fields...)
	}
}

// Warn 记录 Warn 级别日志
func Warn(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Warn(msg, fields...)
	}
}

// Error 记录 Error 级别日志
func Error(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Error(msg, fields...)
	}
}

// Fatal 记录 Fatal 级别日志并退出程序
func Fatal(msg string, fields ...zap.Field) {
	if Logger != nil {
		Logger.Fatal(msg, fields...)
	}
}

// Sync 刷新日志缓冲区
func Sync() error {
	if Logger != nil {
		return Logger.Sync()
	}
	return nil
}

// WithFields 创建带有公共字段的日志记录器
//
// 示例：
//
//	logger := WithFields(
//	  zap.String("node_id", nodeID),
//	  zap.String("wire_id", wireID),
//	)
//	logger.Info("processing wire")
func WithFields(fields ...zap.Field) *zap.Logger {
	if Logger != nil {
		return Logger.With(fields...)
	}
	return zap.NewNop()
}

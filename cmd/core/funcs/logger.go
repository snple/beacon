package funcs

import (
	"go.uber.org/zap"
)

// zapGrpcLogger 实现了 grpclog.LoggerV2 接口
type zapGrpcLogger struct {
	logger *zap.Logger
}

func newZapGrpcLogger(logger *zap.Logger) *zapGrpcLogger {
	return &zapGrpcLogger{
		logger: logger,
	}
}

func (l *zapGrpcLogger) Info(args ...interface{}) {
	l.logger.Sugar().Info(args...)
}

func (l *zapGrpcLogger) Infoln(args ...interface{}) {
	l.logger.Sugar().Info(args...)
}

func (l *zapGrpcLogger) Infof(format string, args ...interface{}) {
	l.logger.Sugar().Infof(format, args...)
}

func (l *zapGrpcLogger) Warning(args ...interface{}) {
	l.logger.Sugar().Warn(args...)
}

func (l *zapGrpcLogger) Warningln(args ...interface{}) {
	l.logger.Sugar().Warn(args...)
}

func (l *zapGrpcLogger) Warningf(format string, args ...interface{}) {
	l.logger.Sugar().Warnf(format, args...)
}

func (l *zapGrpcLogger) Error(args ...interface{}) {
	l.logger.Sugar().Error(args...)
}

func (l *zapGrpcLogger) Errorln(args ...interface{}) {
	l.logger.Sugar().Error(args...)
}

func (l *zapGrpcLogger) Errorf(format string, args ...interface{}) {
	l.logger.Sugar().Errorf(format, args...)
}

func (l *zapGrpcLogger) Fatal(args ...interface{}) {
	l.logger.Sugar().Fatal(args...)
}

func (l *zapGrpcLogger) Fatalln(args ...interface{}) {
	l.logger.Sugar().Fatal(args...)
}

func (l *zapGrpcLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Sugar().Fatalf(format, args...)
}

func (l *zapGrpcLogger) V(level int) bool {
	return true
}

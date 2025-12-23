package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/core/log"
	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

// 测试客户端配置
const (
	TEST_CLIENT_ID     = "test-client"
	TEST_CLIENT_SECRET = "client-secret-123"
)

// ESP32 双路 LED 测试配置
const (
	LED_NODE_ID     = "esp32-led-001"
	LED_NODE_SECRET = "led-secret-123"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "version", "-V":
			fmt.Printf("beacon core version: %v\n", beacon.Version)
			return
		}
	}

	log.Init(true, "logs/core.log")

	log.Logger.Info("main: Started")
	defer log.Logger.Info("main: Completed")

	// 创建 Core
	coreService, err := core.Core(
		core.WithLogger(log.Logger.Named("core")),
		core.WithBadger(badger.DefaultOptions("data").WithLogger(NewBadgerLogger(log.Logger))),
		core.WithQueenBroker(":5208", nil),
		core.WithBatchNotifyInterval(100*time.Millisecond),
	)
	if err != nil {
		log.Logger.Sugar().Fatalf("Failed to create core: %v", err)
	}
	coreService.Start()
	defer coreService.Stop()

	{
		if err := coreService.GetNode().SetSecret(TEST_CLIENT_ID, TEST_CLIENT_SECRET); err != nil {
			coreService.Stop()
			log.Logger.Sugar().Fatalf("Failed to set client secret: %v", err)
		}

		if err := coreService.GetNode().SetSecret(LED_NODE_ID, LED_NODE_SECRET); err != nil {
			coreService.Stop()
			log.Logger.Sugar().Fatalf("Failed to set node secret: %v", err)
		}
	}

	{
		signalCh := make(chan os.Signal, 1)
		signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

		log.Logger.Info("main: Waiting for shutdown signal")

		sig := <-signalCh
		log.Logger.Sugar().Infof("main: Received signal %v, shutting down", sig)
	}
}

type BadgerLogger struct {
	*zap.SugaredLogger
}

func NewBadgerLogger(l *zap.Logger) *BadgerLogger {
	return &BadgerLogger{
		l.Named("badger").Sugar(),
	}
}

func (l *BadgerLogger) Errorf(format string, args ...any) {
	l.SugaredLogger.Errorf(format, args...)
}

func (l *BadgerLogger) Warningf(format string, args ...any) {
	l.SugaredLogger.Warnf(format, args...)
}

func (l *BadgerLogger) Infof(format string, args ...any) {
	l.SugaredLogger.Infof(format, args...)
}

func (l *BadgerLogger) Debugf(format string, args ...any) {
	l.SugaredLogger.Debugf(format, args...)
}

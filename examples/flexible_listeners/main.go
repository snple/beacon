// flexible_listeners 演示用自定义 net.Listener 接入 Core。
package main

import (
	"net"
	"os"

	"github.com/snple/beacon/core"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer func() { _ = logger.Sync() }()

	b, err := core.NewWithOptions(core.NewCoreOptions().WithLogger(logger))
	if err != nil {
		logger.Fatal("create core", zap.Error(err))
	}
	if err := b.Start(); err != nil {
		logger.Fatal("start core", zap.Error(err))
	}
	defer func() { _ = b.Stop() }()

	// Unix socket
	sockPath := "/tmp/beacon.sock"
	_ = os.Remove(sockPath)
	unixListener, err := net.Listen("unix", sockPath)
	if err != nil {
		logger.Fatal("unix listener", zap.Error(err))
	}
	go func() { _ = b.Serve(unixListener) }()

	// TCP
	tcpListener, err := net.Listen("tcp", "127.0.0.1:15208")
	if err != nil {
		logger.Fatal("tcp listener", zap.Error(err))
	}
	logger.Info("serving", zap.String("unix", sockPath), zap.String("tcp", "127.0.0.1:15208"))
	_ = b.Serve(tcpListener)
}

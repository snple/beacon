package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/core/config"
	"github.com/snple/beacon/bin/core/log"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/util"
	_ "github.com/snple/beacon/util/compress/zstd"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "version", "-V":
			fmt.Printf("beacon core version: %v\n", beacon.Version)
			return
		}
	}

	config.Parse()

	log.Init(config.Config.Debug)

	log.Logger.Info("main: Started")
	defer log.Logger.Info("main: Completed")

	badgerOpts := badger.DefaultOptions(config.Config.DB.File)
	badgerOpts.Logger = nil

	command := flag.Arg(0)
	switch command {
	case "seed":
		log.Logger.Sugar().Infof("seed: Completed")
		return
	}

	coreOpts := []core.CoreOption{
		core.WithBadger(badgerOpts),
	}

	// 配置 Queen broker（如果启用）
	if config.Config.CoreService.Enable {
		var tlsConfig *tls.Config
		if config.Config.CoreService.TLS {
			var err error
			tlsConfig, err = util.LoadServerCert(config.Config.CoreService.CA, config.Config.CoreService.Cert, config.Config.CoreService.Key)
			if err != nil {
				log.Logger.Sugar().Fatal(err)
			}
		}

		coreOpts = append(coreOpts, core.WithQueenBroker(config.Config.CoreService.Addr, tlsConfig))
	}

	cs, err := core.Core(coreOpts...)
	if err != nil {
		log.Logger.Sugar().Fatalf("NewCoreService: %v", err)
	}

	cs.Start()
	defer cs.Stop()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

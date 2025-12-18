package main

import (
	"flag"
	"fmt"
	"net"
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
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
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

	cs, err := core.Core(coreOpts...)
	if err != nil {
		log.Logger.Sugar().Fatalf("NewCoreService: %v", err)
	}

	cs.Start()
	defer cs.Stop()

	if config.Config.CoreService.Enable {
		grpcOpts := make([]grpc.ServerOption, 0)

		if config.Config.CoreService.TLS {
			tlsConfig, err := util.LoadServerCert(config.Config.CoreService.CA, config.Config.CoreService.Cert, config.Config.CoreService.Key)
			if err != nil {
				log.Logger.Sugar().Fatal(err)
			}

			grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		} else {
			grpcOpts = append(grpcOpts, grpc.Creds(insecure.NewCredentials()))
		}

		grpcOpts = append(grpcOpts, grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{PermitWithoutStream: true}))

		s := grpc.NewServer(grpcOpts...)
		defer s.Stop()

		cs.Register(s)

		lis, err := net.Listen("tcp", config.Config.CoreService.Addr)
		if err != nil {
			log.Logger.Sugar().Fatalf("failed to listen: %v", err)
		}

		go func() {
			log.Logger.Sugar().Infof("core grpc start: %v", config.Config.CoreService.Addr)
			if err := s.Serve(lis); err != nil {
				log.Logger.Sugar().Errorf("failed to serve: %v", err)
			}
		}()
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

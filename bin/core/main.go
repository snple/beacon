package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dgraph-io/badger/v4"
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/core/config"
	"github.com/snple/beacon/bin/core/log"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/core/node"
	tcp_node "github.com/snple/beacon/tcp/node"
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
	db, err := badger.Open(badgerOpts)
	if err != nil {
		log.Logger.Sugar().Fatalf("opening badger db: %v", err)
	}

	defer db.Close()

	command := flag.Arg(0)
	switch command {
	case "seed":
		log.Logger.Sugar().Infof("seed: Completed")
		return
	}

	coreOpts := make([]core.CoreOption, 0)

	cs, err := core.Core(db, coreOpts...)
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

	if config.Config.NodeService.Enable {
		nodeOpts := make([]node.NodeOption, 0)

		ns, err := node.Node(cs, nodeOpts...)
		if err != nil {
			log.Logger.Sugar().Fatalf("NewNodeService: %v", err)
		}

		ns.Start()
		defer ns.Stop()

		grpcOpts := []grpc.ServerOption{
			grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{PermitWithoutStream: true}),
		}

		if config.Config.NodeService.TLS {
			tlsConfig, err := util.LoadServerCert(config.Config.NodeService.CA, config.Config.NodeService.Cert, config.Config.NodeService.Key)
			if err != nil {
				log.Logger.Sugar().Fatal(err)
			}

			grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		} else {
			grpcOpts = append(grpcOpts, grpc.Creds(insecure.NewCredentials()))
		}

		s := grpc.NewServer(grpcOpts...)
		defer s.Stop()

		ns.RegisterGrpc(s)

		lis, err := net.Listen("tcp", config.Config.NodeService.Addr)
		if err != nil {
			log.Logger.Sugar().Fatalf("failed to listen: %v", err)
		}

		go func() {
			log.Logger.Sugar().Infof("node grpc start: %v", config.Config.NodeService.Addr)
			if err := s.Serve(lis); err != nil {
				log.Logger.Sugar().Fatalf("failed to serve: %v", err)
			}
		}()
	}

	if config.Config.TcpNodeService.Enable {
		nodeOpts := make([]tcp_node.NodeOption, 0)

		nodeOpts = append(nodeOpts, tcp_node.WithAddr(config.Config.TcpNodeService.Addr))

		if config.Config.TcpNodeService.TLS {
			tlsConfig, err := util.LoadServerCert(config.Config.TcpNodeService.CA,
				config.Config.TcpNodeService.Cert, config.Config.TcpNodeService.Key)
			if err != nil {
				log.Logger.Sugar().Fatal(err)
			}

			nodeOpts = append(nodeOpts, tcp_node.WithTLSConfig(tlsConfig))
		}

		ns, err := tcp_node.Node(cs, nodeOpts...)
		if err != nil {
			log.Logger.Sugar().Fatalf("NewNodeService: %v", err)
		}

		log.Logger.Sugar().Infof("tcp node service start: %v", config.Config.TcpNodeService.Addr)

		ns.Start()
		defer ns.Stop()
	}

	if !config.Config.Gin.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

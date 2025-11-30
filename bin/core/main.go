package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/core/config"
	"github.com/snple/beacon/bin/core/log"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/core/node"
	"github.com/snple/beacon/db"
	"github.com/snple/beacon/http"
	"github.com/snple/beacon/http/core/api"
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

	bundb, err := db.ConnectSqlite(config.Config.DB.File, config.Config.DB.Debug)
	if err != nil {
		log.Logger.Sugar().Fatalf("connecting to db: %v", err)
	}

	defer bundb.Close()

	if err = core.CreateSchema(bundb); err != nil {
		log.Logger.Sugar().Fatalf("create schema: %v", err)
	}

	command := flag.Arg(0)
	switch command {
	case "seed":
		log.Logger.Sugar().Infof("seed: Completed")
		return
	}

	coreOpts := make([]core.CoreOption, 0)

	cs, err := core.Core(bundb, coreOpts...)
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

	if config.Config.ApiService.Enable {
		opts := make([]http.HttpServerOption, 0)

		opts = append(opts, http.WithAppName("api"))
		opts = append(opts, http.WithAddr(config.Config.ApiService.Addr))
		opts = append(opts, http.WithDebug(config.Config.ApiService.Debug))

		if config.Config.ApiService.TLS {
			if config.Config.ApiService.CA != "" {
				pool := x509.NewCertPool()

				ca, err := os.ReadFile(config.Config.ApiService.CA)
				if err != nil {
					log.Logger.Sugar().Fatal(err)
				}

				if ok := pool.AppendCertsFromPEM(ca); !ok {
					log.Logger.Sugar().Fatal(err)
				}

				tlsConfig := &tls.Config{
					ClientAuth: tls.RequireAndVerifyClientCert,
					ClientCAs:  pool,
				}

				opts = append(opts, http.WithTLSConfig(tlsConfig))
			}

			opts = append(opts, http.WithTLS(config.Config.ApiService.Cert, config.Config.ApiService.Key))
		}

		hs, err := http.NewHttpServer(cs.Context(), opts...)
		if err != nil {
			log.Logger.Sugar().Fatalf("NewHttpServer: %v", err)
		}

		{
			as, err := api.NewApiService(cs)
			if err != nil {
				log.Logger.Sugar().Fatalf("NewApiService: %v", err)
			}

			as.Register(hs.Engine())

			go as.Start()
			defer as.Stop()
		}

		go hs.Start()
		defer hs.Stop()
	}

	for _, static := range config.Config.Statics {
		if !static.Enable {
			continue
		}

		engine := gin.New()
		engine.Use(gin.Recovery())

		if config.Config.Gin.Debug {
			engine.Use(gin.LoggerWithWriter(os.Stdout))
		}

		engine.Static("/", static.Path)
		engine.NoRoute(func(ctx *gin.Context) {
			ctx.File(static.Path + "/index.html")
		})

		log.Logger.Sugar().Infof("static server start: %v", static.Addr)

		if static.TLS {
			go engine.RunTLS(static.Addr, static.Cert, static.Key)
		} else {
			go engine.Run(static.Addr)
		}
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

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
	"time"

	nethttp "net/http"
	_ "net/http/pprof"

	"github.com/dgraph-io/badger/v4"
	"github.com/gin-gonic/gin"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/edge/config"
	"github.com/snple/beacon/bin/edge/log"
	"github.com/snple/beacon/db"
	"github.com/snple/beacon/edge"
	"github.com/snple/beacon/http"
	"github.com/snple/beacon/http/edge/api"
	"github.com/snple/beacon/util"
	"github.com/snple/beacon/util/compress/zstd"
	"github.com/uptrace/bun"
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
			fmt.Printf("beacon edge version: %v\n", beacon.Version)
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

	if err = edge.CreateSchema(bundb); err != nil {
		log.Logger.Sugar().Fatalf("create schema: %v", err)
	}

	command := flag.Arg(0)
	switch command {
	case "seed":
		if flag.NArg() < 2 {
			log.Logger.Sugar().Fatalf("seed: node name is required")
		}

		nodeName := flag.Arg(1)

		// the seed of edge need to be executed manually
		// must provide the node name
		if err := edge.Seed(bundb, nodeName); err != nil {
			log.Logger.Sugar().Fatalf("seed: %v", err)
		}

		log.Logger.Sugar().Infof("seed: Completed")

		return
	case "pull", "push":
		if err := cli(command, bundb); err != nil {
			log.Logger.Sugar().Errorf("error: shutting down: %s", err)
		}

		return
	}

	edgeOpts := make([]edge.EdgeOption, 0)

	{
		edgeOpts = append(edgeOpts, edge.WithNodeID(config.Config.NodeID, config.Config.Secret))
		edgeOpts = append(edgeOpts, edge.WithLinkTTL(time.Second*time.Duration(config.Config.Status.LinkTTL)))

		edgeOpts = append(edgeOpts, edge.WithSync(edge.SyncOptions{
			TokenRefresh: time.Second * time.Duration(config.Config.Sync.TokenRefresh),
			Link:         time.Second * time.Duration(config.Config.Sync.Link),
			Interval:     time.Second * time.Duration(config.Config.Sync.Interval),
			Realtime:     config.Config.Sync.Realtime,
		}))

		badgerOptions := func() badger.Options {
			if config.Config.BadgerDB.InMemory {
				return badger.DefaultOptions("").WithInMemory(true)
			}

			return badger.DefaultOptions(config.Config.BadgerDB.Path)
		}()

		edgeOpts = append(edgeOpts, edge.WithBadger(badgerOptions))
	}

	if config.Config.NodeClient.Enable {
		kacp := keepalive.ClientParameters{
			Time:                120 * time.Second, // send pings every 120 seconds if there is no activity
			Timeout:             10 * time.Second,  // wait 10 second for ping ack before considering the connection dead
			PermitWithoutStream: true,              // send pings even without active streams
		}

		grpcOpts := []grpc.DialOption{
			grpc.WithKeepaliveParams(kacp),
			grpc.WithDefaultCallOptions(grpc.UseCompressor(zstd.Name)),
		}

		if config.Config.NodeClient.TLS {
			tlsConfig, err := util.LoadClientCert(
				config.Config.NodeClient.CA,
				config.Config.NodeClient.Cert,
				config.Config.NodeClient.Key,
				config.Config.NodeClient.ServerName,
				config.Config.NodeClient.InsecureSkipVerify,
			)
			if err != nil {
				log.Logger.Sugar().Fatalf("LoadClientCert: %v", err)
			}

			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		edgeOpts = append(edgeOpts, edge.WithNode(edge.NodeOptions{
			Enable:      true,
			Addr:        config.Config.NodeClient.Addr,
			GRPCOptions: grpcOpts,
		}))
	}

	es, err := edge.Edge(bundb, edgeOpts...)
	if err != nil {
		log.Logger.Sugar().Fatalf("NewEdgeService: %v", err)
	}

	es.Start()
	defer es.Stop()

	if config.Config.EdgeService.Enable {
		grpcOpts := []grpc.ServerOption{
			grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{PermitWithoutStream: true}),
		}

		if config.Config.EdgeService.TLS {
			tlsConfig, err := util.LoadServerCert(config.Config.EdgeService.CA, config.Config.EdgeService.Cert, config.Config.EdgeService.Key)
			if err != nil {
				log.Logger.Sugar().Fatal(err)
			}

			grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(tlsConfig)))
		} else {
			grpcOpts = append(grpcOpts, grpc.Creds(insecure.NewCredentials()))
		}

		s := grpc.NewServer(grpcOpts...)
		defer s.Stop()
		es.Register(s)

		lis, err := net.Listen("tcp", config.Config.EdgeService.Addr)
		if err != nil {
			log.Logger.Sugar().Fatalf("failed to listen: %v", err)
		}

		go func() {
			log.Logger.Sugar().Infof("edge grpc start: %v, tls: %v", config.Config.EdgeService.Addr, config.Config.EdgeService.TLS)
			if err := s.Serve(lis); err != nil {
				log.Logger.Sugar().Fatalf("failed to serve: %v", err)
			}
		}()
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

		hs, err := http.NewHttpServer(es.Context(), opts...)
		if err != nil {
			log.Logger.Sugar().Fatalf("NewHttpServer: %v", err)
		}

		{
			as, err := api.NewApiService(es)
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
		engine.Static("/", static.Path)
		engine.NoRoute(func(ctx *gin.Context) {
			ctx.File(static.Path + "/index.html")
		})

		log.Logger.Sugar().Infof("static server start： %v", static.Addr)

		if static.TLS {
			go engine.RunTLS(static.Addr, static.Cert, static.Key)
		} else {
			go engine.Run(static.Addr)
		}
	}

	go func() {
		nethttp.ListenAndServe(":6060", nil)
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

func cli(command string, bundb *bun.DB) error {
	log.Logger.Sugar().Infof("cli %v: Started", command)
	defer log.Logger.Sugar().Infof("cli %v : Completed", command)

	edgeOpts := make([]edge.EdgeOption, 0)
	edgeOpts = append(edgeOpts, edge.WithNodeID(config.Config.NodeID, config.Config.Secret))

	{
		kacp := keepalive.ClientParameters{
			Time:                120 * time.Second, // send pings every 120 seconds if there is no activity
			Timeout:             10 * time.Second,  // wait 10 second for ping ack before considering the connection dead
			PermitWithoutStream: true,              // send pings even without active streams
		}

		grpcOpts := []grpc.DialOption{
			grpc.WithKeepaliveParams(kacp),
			grpc.WithDefaultCallOptions(grpc.UseCompressor(zstd.Name)),
		}

		if config.Config.NodeClient.TLS {
			tlsConfig, err := util.LoadClientCert(
				config.Config.NodeClient.CA,
				config.Config.NodeClient.Cert,
				config.Config.NodeClient.Key,
				config.Config.NodeClient.ServerName,
				config.Config.NodeClient.InsecureSkipVerify,
			)
			if err != nil {
				log.Logger.Sugar().Fatalf("LoadClientCert: %v", err)
			}

			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		edgeOpts = append(edgeOpts, edge.WithNode(edge.NodeOptions{
			Addr:        config.Config.NodeClient.Addr,
			GRPCOptions: grpcOpts,
		}))
	}

	es, err := edge.Edge(bundb, edgeOpts...)
	if err != nil {
		log.Logger.Sugar().Fatalf("NewEdgeService: %v", err)
	}

	switch command {
	case "push":
		return es.Push()
	case "pull":
		return es.Pull()
	}

	return nil
}

package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon"
	"github.com/snple/beacon/bin/edge/config"
	"github.com/snple/beacon/bin/edge/log"
	"github.com/snple/beacon/edge"
	"github.com/snple/beacon/util"
	"github.com/snple/beacon/util/compress/zstd"
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

	badgerOpts := badger.DefaultOptions(config.Config.DB.File)
	badgerOpts.Logger = nil

	command := flag.Arg(0)
	switch command {
	case "seed":
		if flag.NArg() < 2 {
			log.Logger.Sugar().Fatalf("seed: node name is required")
		}

		// nodeName := flag.Arg(1)

		// the seed of edge need to be executed manually
		// must provide the node name
		// if err := edge.Seed(db, nodeName); err != nil {
		// 	log.Logger.Sugar().Fatalf("seed: %v", err)
		// }

		log.Logger.Sugar().Infof("seed: Completed")

		return
	case "pull", "push":
		if err := cli(command); err != nil {
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

		edgeOpts = append(edgeOpts, edge.WithBadger(badgerOpts))
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
			Enable: true,
		}))
	}

	es, err := edge.Edge(edgeOpts...)
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

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	<-signalCh
}

func cli(command string) error {
	log.Logger.Sugar().Infof("cli %v: Started", command)
	defer log.Logger.Sugar().Infof("cli %v : Completed", command)

	edgeOpts := make([]edge.EdgeOption, 0)
	edgeOpts = append(edgeOpts, edge.WithNodeID(config.Config.NodeID, config.Config.Secret))

	badgerOpts := badger.DefaultOptions(config.Config.DB.File)
	badgerOpts.Logger = nil
	edgeOpts = append(edgeOpts, edge.WithBadger(badgerOpts))

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
	}

	es, err := edge.Edge(edgeOpts...)
	if err != nil {
		log.Logger.Sugar().Fatalf("NewEdgeService: %v", err)
	}

	switch command {
	case "push":
		return es.Push()
	}

	return nil
}

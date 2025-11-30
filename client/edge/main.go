package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"time"

	"github.com/snple/beacon/client/edge/service"
	"github.com/snple/beacon/pb/edges"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

const (
	address = "127.0.0.1:6010"
)

func main() {
	logger, _ := zap.NewDevelopment()

	logger.Info("main : Started")
	defer logger.Info("main : Completed")

	cfg, err := loadCert()
	if err != nil {
		logger.Fatal(err.Error())
	}

	var kacp = keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}

	// Set up a connection to the server.
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(credentials.NewTLS(cfg)),
		// grpc.WithInsecure(),
		// grpc.WithBlock(),
		grpc.WithKeepaliveParams(kacp),
	)
	if err != nil {
		logger.Sugar().Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()
	_ = ctx

	node := edges.NewNodeServiceClient(conn)
	service.NodeView(ctx, node)
	// service.NodeUpdate(ctx, node)

	// wire := edges.NewWireServiceClient(conn)
	// service.WireList(ctx, wire)
	// service.WireView(ctx, wire)
	// service.WireName(ctx, wire)
	// service.WireCreate(ctx, wire)
	// service.WireUpdate(ctx, wire)
	// service.WireDelete(ctx, wire)
	// service.WireLink(ctx, wire)

	pin := edges.NewPinServiceClient(conn)
	// service.PinList(ctx, pin)
	// service.PinView(ctx, pin)
	// service.PinName(ctx, pin)
	// service.PinCreate(ctx, pin)
	// service.PinUpdate(ctx, pin)
	// service.PinDelete(ctx, pin)
	// t1 := time.Now()
	// for i := 0; i < 10000; i++ {
	// service.PinSetValue(ctx, pin)
	// service.PinGetValue(ctx, pin)
	service.PinGetWrite(ctx, pin)
	// }
	// t2 := time.Now()
	// fmt.Println("t2-t1", t2.Sub(t1))
}

func loadCert() (*tls.Config, error) {
	pool := x509.NewCertPool()

	ca, err := os.ReadFile("certs/ca.crt")
	if err != nil {
		return nil, err
	}

	if ok := pool.AppendCertsFromPEM(ca); !ok {
		return nil, errors.New("pool.AppendCertsFromPEM err")
	}

	cert, err := tls.LoadX509KeyPair("certs/client.crt", "certs/client.key")
	if err != nil {
		return nil, err
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// ServerName:         "snple.com",
		RootCAs:            pool,
		InsecureSkipVerify: true,
	}

	return cfg, nil
}

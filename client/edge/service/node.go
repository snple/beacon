package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func NodeView(ctx context.Context, client edges.NodeServiceClient) {
	request := &pb.MyEmpty{}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

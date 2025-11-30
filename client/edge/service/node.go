package service

import (
	"context"
	"log"

	"github.com/snple/beacon/consts"
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

func NodeUpdate(ctx context.Context, client edges.NodeServiceClient) {
	request := &pb.Node{
		Id:     "0187f0bb5e6cfdd553884496",
		Name:   "node1",
		Secret: "123456.",
		Status: consts.ON,
	}

	reply, err := client.Update(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

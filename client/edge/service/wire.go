package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func WireList(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.MyEmpty{}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireView(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.Id{Id: "017a053b3f7be81d700e0976"}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireName(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.Name{Name: "Wire"}

	reply, err := client.Name(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

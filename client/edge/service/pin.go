package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func PinList(ctx context.Context, client edges.PinServiceClient) {
	request := &edges.PinListRequest{}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinView(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Id{Id: "017a9b416ef270dc2380ce54"}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinName(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Name{Name: "PIN"}

	reply, err := client.Name(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

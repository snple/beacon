package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb/cores"
)

func WireList(ctx context.Context, client cores.WireServiceClient) {
	request := &cores.WireListRequest{
		NodeId: "01946a0cabdabc925941e98a",
	}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireView(ctx context.Context, client cores.WireServiceClient) {
	request := &cores.WireViewRequest{
		NodeId: "01946a0cabdabc925941e98a",
		WireId: "01946a0cabdabc925941e98b",
	}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireName(ctx context.Context, client cores.WireServiceClient) {
	request := &cores.WireNameRequest{
		NodeId: "01946a0cabdabc925941e98a",
		Name:   "wire",
	}

	reply, err := client.Name(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

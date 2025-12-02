package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb/cores"
)

func PinList(ctx context.Context, client cores.PinServiceClient) {
	request := &cores.PinListRequest{
		NodeId: "01946a0cabdabc925941e98a",
	}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinView(ctx context.Context, client cores.PinServiceClient) {
	request := &cores.PinViewRequest{
		NodeId: "01946a0cabdabc925941e98a",
		PinId:  "01880166c70f451c041bb351",
	}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinName(ctx context.Context, client cores.PinServiceClient) {
	request := &cores.PinNameRequest{
		NodeId: "01946a0cabdabc925941e98a",
		Name:   "pin",
	}

	reply, err := client.Name(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

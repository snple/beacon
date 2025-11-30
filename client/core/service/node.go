package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
)

func NodeList(ctx context.Context, client cores.NodeServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "",
		Search:  "",
	}

	request := &cores.NodeListRequest{
		Page: &page,
		Tags: "",
	}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeView(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Id{Id: "017a053b3f7be81caa209b8e"}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeName(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Name{Name: "node"}

	reply, err := client.Name(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

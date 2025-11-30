package service

import (
	"context"
	"log"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func WireList(ctx context.Context, client edges.WireServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "name",
	}

	request := &edges.WireListRequest{
		Page: &page,
	}

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

func WireCreate(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.Wire{
		Name:     "wire1",
		Clusters: "",
		Status:   consts.ON,
	}

	reply, err := client.Create(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireUpdate(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.Id{Id: "018849d40fdd9a6bfb3e5d54"}

	reply, err := client.View(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)

	reply.Status = consts.ON

	reply2, err := client.Update(ctx, reply)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply2)
}

func WireDelete(ctx context.Context, client edges.WireServiceClient) {
	request := &pb.Id{Id: "017a09b3af1f47a21d6b84c0"}

	reply, err := client.Delete(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

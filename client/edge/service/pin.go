package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func PinList(ctx context.Context, client edges.PinServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "name",
	}

	request := &edges.PinListRequest{
		Page: &page,
	}

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

func PinGetValue(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Id{Id: "01946a5aae65c0ceeaa257db"}

	reply, err := client.GetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinSetValue(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.PinValue{
		Id:    "01946a5aae65c0ceeaa257db",
		Value: fmt.Sprintf("%v", rand.Float64()*100),
	}

	reply, err := client.SetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

/*
func PinSetWrite(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.PinValue{
		Id:    "01946a5aae65c0ceeaa257db",
		Value: fmt.Sprintf("%v", rand.Float64()*100),
	}

	reply, err := client.SetWrite(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}
*/

func PinGetWrite(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Id{Id: "01946a5aae65c0ceeaa257db"}

	reply, err := client.GetWrite(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
)

func PinList(ctx context.Context, client cores.PinServiceClient) {
	page := pb.Page{
		Limit:  10,
		Offset: 0,
	}

	request := &cores.PinListRequest{
		NodeId: "01946a0cabdabc925941e98a",
		Page:   &page,
	}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinView(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.Id{Id: "01880166c70f451c041bb351"}

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

func PinGetValue(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.Id{Id: "01880166c70f451c041bb351"}

	reply, err := client.GetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

/*
func PinSetValue(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.PinValue{
		Id:    "01880166c70f451c041bb351",
		Value: fmt.Sprintf("%v", rand.Int31()),
	}

	reply, err := client.SetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}
*/

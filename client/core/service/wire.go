package service

import (
	"context"
	"log"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
)

func WireList(ctx context.Context, client cores.WireServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "name",
	}

	request := &cores.WireListRequest{
		Page: &page,
	}

	reply, err := client.List(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireView(ctx context.Context, client cores.WireServiceClient) {
	request := &pb.Id{Id: "01946a0cabdabc925941e98a"}

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

func WireCreate(ctx context.Context, client cores.WireServiceClient) {
	request := &pb.Wire{
		NodeId: "01946a0cabdabc925941e98a",
		Name:   "wire",
		Desc:   "wire",
		Source: "source",
		Config: "config",
		Status: consts.ON,
		Tags:   "tag1,tag2",
	}

	reply, err := client.Create(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireUpdate(ctx context.Context, client cores.WireServiceClient) {
	request := &pb.Wire{
		Id:     "01946a51cd5bc0cd7a776f35",
		Name:   "wire",
		Desc:   "wire",
		Source: "source",
		Config: "config",
		Status: consts.ON,
		Tags:   "tag1,tag2",
	}

	reply, err := client.Update(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func WireDelete(ctx context.Context, client cores.WireServiceClient) {
	request := &pb.Id{Id: "01946a51cd5bc0cd7a776f35"}

	reply, err := client.Delete(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

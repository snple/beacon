package service

import (
	"context"
	"log"

	"github.com/snple/beacon/consts"
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

func NodeCreate(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Node{
		Name:   "test_node2",
		Secret: "123456",
		Status: consts.ON,
	}

	reply, err := client.Create(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeUpdate(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Node{
		Id:     "01946a0cabdabc925941e98a",
		Name:   "node",
		Secret: "123456.",
		Status: consts.ON,
	}

	reply, err := client.Update(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeDelete(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Id{Id: "017a0980738df69c17287eab"}

	reply, err := client.Delete(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeDestory(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Id{Id: "0189f3d94f0d1579c4e2a817"}

	reply, err := client.Destory(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func NodeClone(ctx context.Context, client cores.NodeServiceClient) {
	request := &pb.Id{Id: "0187f0bb5e6cfdd553884496"}

	reply, err := client.Clone(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

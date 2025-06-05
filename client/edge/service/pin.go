package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
)

func PinList(ctx context.Context, client edges.PinServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "name",
		// Search:  "t",
	}

	request := &edges.PinListRequest{
		Page: &page,
		// NodeId: "017a053b3f7be81caa209b8e",
		// WireId: "017a9b416ef270dbd799c1f5",
		// Tags: "aaa,bbb",
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

func PinCreate(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Pin{
		WireId: "0187712e361544594841b6fb",
		Name:   "pin1",
		Desc:   "",
		Addr:   "test_addr",
		Type:   "F32",
		Rw:     consts.WRITE,
		Status: consts.ON,
	}

	reply, err := client.Create(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinUpdate(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Pin{
		Id:     "017b213669a4984d98b160a6",
		Name:   "PIN",
		Desc:   "",
		Status: -1,
		Tags:   "aaa,bbb",
	}

	reply, err := client.Update(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinDelete(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Id{Id: "017a9b776bda0c0c7bcd3435"}

	reply, err := client.Delete(ctx, request)

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
		// Value: "1",
	}

	reply, err := client.SetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

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

func PinGetWrite(ctx context.Context, client edges.PinServiceClient) {
	request := &pb.Id{Id: "01946a5aae65c0ceeaa257db"}

	reply, err := client.GetWrite(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

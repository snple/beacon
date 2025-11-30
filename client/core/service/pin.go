package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
)

func PinList(ctx context.Context, client cores.PinServiceClient) {
	page := pb.Page{
		Limit:   10,
		Offset:  0,
		OrderBy: "name",
		// Search:  "t",
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

func PinCreate(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.Pin{
		WireId: "01946a51cd5bc0cd7a776f35",
		Name:   "pin1",
		Addr:   "",
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

func PinUpdate(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.Pin{
		Id:     "01880166c70f451c041bb351",
		Name:   "PIN",
		Status: consts.ON,
	}

	reply, err := client.Update(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

func PinDelete(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.Id{Id: "01880166c70f451c041bb351"}

	reply, err := client.Delete(ctx, request)

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

func PinSetValue(ctx context.Context, client cores.PinServiceClient) {
	request := &pb.PinValue{
		Id: "01880166c70f451c041bb351",
		// Value: fmt.Sprintf("%v", rand.Float64()*100),
		Value: fmt.Sprintf("%v", rand.Int31()),
		// Value: "1",
	}

	reply, err := client.SetValue(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

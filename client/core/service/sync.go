package service

import (
	"context"
	"log"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
)

func GetNodeUpdated(ctx context.Context, client cores.SyncServiceClient) {
	request := &pb.Id{Id: "0189f3d94f0d1579c4e2a817"}

	reply, err := client.GetNodeUpdated(ctx, request)

	if err != nil {
		log.Fatalf("Error when calling grpc service: %s", err)
	}
	log.Printf("Resp received: %v", reply)
}

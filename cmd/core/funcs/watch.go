package funcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/types/cache"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type Watch struct {
	root *Root
}

func watchCmd(root *Root) *cobra.Command {
	watch := &Watch{
		root: root,
	}

	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch node, wire, pin, and const",
		Long:  "Watch node, wire, pin, and const",
	}

	watchCmd.AddCommand(watch.pinValueCmd())

	return watchCmd
}

func (w *Watch) pinValueCmd() *cobra.Command {
	watchPinValueCmd := &cobra.Command{
		Use:   "pin-value [name]",
		Short: "Watch pin value",
		Long:  "Watch pin value",
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			w.pinValue(ctx, w.root.GetConn(), cmd, args[0])
		},
	}

	return watchPinValueCmd
}

func (w *Watch) pinValue(ctx context.Context, conn *grpc.ClientConn, _ *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)
	pinServiceClient := cores.NewPinServiceClient(conn)
	syncClient := cores.NewSyncServiceClient(conn)

	nodeReply, err := nodeClient.Name(ctx, &pb.Name{
		Name: name,
	})
	if err != nil {
		w.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node: %s, %s\n", nodeReply.Id, nodeReply.Name)

	pinValueUpdated, err := syncClient.GetPinValueUpdated(ctx, &pb.Id{Id: nodeReply.Id})
	if err != nil {
		w.root.logger.Error("failed to get pin value updated", zap.Error(err))
		os.Exit(1)
	}

	stream, err := syncClient.WaitPinValueUpdated(ctx, &pb.Id{Id: nodeReply.Id})
	if err != nil {
		w.root.logger.Error("failed to watch pin value", zap.Error(err))
		os.Exit(1)
	}

	after := pinValueUpdated.Updated
	limit := uint32(100)

	type pinCacheKey struct {
		wireId string
		pinId  string
	}

	type pinCacheValue struct {
		wireName string
		pinName  string
		pinDesc  string
	}

	pinCache := cache.NewCache(func(ctx context.Context, key pinCacheKey) (pinCacheValue, time.Duration, error) {
		wireReply, err := wireClient.View(ctx, &pb.Id{Id: key.wireId})
		if err != nil {
			return pinCacheValue{}, 0, err
		}

		pinReply, err := pinServiceClient.View(ctx, &pb.Id{Id: key.pinId})
		if err != nil {
			return pinCacheValue{}, 0, err
		}

		return pinCacheValue{wireName: wireReply.Name, pinName: pinReply.Name, pinDesc: pinReply.Desc}, time.Second * 60 * 3, nil
	})

	for {
		_, err := stream.Recv()
		if err != nil {
			w.root.logger.Error("failed to receive pin value", zap.Error(err))
			os.Exit(1)
		}

		for {
			remotes, err := pinServiceClient.PullValue(ctx, &cores.PinPullValueRequest{After: after, Limit: limit})
			if err != nil {
				w.root.logger.Error("failed to pull pin value", zap.Error(err))
				os.Exit(1)
			}

			for _, pin := range remotes.Pins {
				after = pin.Updated

				option, err := pinCache.GetWithMiss(ctx, pinCacheKey{wireId: pin.WireId, pinId: pin.Id})
				if err != nil {
					w.root.logger.Error("failed to get pin cache", zap.Error(err))
					os.Exit(1)
				}

				if option.IsSome() {
					pinCacheValue := option.Unwrap()

					fmt.Printf("name: %s.%s.%s, desc: %s, value: %s\n", nodeReply.Name, pinCacheValue.wireName, pinCacheValue.pinName, pinCacheValue.pinDesc, pin.Value)
				}
			}

			if len(remotes.Pins) < int(limit) {
				break
			}
		}
	}
}

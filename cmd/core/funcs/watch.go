package funcs

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
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
		Long:  `Watch node, wire, pin, and const`,
	}

	watchCmd.AddCommand(watch.watchPinValueCmd())

	return watchCmd
}

func (w *Watch) watchPinValueCmd() *cobra.Command {
	watchPinValueCmd := &cobra.Command{
		Use:   "pin-value",
		Short: "Watch pin value",
		Long:  `Watch node`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			w.watchPinValue(ctx, w.root.GetConn(), cmd, args[0])
		},
	}

	return watchPinValueCmd
}

func (w *Watch) watchPinValue(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	pinServiceClient := cores.NewPinServiceClient(conn)
	syncClient := cores.NewSyncServiceClient(conn)

	reply, err := nodeClient.Name(ctx, &pb.Name{
		Name: name,
	})
	if err != nil {
		w.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node: %s, %s\n", reply.Id, reply.Name)

	pinValueUpdated, err := syncClient.GetPinValueUpdated(ctx, &pb.Id{Id: reply.Id})
	if err != nil {
		w.root.logger.Error("failed to get pin value updated", zap.Error(err))
		os.Exit(1)
	}

	stream, err := syncClient.WaitPinValueUpdated(ctx, &pb.Id{Id: reply.Id})
	if err != nil {
		w.root.logger.Error("failed to watch pin value", zap.Error(err))
		os.Exit(1)
	}

	after := pinValueUpdated.Updated
	limit := uint32(100)

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

			for _, remote := range remotes.Pins {
				fmt.Printf("pin value: %s\n", remote)

				after = remote.Updated
			}

			if len(remotes.Pins) < int(limit) {
				break
			}
		}
	}
}

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

type Pin struct {
	root *Root
}

func pinCmd(root *Root) *cobra.Command {
	pin := &Pin{
		root: root,
	}

	pinCmd := &cobra.Command{
		Use:   "pin",
		Short: "Pin",
		Long:  `Pin`,
	}

	pinCmd.AddCommand(pin.getValueCmd())
	pinCmd.AddCommand(pin.setValueCmd())

	return pinCmd
}

func (p *Pin) getValueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-value [name]",
		Short: "Get pin value",
		Long:  `Get pin value`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a pin name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			p.getValue(ctx, p.root.GetConn(), cmd, args[0])
		},
	}

	cmd.PersistentFlags().StringP("node", "n", "", "node name")

	return cmd
}

func (p *Pin) getValue(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		p.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	var pinReply *pb.Pin

	if nodeName != "" {
		nodeReply, err := nodeClient.Name(ctx, &pb.Name{
			Name: nodeName,
		})
		if err != nil {
			p.root.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}

		pinReply, err = pinClient.Name(ctx, &cores.PinNameRequest{
			NodeId: nodeReply.Id,
			Name:   name,
		})
		if err != nil {
			p.root.logger.Error("failed to get pin", zap.Error(err))
			os.Exit(1)
		}

	} else {
		pinReply, err = pinClient.NameFull(ctx, &pb.Name{
			Name: name,
		})
		if err != nil {
			p.root.logger.Error("failed to get pin", zap.Error(err))
			os.Exit(1)
		}
	}

	pinValueReply, err := pinClient.GetValue(ctx, &pb.Id{
		Id: pinReply.Id,
	})
	if err != nil {
		p.root.logger.Error("failed to get pin value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", pinReply.Name, pinReply.Desc, pinValueReply.Value)
}

func (p *Pin) setValueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-value [name] [value]",
		Short: "Set pin value",
		Long:  `Set pin value`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(2)(cmd, args) != nil {
				return errors.New("must provide a pin name and value")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			p.setValue(ctx, p.root.GetConn(), cmd, args[0], args[1])
		},
	}

	cmd.PersistentFlags().StringP("node", "n", "", "node name")

	return cmd
}

func (p *Pin) setValue(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string, value string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		p.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	var pinReply *pb.Pin

	if nodeName != "" {
		nodeReply, err := nodeClient.Name(ctx, &pb.Name{
			Name: nodeName,
		})
		if err != nil {
			p.root.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}

		pinReply, err = pinClient.Name(ctx, &cores.PinNameRequest{
			NodeId: nodeReply.Id,
			Name:   name,
		})
		if err != nil {
			p.root.logger.Error("failed to get pin", zap.Error(err))
			os.Exit(1)
		}

	} else {
		pinReply, err = pinClient.NameFull(ctx, &pb.Name{
			Name: name,
		})
		if err != nil {
			p.root.logger.Error("failed to get pin", zap.Error(err))
			os.Exit(1)
		}
	}

	_, err = pinClient.SetValue(ctx, &pb.PinValue{
		Id:    pinReply.Id,
		Value: value,
	})
	if err != nil {
		p.root.logger.Error("failed to get pin value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", pinReply.Name, pinReply.Desc, value)
}

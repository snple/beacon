package funcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
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
		Long:  "Pin",
	}

	pinCmd.AddCommand(pin.listCmd())
	pinCmd.AddCommand(pin.getValueCmd())
	pinCmd.AddCommand(pin.setValueCmd())
	pinCmd.AddCommand(pin.getWriteCmd())
	pinCmd.AddCommand(pin.setWriteCmd())

	return pinCmd
}

func (p *Pin) listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pins",
		Long:  "List pins",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			p.list(ctx, p.root.GetConn(), cmd)
		},
	}

	cmd.Flags().Int("limit", 10, "limit")
	cmd.Flags().Int("offset", 0, "offset")
	cmd.Flags().String("order_by", "", "order by")
	cmd.Flags().String("search", "", "search")
	cmd.Flags().String("tags", "", "tags")

	cmd.Flags().StringP("node", "n", "", "node name")
	cmd.Flags().StringP("wire", "w", "", "wire name")

	return cmd
}

func (p *Pin) list(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		p.root.logger.Error("failed to get limit", zap.Error(err))
		os.Exit(1)
	}

	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		p.root.logger.Error("failed to get offset", zap.Error(err))
		os.Exit(1)
	}

	orderBy, err := cmd.Flags().GetString("order_by")
	if err != nil {
		p.root.logger.Error("failed to get order by", zap.Error(err))
		os.Exit(1)
	}

	search, err := cmd.Flags().GetString("search")
	if err != nil {
		p.root.logger.Error("failed to get search", zap.Error(err))
		os.Exit(1)
	}

	tags, err := cmd.Flags().GetString("tags")
	if err != nil {
		p.root.logger.Error("failed to get tags", zap.Error(err))
		os.Exit(1)
	}

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		p.root.logger.Error("failed to get node name", zap.Error(err))
		os.Exit(1)
	}

	wireName, err := cmd.Flags().GetString("wire")
	if err != nil {
		p.root.logger.Error("failed to get wire name", zap.Error(err))
		os.Exit(1)
	}

	if nodeName != "" && wireName != "" {
		fmt.Println("node and wire cannot be used together")
		os.Exit(1)
	}

	if nodeName == "" && wireName == "" {
		fmt.Println("node or wire is required")
		os.Exit(1)
	}

	page := pb.Page{
		Limit:   uint32(limit),
		Offset:  uint32(offset),
		OrderBy: orderBy,
		Search:  search,
	}

	var nodeId string
	var wireId string

	if nodeName != "" {
		nodeReply, err := nodeClient.Name(ctx, &pb.Name{
			Name: nodeName,
		})
		if err != nil {
			p.root.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}

		nodeId = nodeReply.Id
	}

	if wireName != "" {
		wireReply, err := wireClient.NameFull(ctx, &pb.Name{
			Name: wireName,
		})
		if err != nil {
			p.root.logger.Error("failed to get wire", zap.Error(err))
			os.Exit(1)
		}

		wireId = wireReply.Id
	}

	pinReply, err := pinClient.List(ctx, &cores.PinListRequest{
		Page:   &page,
		NodeId: nodeId,
		WireId: wireId,
		Tags:   tags,
	})
	if err != nil {
		p.root.logger.Error("failed to get pin", zap.Error(err))
		os.Exit(1)
	}

	t := table.NewWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Desc", "Tags", "Type", "Addr", "Value", "Status", "Created", "Updated"})

	timeformatFn := func(t int64) string {
		return time.UnixMicro(t).Format("2006-01-02 15:04:05")
	}

	pinNamePrefix := wireName

	for _, pin := range pinReply.Pins {
		if pinNamePrefix == "" {
			wireReply, err := wireClient.View(ctx, &pb.Id{
				Id: pin.WireId,
			})
			if err != nil {
				p.root.logger.Error("failed to get wire", zap.Error(err))
				os.Exit(1)
			}

			pinNamePrefix = fmt.Sprintf("%s.%s", nodeName, wireReply.Name)
		}

		t.AppendRow(table.Row{pin.Id, fmt.Sprintf("%s.%s", pinNamePrefix, pin.Name), pin.Desc, pin.Tags, pin.Type, pin.Addr, pin.Value, pin.Status, timeformatFn(pin.Created), timeformatFn(pin.Updated)})
	}

	// time to take a peek
	t.SetCaption(fmt.Sprintf("Total: %d/%d Rows.", len(pinReply.Pins), pinReply.Count))
	fmt.Println(t.Render())
}

func (p *Pin) getValueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-value [name]",
		Short: "Get pin value",
		Long:  "Get pin value",
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

	cmd.Flags().StringP("node", "n", "", "node name")

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
		Long:  "Set pin value",
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

	cmd.Flags().StringP("node", "n", "", "node name")

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

func (p *Pin) getWriteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-write [name]",
		Short: "Get pin write",
		Long:  "Get pin write",
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a pin name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			p.getWrite(ctx, p.root.GetConn(), cmd, args[0])
		},
	}

	cmd.Flags().StringP("node", "n", "", "node name")

	return cmd
}

func (p *Pin) getWrite(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
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

	pinWriteReply, err := pinClient.GetWrite(ctx, &pb.Id{
		Id: pinReply.Id,
	})
	if err != nil {
		p.root.logger.Error("failed to get pin value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", pinReply.Name, pinReply.Desc, pinWriteReply.Value)
}

func (p *Pin) setWriteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-write [name] [value]",
		Short: "Set pin write",
		Long:  "Set pin write",
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(2)(cmd, args) != nil {
				return errors.New("must provide a pin name and value")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			p.setWrite(ctx, p.root.GetConn(), cmd, args[0], args[1])
		},
	}

	cmd.Flags().StringP("node", "n", "", "node name")

	return cmd
}

func (p *Pin) setWrite(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string, value string) {
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

	_, err = pinClient.SetWrite(ctx, &pb.PinValue{
		Id:    pinReply.Id,
		Value: value,
	})
	if err != nil {
		p.root.logger.Error("failed to get pin value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", pinReply.Name, pinReply.Desc, value)
}

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

type Consts struct {
	root *Root
}

func constsCmd(root *Root) *cobra.Command {
	consts := &Consts{
		root: root,
	}

	constsCmd := &cobra.Command{
		Use:   "consts",
		Short: "Consts",
		Long:  "Consts",
	}

	constsCmd.AddCommand(consts.listCmd())
	constsCmd.AddCommand(consts.getValueCmd())
	constsCmd.AddCommand(consts.setValueCmd())

	return constsCmd
}

func (c *Consts) listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List consts",
		Long:  "List consts",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			c.list(ctx, c.root.GetConn(), cmd)
		},
	}

	cmd.Flags().Int("limit", 10, "limit")
	cmd.Flags().Int("offset", 0, "offset")
	cmd.Flags().String("order_by", "", "order by")
	cmd.Flags().String("search", "", "search")
	cmd.Flags().String("tags", "", "tags")

	cmd.Flags().StringP("node", "n", "", "node name")
	cmd.MarkFlagRequired("node")

	return cmd
}

func (c *Consts) list(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command) {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		c.root.logger.Error("failed to get limit", zap.Error(err))
		os.Exit(1)
	}

	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		c.root.logger.Error("failed to get offset", zap.Error(err))
		os.Exit(1)
	}

	orderBy, err := cmd.Flags().GetString("order_by")
	if err != nil {
		c.root.logger.Error("failed to get order by", zap.Error(err))
		os.Exit(1)
	}

	search, err := cmd.Flags().GetString("search")
	if err != nil {
		c.root.logger.Error("failed to get search", zap.Error(err))
		os.Exit(1)
	}

	tags, err := cmd.Flags().GetString("tags")
	if err != nil {
		c.root.logger.Error("failed to get tags", zap.Error(err))
		os.Exit(1)
	}

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		c.root.logger.Error("failed to get node name", zap.Error(err))
		os.Exit(1)
	}

	page := pb.Page{
		Limit:   uint32(limit),
		Offset:  uint32(offset),
		OrderBy: orderBy,
		Search:  search,
	}

	nodeClient := cores.NewNodeServiceClient(conn)
	constsClient := cores.NewConstServiceClient(conn)

	nodeReply, err := nodeClient.Name(ctx, &pb.Name{
		Name: nodeName,
	})
	if err != nil {
		c.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node: %s, %s\n", nodeReply.Id, nodeReply.Name)

	constsReply, err := constsClient.List(ctx, &cores.ConstListRequest{
		Page:   &page,
		Tags:   tags,
		NodeId: nodeReply.Id,
	})
	if err != nil {
		c.root.logger.Error("failed to get const list", zap.Error(err))
		os.Exit(1)
	}

	t := table.NewWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Desc", "Tags", "Type", "Value", "Status", "Created", "Updated"})

	timeformatFn := func(t int64) string {
		return time.UnixMicro(t).Format("2006-01-02 15:04:05")
	}

	for _, consts := range constsReply.Consts {
		t.AppendRow(table.Row{consts.Id, fmt.Sprintf("%s.%s", nodeName, consts.Name), consts.Desc, consts.Tags, consts.Type, consts.Value, consts.Status, timeformatFn(consts.Created), timeformatFn(consts.Updated)})
	}

	// time to take a peek
	t.SetCaption(fmt.Sprintf("Total: %d/%d Rows.", len(constsReply.Consts), constsReply.Count))
	fmt.Println(t.Render())
}

func (c *Consts) getValueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get-value [name]",
		Short: "Get const value",
		Long:  "Get const value",
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a pin name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			c.getValue(ctx, c.root.GetConn(), cmd, args[0])
		},
	}

	cmd.Flags().StringP("node", "n", "", "node name")

	return cmd
}

func (c *Consts) getValue(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	constsClient := cores.NewConstServiceClient(conn)

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		c.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	var constReply *pb.Const

	if nodeName != "" {
		nodeReply, err := nodeClient.Name(ctx, &pb.Name{
			Name: nodeName,
		})
		if err != nil {
			c.root.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}

		constReply, err = constsClient.Name(ctx, &cores.ConstNameRequest{
			NodeId: nodeReply.Id,
			Name:   name,
		})
		if err != nil {
			c.root.logger.Error("failed to get const", zap.Error(err))
			os.Exit(1)
		}
	} else {
		constReply, err = constsClient.NameFull(ctx, &pb.Name{
			Name: name,
		})
		if err != nil {
			c.root.logger.Error("failed to get const", zap.Error(err))
			os.Exit(1)
		}
	}

	constValueReply, err := constsClient.GetValue(ctx, &pb.Id{
		Id: constReply.Id,
	})
	if err != nil {
		c.root.logger.Error("failed to get const value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", constReply.Name, constReply.Desc, constValueReply.Value)
}

func (c *Consts) setValueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-value [name] [value]",
		Short: "Set const value",
		Long:  "Set const value",
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(2)(cmd, args) != nil {
				return errors.New("must provide a pin name and value")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			c.setValue(ctx, c.root.GetConn(), cmd, args[0], args[1])
		},
	}

	cmd.Flags().StringP("node", "n", "", "node name")

	return cmd
}

func (c *Consts) setValue(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string, value string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	constsClient := cores.NewConstServiceClient(conn)

	nodeName, err := cmd.Flags().GetString("node")
	if err != nil {
		c.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	var constReply *pb.Const

	if nodeName != "" {
		nodeReply, err := nodeClient.Name(ctx, &pb.Name{
			Name: nodeName,
		})
		if err != nil {
			c.root.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}

		constReply, err = constsClient.Name(ctx, &cores.ConstNameRequest{
			NodeId: nodeReply.Id,
			Name:   name,
		})
		if err != nil {
			c.root.logger.Error("failed to get const", zap.Error(err))
			os.Exit(1)
		}
	} else {
		constReply, err = constsClient.NameFull(ctx, &pb.Name{
			Name: name,
		})
		if err != nil {
			c.root.logger.Error("failed to get const", zap.Error(err))
			os.Exit(1)
		}
	}

	_, err = constsClient.SetValue(ctx, &pb.ConstValue{
		Id:    constReply.Id,
		Value: value,
	})
	if err != nil {
		c.root.logger.Error("failed to set const value", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("name: %s, desc: %s, value: %s\n", constReply.Name, constReply.Desc, value)
}

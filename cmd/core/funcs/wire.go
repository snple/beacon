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

type Wire struct {
	root *Root
}

func wireCmd(root *Root) *cobra.Command {
	wire := &Wire{
		root: root,
	}

	wireCmd := &cobra.Command{
		Use:   "wire",
		Short: "Manage wires",
		Long:  `Manage wires`,
	}

	wireCmd.AddCommand(wire.wireListCmd())
	return wireCmd
}

func (w *Wire) wireListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List wires",
		Long:  `List wires`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			w.wireList(ctx, w.root.GetConn(), cmd, args[0])
		},
	}

	cmd.PersistentFlags().Int("limit", 10, "limit")
	cmd.PersistentFlags().Int("offset", 0, "offset")
	cmd.PersistentFlags().String("order_by", "", "order by")
	cmd.PersistentFlags().String("search", "", "search")
	cmd.PersistentFlags().String("tags", "", "tags")

	return cmd
}

func (w *Wire) wireList(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		w.root.logger.Error("failed to get limit", zap.Error(err))
		os.Exit(1)
	}

	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		w.root.logger.Error("failed to get offset", zap.Error(err))
		os.Exit(1)
	}

	orderBy, err := cmd.Flags().GetString("order_by")
	if err != nil {
		w.root.logger.Error("failed to get order by", zap.Error(err))
		os.Exit(1)
	}

	search, err := cmd.Flags().GetString("search")
	if err != nil {
		w.root.logger.Error("failed to get search", zap.Error(err))
		os.Exit(1)
	}

	tags, err := cmd.Flags().GetString("tags")
	if err != nil {
		w.root.logger.Error("failed to get tags", zap.Error(err))
		os.Exit(1)
	}

	page := pb.Page{
		Limit:   uint32(limit),
		Offset:  uint32(offset),
		OrderBy: orderBy,
		Search:  search,
	}

	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)

	nodeReply, err := nodeClient.Name(ctx, &pb.Name{
		Name: name,
	})
	if err != nil {
		w.root.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node: %s, %s\n", nodeReply.Id, nodeReply.Name)

	wireReply, err := wireClient.List(ctx, &cores.WireListRequest{
		Page:   &page,
		Tags:   tags,
		NodeId: nodeReply.Id,
	})
	if err != nil {
		w.root.logger.Error("failed to get wire list", zap.Error(err))
		os.Exit(1)
	}

	t := table.NewWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Desc", "Tags", "Source", "Status", "Created", "Updated"})

	timeformatFn := func(t int64) string {
		return time.UnixMicro(t).Format("2006-01-02 15:04:05")
	}

	for _, wire := range wireReply.Wires {
		t.AppendRow(table.Row{wire.Id, wire.Name, wire.Desc, wire.Tags, wire.Source, wire.Status, timeformatFn(wire.Created), timeformatFn(wire.Updated)})
	}

	// time to take a peek
	t.SetCaption(fmt.Sprintf("Total: %d/%d Rows.", len(wireReply.Wires), wireReply.Count))
	fmt.Println(t.Render())
}

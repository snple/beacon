package funcs

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func (r *Root) nodeCmd() *cobra.Command {
	nodeCmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes",
		Long:  `Manage nodes`,
	}

	nodeCmd.AddCommand(r.nodeListCmd())

	return nodeCmd
}

func (r *Root) nodeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List nodes",
		Long:  `List nodes`,
		Run: func(cmd *cobra.Command, args []string) {

			fmt.Printf("args: %v\n", args)

			ctx := context.Background()

			r.nodeList(ctx, cores.NewNodeServiceClient(r.GetConn()))
		},
	}
}

func (r *Root) nodeList(ctx context.Context, client cores.NodeServiceClient) {
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
		r.logger.Error("failed to list nodes", zap.Error(err))
		os.Exit(1)
	}

	t := table.NewWriter()

	t.SetAutoIndex(true)
	t.AppendHeader(table.Row{"ID", "Name", "Desc", "Tags", "Secret", "Status", "Created", "Updated"})

	for _, node := range reply.GetNode() {
		t.AppendRow(table.Row{node.Id, node.Name, node.Desc, node.Tags, node.Secret, node.Status, node.Created, node.Updated})
	}

	// time to take a peek
	t.SetCaption(fmt.Sprintf("Total: %d Rows.", len(reply.GetNode())))
	fmt.Println(t.Render())
}

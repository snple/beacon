package funcs

import (
	"context"
	"errors"
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
	// nodeCmd.AddCommand(r.nodeCreateCmd())
	// nodeCmd.AddCommand(r.nodeUpdateCmd())
	// nodeCmd.AddCommand(r.nodeDeleteCmd())
	nodeCmd.AddCommand(r.nodeExportCmd())

	return nodeCmd
}

func (r *Root) nodeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List nodes",
		Long:  `List nodes`,
		Run: func(cmd *cobra.Command, args []string) {

			fmt.Printf("args: %v\n", args)

			ctx := context.Background()

			r.nodeList(ctx, cores.NewNodeServiceClient(r.GetConn()), cmd)
		},
	}

	cmd.PersistentFlags().Int("limit", 10, "limit")
	cmd.PersistentFlags().Int("offset", 0, "offset")
	cmd.PersistentFlags().String("order_by", "", "order by")
	cmd.PersistentFlags().String("search", "", "search")
	cmd.PersistentFlags().String("tags", "", "tags")

	return cmd
}

func (r *Root) nodeList(ctx context.Context, client cores.NodeServiceClient, cmd *cobra.Command) {
	limit, err := cmd.Flags().GetInt("limit")
	if err != nil {
		r.logger.Error("failed to get limit", zap.Error(err))
		os.Exit(1)
	}

	offset, err := cmd.Flags().GetInt("offset")
	if err != nil {
		r.logger.Error("failed to get offset", zap.Error(err))
		os.Exit(1)
	}

	orderBy, err := cmd.Flags().GetString("order_by")
	if err != nil {
		r.logger.Error("failed to get order by", zap.Error(err))
		os.Exit(1)
	}

	search, err := cmd.Flags().GetString("search")
	if err != nil {
		r.logger.Error("failed to get search", zap.Error(err))
		os.Exit(1)
	}

	tags, err := cmd.Flags().GetString("tags")
	if err != nil {
		r.logger.Error("failed to get tags", zap.Error(err))
		os.Exit(1)
	}

	page := pb.Page{
		Limit:   uint32(limit),
		Offset:  uint32(offset),
		OrderBy: orderBy,
		Search:  search,
	}

	request := &cores.NodeListRequest{
		Page: &page,
		Tags: tags,
	}

	reply, err := client.List(ctx, request)
	if err != nil {
		r.logger.Error("failed to list nodes", zap.Error(err))
		os.Exit(1)
	}

	t := table.NewWriter()

	t.AppendHeader(table.Row{"ID", "Name", "Desc", "Tags", "Secret", "Status", "Created", "Updated"})

	for _, node := range reply.GetNode() {
		t.AppendRow(table.Row{node.Id, node.Name, node.Desc, node.Tags, node.Secret, node.Status, node.Created, node.Updated})
	}

	// time to take a peek
	t.SetCaption(fmt.Sprintf("Total: %d/%d Rows.", len(reply.GetNode()), reply.GetCount()))
	fmt.Println(t.Render())
}

func (r *Root) nodeCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a node",
		Long:  `Create a node`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeCreate(ctx, cores.NewNodeServiceClient(r.GetConn()), cmd)
		},
	}

	return cmd
}

func (r *Root) nodeCreate(ctx context.Context, client cores.NodeServiceClient, cmd *cobra.Command) {

}

func (r *Root) nodeUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a node",
		Long:  `Update a node`,
	}

	return cmd
}

func (r *Root) nodeUpdate(ctx context.Context, client cores.NodeServiceClient, cmd *cobra.Command) {

}

func (r *Root) nodeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a node",
		Long:  `Delete a node`,
	}

	return cmd
}

func (r *Root) nodeDelete(ctx context.Context, client cores.NodeServiceClient, cmd *cobra.Command) {

}

func (r *Root) nodeExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [id]",
		Short: "Export a node",
		Long:  `Export a node`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node id")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeExport(ctx, cores.NewNodeServiceClient(r.GetConn()), cmd, args[0])
		},
	}

	return cmd
}

func (r *Root) nodeExport(ctx context.Context, client cores.NodeServiceClient, cmd *cobra.Command, id string) {
	fmt.Println("nodeExport")

	request := &pb.Id{
		Id: id,
	}

	reply, err := client.View(ctx, request)
	if err != nil {
		r.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Println(reply)
}

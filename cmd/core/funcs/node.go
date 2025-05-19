package funcs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pelletier/go-toml/v2"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"google.golang.org/grpc"
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
	nodeCmd.AddCommand(r.nodeImportCmd())
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

	timeformatFn := func(t int64) string {
		return time.UnixMicro(t).Format("2006-01-02 15:04:05")
	}

	for _, node := range reply.GetNode() {
		t.AppendRow(table.Row{node.Id, node.Name, node.Desc, node.Tags, node.Secret, node.Status, timeformatFn(node.Created), timeformatFn(node.Updated)})
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

type ExportNode struct {
	Id     string       `toml:"id"`
	Name   string       `toml:"name"`
	Desc   string       `toml:"desc"`
	Tags   string       `toml:"tags"`
	Secret string       `toml:"secret"`
	Config string       `toml:"config"`
	Wires  []ExportWire `toml:"wires"`
}

type ExportWire struct {
	Id     string      `toml:"id"`
	Name   string      `toml:"name"`
	Desc   string      `toml:"desc"`
	Tags   string      `toml:"tags"`
	Source string      `toml:"source"`
	Params string      `toml:"params"`
	Config string      `toml:"config"`
	Pins   []ExportPin `toml:"pins"`
}

type ExportPin struct {
	Id       string `toml:"id"`
	Name     string `toml:"name"`
	Desc     string `toml:"desc"`
	Tags     string `toml:"tags"`
	DataType string `toml:"data_type"`
	Address  string `toml:"address"`
	Value    string `toml:"value"`
	Config   string `toml:"config"`
	Access   int32  `toml:"access"`
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

			r.nodeExport(ctx, r.GetConn(), cmd, args[0])
		},
	}

	cmd.PersistentFlags().StringP("output", "o", "", "output")

	return cmd
}

func (r *Root) nodeExport(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, id string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)
	// query node
	request := &pb.Id{
		Id: id,
	}

	reply, err := nodeClient.View(ctx, request)
	if err != nil {
		r.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	// query wires
	wireRequest := &cores.WireListRequest{
		NodeId: id,
	}

	wires, err := wireClient.List(ctx, wireRequest)
	if err != nil {
		r.logger.Error("failed to get wires", zap.Error(err))
		os.Exit(1)
	}

	exportNode := &ExportNode{
		Id:     reply.Id,
		Name:   reply.Name,
		Desc:   reply.Desc,
		Tags:   reply.Tags,
		Secret: reply.Secret,
		Config: reply.Config,
	}

	for _, wire := range wires.GetWire() {
		exportWire := &ExportWire{
			Id:     wire.Id,
			Name:   wire.Name,
			Desc:   wire.Desc,
			Tags:   wire.Tags,
			Source: wire.Source,
			Params: wire.Params,
			Config: wire.Config,
		}

		queryPins := &cores.PinListRequest{
			WireId: wire.Id,
		}

		pins, err := pinClient.List(ctx, queryPins)
		if err != nil {
			r.logger.Error("failed to get pins", zap.Error(err))
			os.Exit(1)
		}

		for _, pin := range pins.GetPin() {
			exportPin := ExportPin{
				Id:       pin.Id,
				Name:     pin.Name,
				Desc:     pin.Desc,
				Tags:     pin.Tags,
				DataType: pin.DataType,
				Address:  pin.Address,
				Value:    pin.Value,
				Config:   pin.Config,
				Access:   pin.Access,
			}

			exportWire.Pins = append(exportWire.Pins, exportPin)
		}

		exportNode.Wires = append(exportNode.Wires, *exportWire)
	}

	export, err := toml.Marshal(exportNode)
	if err != nil {
		r.logger.Error("failed to marshal export node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Println(string(export))

	output, err := cmd.Flags().GetString("output")
	if err != nil {
		r.logger.Error("failed to get output", zap.Error(err))
		os.Exit(1)
	}

	if output != "" {
		err = os.WriteFile(output, export, 0644)
		if err != nil {
			r.logger.Error("failed to write export node", zap.Error(err))
			os.Exit(1)
		}
	}
}

func (r *Root) nodeImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a node",
		Long:  `Import a node`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeImport(ctx, r.GetConn(), cmd)
		},
	}

	cmd.PersistentFlags().StringP("input", "i", "", "input")
	cmd.MarkPersistentFlagRequired("input")

	return cmd
}

func (r *Root) nodeImport(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command) {
	// nodeClient := cores.NewNodeServiceClient(conn)
	// wireClient := cores.NewWireServiceClient(conn)
	// pinClient := cores.NewPinServiceClient(conn)

	input, err := cmd.Flags().GetString("input")
	if err != nil {
		r.logger.Error("failed to get input", zap.Error(err))
		os.Exit(1)
	}

	content, err := os.ReadFile(input)
	if err != nil {
		r.logger.Error("failed to read input", zap.Error(err))
		os.Exit(1)
	}

	importNode := &ExportNode{}

	err = toml.Unmarshal(content, importNode)
	if err != nil {
		r.logger.Error("failed to unmarshal import node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Println(importNode)

}

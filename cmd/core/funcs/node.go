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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (r *Root) nodeCmd() *cobra.Command {
	nodeCmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes",
		Long:  `Manage nodes`,
	}

	nodeCmd.AddCommand(r.nodeListCmd())
	nodeCmd.AddCommand(r.nodeDeleteCmd())
	nodeCmd.AddCommand(r.nodeDeleteWireCmd())
	nodeCmd.AddCommand(r.nodeDeletePinCmd())
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

func (r *Root) nodeDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a node",
		Long:  `Delete a node`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeDelete(ctx, r.GetConn(), cmd, args[0])
		},
	}

	return cmd
}

func (r *Root) nodeDeleteWireCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-wire [name]",
		Short: "Delete a wire",
		Long:  `Delete a wire`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a wire name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeDeleteWire(ctx, r.GetConn(), cmd, args[0])
		},
	}

	return cmd
}

func (r *Root) nodeDeletePinCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-pin [name]",
		Short: "Delete a pin",
		Long:  `Delete a pin`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a pin name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeDeletePin(ctx, r.GetConn(), cmd, args[0])
		},
	}

	return cmd
}

func (r *Root) nodeDelete(ctx context.Context, conn *grpc.ClientConn, _ *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)

	request := &pb.Name{
		Name: name,
	}

	reply, err := nodeClient.Name(ctx, request)
	if err != nil {
		r.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node: %s, %s\n", reply.Id, reply.Name)

	{
		reply, err := wireClient.List(ctx, &cores.WireListRequest{
			NodeId: reply.Id,
		})
		if err != nil {
			r.logger.Error("failed to get wires", zap.Error(err))
			os.Exit(1)
		}

		for _, wire := range reply.GetWire() {
			fmt.Printf("wire: %s, %s\n", wire.Id, wire.Name)

			r.nodeDeleteWire2(ctx, conn, wire.Id)
		}
	}

	_, err = nodeClient.Delete(ctx, &pb.Id{Id: reply.Id})
	if err != nil {
		r.logger.Error("failed to delete node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node deleted: %s, %s\n", reply.Id, reply.Name)
}

func (r *Root) nodeDeleteWire(ctx context.Context, conn *grpc.ClientConn, _ *cobra.Command, name string) {
	wireClient := cores.NewWireServiceClient(conn)

	reply, err := wireClient.NameFull(ctx, &pb.Name{
		Name: name,
	})
	if err != nil {
		r.logger.Error("failed to get wire", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("wire: %s, %s\n", reply.Id, reply.Name)

	r.nodeDeleteWire2(ctx, conn, reply.Id)
}

func (r *Root) nodeDeletePin(ctx context.Context, conn *grpc.ClientConn, _ *cobra.Command, name string) {
	pinClient := cores.NewPinServiceClient(conn)

	reply, err := pinClient.NameFull(ctx, &pb.Name{
		Name: name,
	})
	if err != nil {
		r.logger.Error("failed to get pin", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("pin: %s, %s\n", reply.Id, reply.Name)

	r.nodeDeletePin2(ctx, conn, reply.Id)
}

func (r *Root) nodeDeleteWire2(ctx context.Context, conn *grpc.ClientConn, wireId string) {
	wireClient := cores.NewWireServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	{
		reply, err := pinClient.List(ctx, &cores.PinListRequest{
			WireId: wireId,
		})
		if err != nil {
			r.logger.Error("failed to get pins", zap.Error(err))
			os.Exit(1)
		}

		for _, pin := range reply.GetPin() {
			fmt.Printf("pin: %s, %s\n", pin.Id, pin.Name)

			r.nodeDeletePin2(ctx, conn, pin.Id)
		}
	}

	_, err := wireClient.Delete(ctx, &pb.Id{
		Id: wireId,
	})
	if err != nil {
		r.logger.Error("failed to delete wire", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("wire deleted: %s\n", wireId)
}

func (r *Root) nodeDeletePin2(ctx context.Context, conn *grpc.ClientConn, pinId string) {
	pinClient := cores.NewPinServiceClient(conn)

	_, err := pinClient.Delete(ctx, &pb.Id{
		Id: pinId,
	})
	if err != nil {
		r.logger.Error("failed to delete pin", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("pin deleted: %s\n", pinId)
}

type ExportNode struct {
	Name   string       `toml:"name"`
	Desc   string       `toml:"desc"`
	Tags   string       `toml:"tags"`
	Secret string       `toml:"secret"`
	Config string       `toml:"config"`
	Wires  []ExportWire `toml:"wires"`
}

type ExportWire struct {
	Name   string      `toml:"name"`
	Desc   string      `toml:"desc"`
	Tags   string      `toml:"tags"`
	Source string      `toml:"source"`
	Params string      `toml:"params"`
	Config string      `toml:"config"`
	Pins   []ExportPin `toml:"pins"`
}

type ExportPin struct {
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
		Use:   "export [name]",
		Short: "Export a node",
		Long:  `Export a node`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a node name")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeExport(ctx, r.GetConn(), cmd, args[0])
		},
	}

	cmd.PersistentFlags().StringP("output", "o", "", "output")
	cmd.PersistentFlags().Bool("ignore-id", false, "ignore id")

	return cmd
}

func (r *Root) nodeExport(ctx context.Context, conn *grpc.ClientConn, cmd *cobra.Command, name string) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	// query node
	request := &pb.Name{
		Name: name,
	}

	reply, err := nodeClient.Name(ctx, request)
	if err != nil {
		r.logger.Error("failed to get node", zap.Error(err))
		os.Exit(1)
	}

	// query wires
	wireRequest := &cores.WireListRequest{
		NodeId: reply.Id,
	}

	wires, err := wireClient.List(ctx, wireRequest)
	if err != nil {
		r.logger.Error("failed to get wires", zap.Error(err))
		os.Exit(1)
	}

	exportNode := &ExportNode{
		Name:   reply.Name,
		Desc:   reply.Desc,
		Tags:   reply.Tags,
		Secret: reply.Secret,
		Config: reply.Config,
	}

	for _, wire := range wires.GetWire() {
		exportWire := &ExportWire{
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
		Use:   "import [file]",
		Short: "Import a node",
		Long:  `Import a node`,
		Args: func(cmd *cobra.Command, args []string) error {
			if cobra.ExactArgs(1)(cmd, args) != nil {
				return errors.New("must provide a file")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			ctx := context.Background()

			r.nodeImport(ctx, r.GetConn(), cmd, args[0])
		},
	}

	return cmd
}

func (r *Root) nodeImport(ctx context.Context, conn *grpc.ClientConn, _ *cobra.Command, fileName string) {
	nodeClient := cores.NewNodeServiceClient(conn)

	content, err := os.ReadFile(fileName)
	if err != nil {
		r.logger.Error("failed to read file", zap.Error(err))
		os.Exit(1)
	}

	importNode := &ExportNode{}

	err = toml.Unmarshal(content, importNode)
	if err != nil {
		r.logger.Error("failed to unmarshal import node", zap.Error(err))
		os.Exit(1)
	}

	request := &pb.Name{
		Name: importNode.Name,
	}

	reply, err := nodeClient.Name(ctx, request)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			r.nodeImportCreateNode(ctx, conn, importNode)
		} else {
			r.logger.Error("failed to get node", zap.Error(err))
			os.Exit(1)
		}
	} else {
		r.nodeImportUpdateNode(ctx, conn, reply, importNode)
	}
}

func (r *Root) nodeImportCreateNode(ctx context.Context, conn *grpc.ClientConn, importNode *ExportNode) {
	nodeClient := cores.NewNodeServiceClient(conn)

	node := &pb.Node{
		Name:   importNode.Name,
		Desc:   importNode.Desc,
		Tags:   importNode.Tags,
		Secret: importNode.Secret,
		Config: importNode.Config,
	}

	reply, err := nodeClient.Create(ctx, node)
	if err != nil {
		r.logger.Error("failed to create node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node created: %s, %s\n", reply.Id, reply.Name)

	for _, wire := range importNode.Wires {
		r.nodeImportCreateWire(ctx, conn, reply.Id, &wire)
	}
}

func (r *Root) nodeImportUpdateNode(ctx context.Context, conn *grpc.ClientConn, node *pb.Node, importNode *ExportNode) {
	nodeClient := cores.NewNodeServiceClient(conn)
	wireClient := cores.NewWireServiceClient(conn)

	node.Name = importNode.Name
	node.Desc = importNode.Desc
	node.Tags = importNode.Tags
	node.Secret = importNode.Secret
	node.Config = importNode.Config

	reply, err := nodeClient.Update(ctx, node)
	if err != nil {
		r.logger.Error("failed to update node", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("node updated: %s, %s\n", reply.Id, reply.Name)

	for _, wire := range importNode.Wires {
		request := &cores.WireNameRequest{
			NodeId: node.Id,
			Name:   wire.Name,
		}

		reply, err := wireClient.Name(ctx, request)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				r.nodeImportCreateWire(ctx, conn, node.Id, &wire)
			} else {
				r.logger.Error("failed to get wire", zap.Error(err))
				os.Exit(1)
			}
		} else {
			r.nodeImportUpdateWire(ctx, conn, reply, &wire)
		}
	}
}

func (r *Root) nodeImportCreateWire(ctx context.Context, conn *grpc.ClientConn, nodeId string, importWire *ExportWire) {
	wireClient := cores.NewWireServiceClient(conn)

	wire := &pb.Wire{
		NodeId: nodeId,
		Name:   importWire.Name,
		Desc:   importWire.Desc,
		Tags:   importWire.Tags,
		Source: importWire.Source,
		Params: importWire.Params,
		Config: importWire.Config,
	}

	reply, err := wireClient.Create(ctx, wire)
	if err != nil {
		r.logger.Error("failed to create wire", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("wire created: %s, %s\n", reply.Id, reply.Name)

	for _, pin := range importWire.Pins {
		r.nodeImportCreatePin(ctx, conn, nodeId, reply.Id, &pin)
	}
}

func (r *Root) nodeImportUpdateWire(ctx context.Context, conn *grpc.ClientConn, wire *pb.Wire, importWire *ExportWire) {
	wireClient := cores.NewWireServiceClient(conn)
	pinClient := cores.NewPinServiceClient(conn)

	wire.Name = importWire.Name
	wire.Desc = importWire.Desc
	wire.Tags = importWire.Tags
	wire.Source = importWire.Source
	wire.Params = importWire.Params
	wire.Config = importWire.Config

	reply, err := wireClient.Update(ctx, wire)
	if err != nil {
		r.logger.Error("failed to update wire", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("wire updated: %s, %s\n", reply.Id, reply.Name)

	for _, pin := range importWire.Pins {
		request := &cores.PinNameRequest{
			NodeId: wire.NodeId,
			Name:   fmt.Sprintf("%s.%s", wire.Name, pin.Name),
		}

		reply, err := pinClient.Name(ctx, request)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				r.nodeImportCreatePin(ctx, conn, wire.NodeId, wire.Id, &pin)
			} else {
				r.logger.Error("failed to get pin", zap.Error(err))
				os.Exit(1)
			}
		} else {
			r.nodeImportUpdatePin(ctx, conn, reply, &pin)
		}
	}
}

func (r *Root) nodeImportCreatePin(ctx context.Context, conn *grpc.ClientConn, nodeId string, wireId string, importPin *ExportPin) {
	pinClient := cores.NewPinServiceClient(conn)

	pin := &pb.Pin{
		NodeId:   nodeId,
		WireId:   wireId,
		Name:     importPin.Name,
		Desc:     importPin.Desc,
		Tags:     importPin.Tags,
		DataType: importPin.DataType,
		Address:  importPin.Address,
		Value:    importPin.Value,
		Config:   importPin.Config,
		Access:   importPin.Access,
	}

	reply, err := pinClient.Create(ctx, pin)
	if err != nil {
		r.logger.Error("failed to create pin", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("pin created: %s, %s\n", reply.Id, reply.Name)
}

func (r *Root) nodeImportUpdatePin(ctx context.Context, conn *grpc.ClientConn, pin *pb.Pin, importPin *ExportPin) {
	pinClient := cores.NewPinServiceClient(conn)

	pin.Name = importPin.Name
	pin.Desc = importPin.Desc
	pin.Tags = importPin.Tags
	pin.DataType = importPin.DataType
	pin.Address = importPin.Address
	pin.Value = importPin.Value
	pin.Config = importPin.Config
	pin.Access = importPin.Access

	reply, err := pinClient.Update(ctx, pin)
	if err != nil {
		r.logger.Error("failed to update pin", zap.Error(err))
		os.Exit(1)
	}

	fmt.Printf("pin updated: %s, %s\n", reply.Id, reply.Name)
}

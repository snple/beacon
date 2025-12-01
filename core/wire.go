package core

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/uptrace/bun"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WireService struct {
	cs *CoreService

	cores.UnimplementedWireServiceServer
}

func newWireService(cs *CoreService) *WireService {
	return &WireService{
		cs: cs,
	}
}

func (s *WireService) View(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
	var output pb.Wire
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *WireService) Name(ctx context.Context, in *cores.WireNameRequest) (*pb.Wire, error) {
	var output pb.Wire
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	item, err := s.ViewByNodeIDAndName(ctx, in.NodeId, in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *WireService) NameFull(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire
	// var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	nodeName := consts.DEFAULT_NODE
	itemName := in.Name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}

		nodeName = splits[0]
		itemName = splits[1]
	}

	node, err := s.cs.GetNode().ViewByName(ctx, nodeName)
	if err != nil {
		return &output, err
	}

	item, err := s.ViewByNodeIDAndName(ctx, node.ID, itemName)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Name = in.Name

	return &output, nil
}

func (s *WireService) List(ctx context.Context, in *cores.WireListRequest) (*cores.WireListResponse, error) {
	var err error
	var output cores.WireListResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	defaultPage := pb.Page{
		Limit:  10,
		Offset: 0,
	}

	if in.GetPage() == nil {
		in.Page = &defaultPage
	}

	output.Page = in.GetPage()

	var items []model.Wire

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.NodeId != "" {
		query.Where("node_id = ?", in.NodeId)
	}

	if in.GetPage().GetSearch() != "" {
		search := fmt.Sprintf("%%%v%%", in.GetPage().GetSearch())

		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			q = q.Where(`"name" LIKE ?`, search).
				WhereOr(`"desc" LIKE ?`, search)

			return q
		})
	}

	if in.Tags != "" {
		tagsSplit := strings.Split(in.Tags, ",")

		if len(tagsSplit) == 1 {
			search := fmt.Sprintf("%%%v%%", tagsSplit[0])

			query = query.Where(`"tags" LIKE ?`, search)
		} else {
			query = query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
				for i := 0; i < len(tagsSplit); i++ {
					search := fmt.Sprintf("%%%v%%", tagsSplit[i])

					q = q.WhereOr(`"tags" LIKE ?`, search)
				}

				return q
			})
		}
	}

	if in.Source != "" {
		query = query.Where(`source = ?`, in.Source)
	}

	if in.GetPage().GetOrderBy() != "" && (in.GetPage().GetOrderBy() == "id" || in.GetPage().GetOrderBy() == "name" ||
		in.GetPage().GetOrderBy() == "created" || in.GetPage().GetOrderBy() == "updated") {
		query.Order(in.GetPage().GetOrderBy() + " " + in.GetPage().GetSort().String())
	} else {
		query.Order("id ASC")
	}

	count, err := query.Offset(int(in.GetPage().GetOffset())).Limit(int(in.GetPage().GetLimit())).ScanAndCount(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	output.Count = uint32(count)

	for i := range items {
		item := pb.Wire{}

		s.copyModelToOutput(&item, &items[i])

		output.Wires = append(output.Wires, &item)
	}

	return &output, nil
}

func (s *WireService) ViewByID(ctx context.Context, id string) (model.Wire, error) {
	item := model.Wire{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Wire.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *WireService) ViewByNodeIDAndName(ctx context.Context, nodeID, name string) (model.Wire, error) {
	item := model.Wire{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("node_id = ?", nodeID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, NodeID: %v, Wire.Name: %v", err, nodeID, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *WireService) copyModelToOutput(output *pb.Wire, item *model.Wire) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.Name = item.Name
	output.Clusters = item.Clusters
	output.Updated = item.Updated.UnixMicro()
}

// Push 接收 stream 数据并插入 Wire
func (s *WireService) Push(stream grpc.ClientStreamingServer[pb.Wire, pb.MyBool]) error {
	ctx := stream.Context()

	for {
		in, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return stream.SendAndClose(&pb.MyBool{Bool: true})
			}
			return err
		}

		// basic validation
		{
			if in.Id == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
			}

			if in.NodeId == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Wire.NodeId")
			}

			if in.Name == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
			}
		}

		// node validation
		{
			_, err = s.cs.GetNode().ViewByID(ctx, in.NodeId)
			if err != nil {
				return err
			}
		}

		item := model.Wire{
			ID:       in.Id,
			NodeID:   in.NodeId,
			Name:     in.Name,
			Clusters: in.Clusters,
			Updated:  time.UnixMicro(in.Updated),
		}

		_, err = s.cs.GetDB().NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}

		if err = s.afterUpdate(ctx, &item); err != nil {
			return err
		}
	}
}

func (s *WireService) afterUpdate(ctx context.Context, item *model.Wire) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	return nil
}

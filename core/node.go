package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/util"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	cs *CoreService

	cores.UnimplementedNodeServiceServer
}

func newNodeService(cs *CoreService) *NodeService {
	s := &NodeService{
		cs: cs,
	}

	return s
}

func (s *NodeService) Create(ctx context.Context, in *pb.Node) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Name")
		}
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Node.Name min 2 character")
		}

		err = s.cs.GetDB().NewSelect().Model(&model.Node{}).Where("name = ?", in.Name).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Node.Name must be unique")
		}
	}

	item := model.Node{
		ID:      in.Id,
		Name:    in.Name,
		Secret:  in.Secret,
		Status:  in.Status,
		Updated: time.Now(),
	}

	if item.ID == "" {
		item.ID = util.RandomID()
	}

	_, err = s.cs.GetDB().NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Insert: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) Update(ctx context.Context, in *pb.Node) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Name")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Node.Name min 2 character")
		}

		modelItem := model.Node{}
		err = s.cs.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			if modelItem.ID != item.ID {
				return &output, status.Error(codes.AlreadyExists, "Node.Name must be unique")
			}
		}
	}

	item.Name = in.Name
	item.Secret = in.Secret
	item.Status = in.Status
	item.Updated = time.Now()

	_, err = s.cs.GetDB().NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	item.Updated = time.Now()
	item.Deleted = time.Now()

	_, err = s.cs.GetDB().NewUpdate().Model(&item).Column("updated", "deleted").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Delete: %v", err)
	}

	if err = s.afterDelete(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *NodeService) afterUpdate(ctx context.Context, item *model.Node) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.ID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	return nil
}

func (s *NodeService) afterDelete(ctx context.Context, item *model.Node) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.ID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	return nil
}

func (s *NodeService) View(ctx context.Context, in *pb.Id) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) Name(ctx context.Context, in *pb.Name) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Name")
		}
	}

	item, err := s.ViewByName(ctx, in.Name)
	if err != nil {
		return nil, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) List(ctx context.Context, in *cores.NodeListRequest) (*cores.NodeListResponse, error) {
	var err error
	var output cores.NodeListResponse

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

	var items []model.Node

	query := s.cs.GetDB().NewSelect().Model(&items)

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
		item := pb.Node{}

		s.copyModelToOutput(&item, &items[i])

		output.Nodes = append(output.Nodes, &item)
	}

	return &output, nil
}

// Push 清除 Node 下所有配置数据，为接收新的 Push 数据做准备
func (s *NodeService) Push(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.ID")
		}
	}

	// 验证 Node 存在
	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	// 清除 Node 下所有配置数据（Wire, Pin）但保留运行数据（PinValue, PinWrite）
	err = func() error {
		models := []any{
			(*model.Pin)(nil),
			(*model.Wire)(nil),
		}

		tx, err := s.cs.GetDB().BeginTx(ctx, nil)
		if err != nil {
			return status.Errorf(codes.Internal, "BeginTx: %v", err)
		}
		var done bool
		defer func() {
			if !done {
				_ = tx.Rollback()
			}
		}()

		for _, model := range models {
			_, err = tx.NewDelete().Model(model).Where("node_id = ?", item.ID).Exec(ctx)
			if err != nil {
				return status.Errorf(codes.Internal, "Delete: %v", err)
			}
		}

		done = true
		err = tx.Commit()
		if err != nil {
			return status.Errorf(codes.Internal, "Commit: %v", err)
		}

		return nil
	}()

	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *NodeService) Destory(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	err = func() error {
		models := []any{
			(*model.Wire)(nil),
			(*model.Pin)(nil),
			(*model.PinValue)(nil),
			(*model.PinWrite)(nil),
		}

		tx, err := s.cs.GetDB().BeginTx(ctx, nil)
		if err != nil {
			return status.Errorf(codes.Internal, "BeginTx: %v", err)
		}
		var done bool
		defer func() {
			if !done {
				_ = tx.Rollback()
			}
		}()

		for _, model := range models {
			_, err = tx.NewDelete().Model(model).Where("node_id = ?", item.ID).ForceDelete().Exec(ctx)
			if err != nil {
				return status.Errorf(codes.Internal, "Delete: %v", err)
			}
		}

		s.cs.GetSync().destory(ctx, tx, item.ID)
		if err != nil {
			return status.Errorf(codes.Internal, "Delete: %v", err)
		}

		done = true
		err = tx.Commit()
		if err != nil {
			return status.Errorf(codes.Internal, "Commit: %v", err)
		}

		return nil
	}()

	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *NodeService) ViewByID(ctx context.Context, id string) (model.Node, error) {
	item := model.Node{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Node.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *NodeService) ViewByName(ctx context.Context, name string) (model.Node, error) {
	item := model.Node{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Node.Name: %v", err, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *NodeService) copyModelToOutput(output *pb.Node, item *model.Node) {
	output.Id = item.ID
	output.Name = item.Name
	output.Secret = item.Secret
	output.Status = item.Status
	output.Updated = item.Updated.UnixMicro()
}

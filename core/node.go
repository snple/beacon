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
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	cs *CoreService

	cache *cache.Cache[string, model.Node]

	cores.UnimplementedNodeServiceServer
}

func newNodeService(cs *CoreService) *NodeService {
	s := &NodeService{
		cs:    cs,
		cache: cache.NewCache[string, model.Node](nil),
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
		Created: time.Now(),
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

func (s *NodeService) Clone(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var output pb.MyBool
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

	tx, err := s.cs.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BeginTx: %v", err)
	}
	var done bool
	defer func() {
		if !done {
			_ = tx.Rollback()
		}
	}()

	err = s.cs.getClone().node(ctx, tx, in.Id)
	if err != nil {
		return &output, err
	}

	done = true
	err = tx.Commit()
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Commit: %v", err)
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
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Deleted.UnixMicro()
}

func (s *NodeService) afterUpdate(ctx context.Context, item *model.Node) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.ID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_NODE, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

func (s *NodeService) afterDelete(ctx context.Context, item *model.Node) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.ID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_NODE, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

// sync

func (s *NodeService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Node, error) {
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

	item, err := s.viewWithDeleted(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) viewWithDeleted(ctx context.Context, id string) (model.Node, error) {
	item := model.Node{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Node.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *NodeService) Pull(ctx context.Context, in *cores.NodePullRequest) (*cores.NodePullResponse, error) {
	var err error
	var output cores.NodePullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.After
	output.Limit = in.Limit

	var items []model.Node

	query := s.cs.GetDB().NewSelect().Model(&items)

	err = query.Where("updated > ?", time.UnixMicro(in.After)).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.Limit)).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
		item := pb.Node{}

		s.copyModelToOutput(&item, &items[i])

		output.Nodes = append(output.Nodes, &item)
	}

	return &output, nil
}

func (s *NodeService) Sync(ctx context.Context, in *pb.Node) (*pb.MyBool, error) {
	var output pb.MyBool
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

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Updated")
		}
	}

	insert := false
	update := false

	item, err := s.viewWithDeleted(ctx, in.Id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				insert = true
				goto SKIP
			}
		}

		return &output, err
	}

	update = true

SKIP:

	//	insert
	if insert {
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
			Created: time.UnixMicro(in.Created),
			Updated: time.UnixMicro(in.Updated),
			Deleted: time.UnixMicro(in.Deleted),
		}

		_, err = s.cs.GetDB().NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	// update
	if update {
		if in.Updated <= item.Updated.UnixMicro() {
			return &output, nil
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
		item.Updated = time.UnixMicro(in.Updated)
		item.Deleted = time.UnixMicro(in.Deleted)

		_, err = s.cs.GetDB().NewUpdate().Model(&item).WherePK().WhereAllWithDeleted().Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Update: %v", err)
		}
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

// cache

func (s *NodeService) GC() {
	s.cache.GC()
}

func (s *NodeService) ViewFromCacheByID(ctx context.Context, id string) (model.Node, error) {
	if !s.cs.dopts.cache {
		return s.ViewByID(ctx, id)
	}

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByID(ctx, id)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.cs.dopts.cacheTTL)

	return item, nil
}

func (s *NodeService) ViewFromCacheByName(ctx context.Context, name string) (model.Node, error) {
	if !s.cs.dopts.cache {
		return s.ViewByName(ctx, name)
	}

	if option := s.cache.Get(name); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByName(ctx, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(name, item, s.cs.dopts.cacheTTL)

	return item, nil
}

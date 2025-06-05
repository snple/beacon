package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/util"
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConstService struct {
	cs *CoreService

	cache *cache.Cache[model.Const]

	cores.UnimplementedConstServiceServer
}

func newConstService(cs *CoreService) *ConstService {
	return &ConstService{
		cs:    cs,
		cache: cache.NewCache[model.Const](nil),
	}
}

func (s *ConstService) Create(ctx context.Context, in *pb.Const) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Type")
		}

		if !dt.ValidateValue(in.GetValue(), in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	// node validation
	{
		_, err = s.cs.GetNode().ViewByID(ctx, in.GetNodeId())
		if err != nil {
			return &output, err
		}
	}

	// name validation
	{
		if len(in.GetName()) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
		}

		err = s.cs.GetDB().NewSelect().Model(&model.Const{}).Where("node_id = ?", in.GetNodeId()).Where("name = ?", in.GetName()).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
		}
	}

	item := model.Const{
		ID:      in.GetId(),
		NodeID:  in.GetNodeId(),
		Name:    in.GetName(),
		Desc:    in.GetDesc(),
		Tags:    in.GetTags(),
		Type:    in.GetType(),
		Value:   in.GetValue(),
		Config:  in.GetConfig(),
		Status:  in.GetStatus(),
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

func (s *ConstService) Update(ctx context.Context, in *pb.Const) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Type")
		}

		if !dt.ValidateValue(in.GetValue(), in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	// name validation
	{
		if len(in.GetName()) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
		}

		modelItem := model.Const{}
		err = s.cs.GetDB().NewSelect().Model(&modelItem).Where("node_id = ?", item.NodeID).Where("name = ?", in.GetName()).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			if modelItem.ID != item.ID {
				return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
			}
		}
	}

	item.Name = in.GetName()
	item.Desc = in.GetDesc()
	item.Tags = in.GetTags()
	item.Type = in.GetType()
	item.Value = in.GetValue()
	item.Config = in.GetConfig()
	item.Status = in.GetStatus()
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

func (s *ConstService) View(ctx context.Context, in *pb.Id) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *ConstService) Name(ctx context.Context, in *cores.ConstNameRequest) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}
	}

	item, err := s.ViewByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *ConstService) NameFull(ctx context.Context, in *pb.Name) (*pb.Const, error) {
	var output pb.Const
	// var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}
	}

	nodeName := consts.DEFAULT_NODE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
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

	output.Name = in.GetName()

	return &output, nil
}

func (s *ConstService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.GetId())
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

func (s *ConstService) List(ctx context.Context, in *cores.ConstListRequest) (*cores.ConstListResponse, error) {
	var err error
	var output cores.ConstListResponse

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

	items := make([]model.Const, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	if in.GetPage().GetSearch() != "" {
		search := fmt.Sprintf("%%%v%%", in.GetPage().GetSearch())

		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			q = q.Where(`"name" LIKE ?`, search).
				WhereOr(`"desc" LIKE ?`, search)

			return q
		})
	}

	if in.GetTags() != "" {
		tagsSplit := strings.Split(in.GetTags(), ",")

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
		item := pb.Const{}

		s.copyModelToOutput(&item, &items[i])

		output.Const = append(output.Const, &item)
	}

	return &output, nil
}

func (s *ConstService) Clone(ctx context.Context, in *cores.ConstCloneRequest) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	err = s.cs.getClone().const_(ctx, s.cs.GetDB(), in.GetId(), in.GetNodeId())
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *ConstService) ViewByID(ctx context.Context, id string) (model.Const, error) {
	item := model.Const{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Const.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) ViewByNodeIDAndName(ctx context.Context, nodeID, name string) (model.Const, error) {
	item := model.Const{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("node_id = ?", nodeID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, NodeID: %v, Const.Name: %v", err, nodeID, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) copyModelToOutput(output *pb.Const, item *model.Const) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.Name = item.Name
	output.Desc = item.Desc
	output.Tags = item.Tags
	output.Type = item.Type
	output.Value = item.Value
	output.Config = item.Config
	output.Status = item.Status
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Deleted.UnixMicro()
}

func (s *ConstService) afterUpdate(ctx context.Context, item *model.Const) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_CONST, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

func (s *ConstService) afterDelete(ctx context.Context, item *model.Const) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_CONST, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

// sync

func (s *ConstService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.viewWithDeleted(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *ConstService) viewWithDeleted(ctx context.Context, id string) (model.Const, error) {
	item := model.Const{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Const.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) Pull(ctx context.Context, in *cores.ConstPullRequest) (*cores.ConstPullResponse, error) {
	var err error
	var output cores.ConstPullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.Const, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	err = query.Where("updated > ?", time.UnixMicro(in.GetAfter())).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.GetLimit())).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
		item := pb.Const{}

		s.copyModelToOutput(&item, &items[i])

		output.Const = append(output.Const, &item)
	}

	return &output, nil
}

func (s *ConstService) Sync(ctx context.Context, in *pb.Const) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Type")
		}

		if !dt.ValidateValue(in.GetValue(), in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Updated")
		}
	}

	insert := false
	update := false

	item, err := s.viewWithDeleted(ctx, in.GetId())
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

	// insert
	if insert {
		// node validation
		{
			_, err = s.cs.GetNode().viewWithDeleted(ctx, in.GetNodeId())
			if err != nil {
				return &output, err
			}
		}

		// name validation
		{
			if len(in.GetName()) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
			}

			err = s.cs.GetDB().NewSelect().Model(&model.Const{}).Where("node_id = ?", in.GetNodeId()).Where("name = ?", in.GetName()).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
			}
		}

		item := model.Const{
			ID:      in.GetId(),
			NodeID:  in.GetNodeId(),
			Name:    in.GetName(),
			Desc:    in.GetDesc(),
			Tags:    in.GetTags(),
			Type:    in.GetType(),
			Value:   in.GetValue(),
			Config:  in.GetConfig(),
			Status:  in.GetStatus(),
			Created: time.UnixMicro(in.GetCreated()),
			Updated: time.UnixMicro(in.GetUpdated()),
			Deleted: time.UnixMicro(in.GetDeleted()),
		}

		_, err = s.cs.GetDB().NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	// update
	if update {
		if in.GetNodeId() != item.NodeID {
			return &output, status.Error(codes.NotFound, "Query: in.GetNodeId() != item.NodeID")
		}

		if in.GetUpdated() <= item.Updated.UnixMicro() {
			return &output, nil
		}

		// name validation
		{
			if len(in.GetName()) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
			}

			modelItem := model.Const{}
			err = s.cs.GetDB().NewSelect().Model(&modelItem).Where("node_id = ?", item.NodeID).Where("name = ?", in.GetName()).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				if modelItem.ID != item.ID {
					return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
				}
			}
		}

		item.Name = in.GetName()
		item.Desc = in.GetDesc()
		item.Tags = in.GetTags()
		item.Type = in.GetType()
		item.Value = in.GetValue()
		item.Config = in.GetConfig()
		item.Status = in.GetStatus()
		item.Updated = time.UnixMicro(in.GetUpdated())
		item.Deleted = time.UnixMicro(in.GetDeleted())

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

func (s *ConstService) GC() {
	s.cache.GC()
}

func (s *ConstService) ViewFromCacheByID(ctx context.Context, id string) (model.Const, error) {
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

func (s *ConstService) ViewFromCacheByNodeIDAndName(ctx context.Context, nodeID, name string) (model.Const, error) {
	if !s.cs.dopts.cache {
		return s.ViewByNodeIDAndName(ctx, nodeID, name)
	}

	id := nodeID + name

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByNodeIDAndName(ctx, nodeID, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.cs.dopts.cacheTTL)

	return item, nil
}

// value

func (s *ConstService) GetValue(ctx context.Context, in *pb.Id) (*pb.ConstValue, error) {
	var err error
	var output pb.ConstValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.ViewFromCacheByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	output.Id = in.GetId()
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()

	return &output, nil
}

func (s *ConstService) SetValue(ctx context.Context, in *pb.ConstValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.ViewFromCacheByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Const.Status != ON")
	}

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Const.Value")
	}

	item.Value = in.GetValue()
	item.Updated = time.Now()

	_, err = s.cs.GetDB().NewUpdate().Model(&item).Column("value", "updated").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *ConstService) GetValueByName(ctx context.Context, in *cores.ConstGetValueByNameRequest) (*cores.ConstNameValue, error) {
	var err error
	var output cores.ConstNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.ID")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	output.NodeId = in.GetNodeId()
	output.Id = item.ID
	output.Name = in.GetName()
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()

	return &output, nil
}

func (s *ConstService) SetValueByName(ctx context.Context, in *cores.ConstNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Const.Status != ON")
	}

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Const.Value")
	}

	item.Value = in.GetValue()
	item.Updated = time.Now()

	_, err = s.cs.GetDB().NewUpdate().Model(&item).Column("value", "updated").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

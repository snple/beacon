package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/beacon/util"
	"github.com/snple/beacon/util/datatype"
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	cs *CoreService

	cache *cache.Cache[model.Pin]

	cores.UnimplementedPinServiceServer
}

func newPinService(cs *CoreService) *PinService {
	return &PinService{
		cs:    cs,
		cache: cache.NewCache[model.Pin](nil),
	}
}

func (s *PinService) Create(ctx context.Context, in *pb.Pin) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if len(in.GetSourceId()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.SourceID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	// name validation
	{
		if len(in.GetName()) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
		}

		err = s.cs.GetDB().NewSelect().Model(&model.Pin{}).Where("name = ?", in.GetName()).Where("source_id = ?", in.GetSourceId()).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
		}
	}

	item := model.Pin{
		ID:       in.GetId(),
		SourceID: in.GetSourceId(),
		Name:     in.GetName(),
		Desc:     in.GetDesc(),
		Tags:     in.GetTags(),
		DataType: in.GetDataType(),
		Address:  in.GetAddress(),
		Config:   in.GetConfig(),
		Status:   in.GetStatus(),
		Access:   in.GetAccess(),
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	// source validation
	{
		source, err := s.cs.GetSource().ViewByID(ctx, in.GetSourceId())
		if err != nil {
			return &output, err
		}

		item.NodeID = source.NodeID
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

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

	return &output, nil
}

func (s *PinService) Update(ctx context.Context, in *pb.Pin) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	// name validation
	{
		if len(in.GetName()) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
		}

		modelItem := model.Pin{}
		err = s.cs.GetDB().NewSelect().Model(&modelItem).Where("source_id = ?", item.SourceID).Where("name = ?", in.GetName()).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			if modelItem.ID != item.ID {
				return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
			}
		}
	}

	item.Name = in.GetName()
	item.Desc = in.GetDesc()
	item.Tags = in.GetTags()
	item.DataType = in.GetDataType()
	item.Address = in.GetAddress()
	item.Config = in.GetConfig()
	item.Status = in.GetStatus()
	item.Access = in.GetAccess()
	item.Updated = time.Now()

	_, err = s.cs.GetDB().NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

	return &output, nil
}

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

	return &output, nil
}

func (s *PinService) Name(ctx context.Context, in *cores.PinNameRequest) (*pb.Pin, error) {
	var output pb.Pin
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
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Name = in.GetName()

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

	return &output, nil
}

func (s *PinService) NameFull(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	nodeName := consts.DEFAULT_NODE
	sourceName := consts.DEFAULT_SOURCE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")

		switch len(splits) {
		case 2:
			sourceName = splits[0]
			itemName = splits[1]
		case 3:
			nodeName = splits[0]
			sourceName = splits[1]
			itemName = splits[2]
		default:
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	node, err := s.cs.GetNode().ViewByName(ctx, nodeName)
	if err != nil {
		return &output, err
	}

	source, err := s.cs.GetSource().ViewByNodeIDAndName(ctx, node.ID, sourceName)
	if err != nil {
		return &output, err
	}

	item, err := s.ViewBySourceIDAndName(ctx, source.ID, itemName)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Name = in.GetName()

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

	return &output, nil
}

func (s *PinService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
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

func (s *PinService) List(ctx context.Context, in *cores.PinListRequest) (*cores.PinListResponse, error) {
	var err error
	var output cores.PinListResponse

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

	items := make([]model.Pin, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	if len(in.GetSourceId()) > 0 {
		query.Where("source_id = ?", in.GetSourceId())
	}

	if in.GetPage().GetSearch() != "" {
		search := fmt.Sprintf("%%%v%%", in.GetPage().GetSearch())

		query.WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			q = q.Where(`"name" LIKE ?`, search).
				WhereOr(`"desc" LIKE ?`, search).
				WhereOr(`"address" LIKE ?`, search)

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

	for i := 0; i < len(items); i++ {
		item := pb.Pin{}

		s.copyModelToOutput(&item, &items[i])

		item.Value, err = s.getPinValue(ctx, items[i].ID)
		if err != nil {
			return &output, err
		}

		output.Pin = append(output.Pin, &item)
	}

	return &output, nil
}

func (s *PinService) Clone(ctx context.Context, in *cores.PinCloneRequest) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	err = s.cs.getClone().pin(ctx, s.cs.GetDB(), in.GetId(), in.GetSourceId())
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) ViewByID(ctx context.Context, id string) (model.Pin, error) {
	item := model.Pin{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) ViewByNodeIDAndName(ctx context.Context, nodeID, name string) (model.Pin, error) {
	item := model.Pin{}

	sourceName := consts.DEFAULT_SOURCE
	itemName := name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return item, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		sourceName = splits[0]
		itemName = splits[1]
	}

	source, err := s.cs.GetSource().ViewByNodeIDAndName(ctx, nodeID, sourceName)
	if err != nil {
		return item, err
	}

	return s.ViewBySourceIDAndName(ctx, source.ID, itemName)
}

func (s *PinService) ViewBySourceIDAndName(ctx context.Context, sourceID, name string) (model.Pin, error) {
	item := model.Pin{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("source_id = ?", sourceID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, SourceID: %v, Pin.Name: %v", err, sourceID, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) ViewBySourceIDAndAddress(ctx context.Context, sourceID, address string) (model.Pin, error) {
	item := model.Pin{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("source_id = ?", sourceID).Where("address = ?", address).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, SourceID: %v, Pin.Address: %v", err, sourceID, address)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutput(output *pb.Pin, item *model.Pin) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.SourceId = item.SourceID
	output.Name = item.Name
	output.Desc = item.Desc
	output.Tags = item.Tags
	output.DataType = item.DataType
	output.Address = item.Address
	output.Config = item.Config
	output.Status = item.Status
	output.Access = item.Access
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Deleted.UnixMicro()
}

func (s *PinService) afterUpdate(ctx context.Context, item *model.Pin) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_PIN, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

func (s *PinService) afterDelete(ctx context.Context, item *model.Pin) error {
	var err error

	err = s.cs.GetSync().setNodeUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.cs.GetSyncGlobal().setUpdated(ctx, s.cs.GetDB(), model.SYNC_GLOBAL_PIN, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "SyncGlobal.setUpdated: %v", err)
	}

	return nil
}

// sync

func (s *PinService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.viewWithDeleted(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *PinService) viewWithDeleted(ctx context.Context, id string) (model.Pin, error) {
	item := model.Pin{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) Pull(ctx context.Context, in *cores.PinPullRequest) (*cores.PinPullResponse, error) {
	var err error
	var output cores.PinPullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.Pin, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	if in.GetSourceId() != "" {
		query.Where("source_id = ?", in.GetSourceId())
	}

	err = query.Where("updated > ?", time.UnixMicro(in.GetAfter())).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.GetLimit())).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := 0; i < len(items); i++ {
		item := pb.Pin{}

		s.copyModelToOutput(&item, &items[i])

		output.Pin = append(output.Pin, &item)
	}

	return &output, nil
}

func (s *PinService) Sync(ctx context.Context, in *pb.Pin) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Updated")
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

		// source validation
		{
			source, err := s.cs.GetSource().viewWithDeleted(ctx, in.GetSourceId())
			if err != nil {
				return &output, err
			}

			if source.NodeID != in.GetNodeId() {
				return &output, status.Error(codes.NotFound, "Query: source.NodeID != in.GetNodeId()")
			}
		}

		// name validation
		{
			if len(in.GetName()) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
			}

			err = s.cs.GetDB().NewSelect().Model(&model.Pin{}).Where("name = ?", in.GetName()).Where("source_id = ?", in.GetSourceId()).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
			}
		}

		item := model.Pin{
			ID:       in.GetId(),
			NodeID:   in.GetNodeId(),
			SourceID: in.GetSourceId(),
			Name:     in.GetName(),
			Desc:     in.GetDesc(),
			Tags:     in.GetTags(),
			DataType: in.GetDataType(),
			Address:  in.GetAddress(),
			Config:   in.GetConfig(),
			Status:   in.GetStatus(),
			Access:   in.GetAccess(),
			Created:  time.UnixMicro(in.GetCreated()),
			Updated:  time.UnixMicro(in.GetUpdated()),
			Deleted:  time.UnixMicro(in.GetDeleted()),
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
				return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
			}

			modelItem := model.Pin{}
			err = s.cs.GetDB().NewSelect().Model(&modelItem).Where("source_id = ?", item.SourceID).Where("name = ?", in.GetName()).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				if modelItem.ID != item.ID {
					return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
				}
			}
		}

		item.Name = in.GetName()
		item.Desc = in.GetDesc()
		item.Tags = in.GetTags()
		item.DataType = in.GetDataType()
		item.Address = in.GetAddress()
		item.Config = in.GetConfig()
		item.Status = in.GetStatus()
		item.Access = in.GetAccess()
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

func (s *PinService) GC() {
	s.cache.GC()
}

func (s *PinService) ViewFromCacheByID(ctx context.Context, id string) (model.Pin, error) {
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

func (s *PinService) ViewFromCacheByNodeIDAndName(ctx context.Context, nodeID, name string) (model.Pin, error) {
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

func (s *PinService) ViewFromCacheBySourceIDAndName(ctx context.Context, sourceID, name string) (model.Pin, error) {
	if !s.cs.dopts.cache {
		return s.ViewBySourceIDAndName(ctx, sourceID, name)
	}

	id := sourceID + name

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewBySourceIDAndName(ctx, sourceID, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.cs.dopts.cacheTTL)

	return item, nil
}

func (s *PinService) ViewFromCacheBySourceIDAndAddress(ctx context.Context, sourceID, address string) (model.Pin, error) {
	if !s.cs.dopts.cache {
		return s.ViewBySourceIDAndAddress(ctx, sourceID, address)
	}

	id := sourceID + address

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewBySourceIDAndAddress(ctx, sourceID, address)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.cs.dopts.cacheTTL)

	return item, nil
}

// value

func (s *PinService) GetValue(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	output.Id = in.GetId()

	item2, err := s.getPinValueUpdated(ctx, in.GetId())
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item2.Value
	output.Updated = item2.Updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// pin
	item, err := s.ViewFromCacheByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	// validation node and source
	{
		// node
		{
			node, err := s.cs.GetNode().ViewFromCacheByID(ctx, item.NodeID)
			if err != nil {
				return &output, err
			}

			if node.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
			}
		}

		// source
		{
			source, err := s.cs.GetSource().ViewFromCacheByID(ctx, item.SourceID)
			if err != nil {
				return &output, err
			}

			if source.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Source.Status != ON")
			}
		}
	}

	if err = s.setPinValueUpdated(ctx, &item, in.GetValue(), time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateValue(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) GetValueByName(ctx context.Context, in *cores.PinGetValueByNameRequest) (*cores.PinNameValue, error) {
	var err error
	var output cores.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	output.NodeId = in.GetNodeId()
	output.Id = item.ID
	output.Name = in.GetName()

	item2, err := s.getPinValueUpdated(ctx, item.ID)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item2.Value
	output.Updated = item2.Updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetValueByName(ctx context.Context, in *cores.PinNameValue) (*pb.MyBool, error) {
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
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// node
	node, err := s.cs.GetNode().ViewFromCacheByID(ctx, in.GetNodeId())
	if err != nil {
		return &output, err
	}

	if node.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
	}

	// name
	nodeName := consts.DEFAULT_NODE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		nodeName = splits[0]
		itemName = splits[1]
	}

	// source
	source, err := s.cs.GetSource().ViewFromCacheByNodeIDAndName(ctx, node.ID, nodeName)
	if err != nil {
		return &output, err
	}

	if source.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Source.Status != ON")
	}

	// pin
	item, err := s.ViewFromCacheBySourceIDAndName(ctx, source.ID, itemName)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	if err = s.setPinValueUpdated(ctx, &item, in.GetValue(), time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateValue(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) getPinValue(ctx context.Context, id string) (string, error) {
	item2, err := s.getPinValueUpdated(ctx, id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return "", nil
			}
		}

		return "", err
	}

	return item2.Value, nil
}

func (s *PinService) afterUpdateValue(ctx context.Context, item *model.Pin, _ string) error {
	var err error

	err = s.cs.GetSync().setPinValueUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setPinValueUpdated: %v", err)
	}

	return nil
}

// sync value

func (s *PinService) ViewValue(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.getPinValueUpdated(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutputPinValue(&output, &item)

	return &output, nil
}

func (s *PinService) DeleteValue(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.getPinValueUpdated(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	_, err = s.cs.GetDB().NewDelete().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Delete: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) PullValue(ctx context.Context, in *cores.PinPullValueRequest) (*cores.PinPullValueResponse, error) {
	var err error
	var output cores.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.PinValue, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	if len(in.GetSourceId()) > 0 {
		query.Where("source_id = ?", in.GetSourceId())
	}

	err = query.Where("updated > ?", time.UnixMicro(in.GetAfter())).Order("updated ASC").Limit(int(in.GetLimit())).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := 0; i < len(items); i++ {
		item := pb.PinValueUpdated{}

		s.copyModelToOutputPinValue(&item, &items[i])

		output.Pin = append(output.Pin, &item)
	}

	return &output, nil
}

func (s *PinService) SyncValue(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value.Updated")
		}
	}

	// pin
	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	value, err := s.getPinValueUpdated(ctx, in.GetId())
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				goto UPDATED
			}
		}

		return &output, err
	}

	if in.GetUpdated() <= value.Updated.UnixMicro() {
		return &output, nil
	}

UPDATED:
	if err = s.setPinValueUpdated(ctx, &item, in.GetValue(), time.UnixMicro(in.GetUpdated())); err != nil {
		return &output, err
	}

	if err = s.afterUpdateValue(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) setPinValueUpdated(ctx context.Context, item *model.Pin, value string, updated time.Time) error {
	var err error

	item2 := model.PinValue{
		ID:       item.ID,
		NodeID:   item.NodeID,
		SourceID: item.SourceID,
		Value:    value,
		Updated:  updated,
	}

	ret, err := s.cs.GetDB().NewUpdate().Model(&item2).WherePK().WhereAllWithDeleted().Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Update: %v", err)
	}

	n, err := ret.RowsAffected()
	if err != nil {
		return status.Errorf(codes.Internal, "RowsAffected: %v", err)
	}

	if n < 1 {
		_, err = s.cs.GetDB().NewInsert().Model(&item2).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	return nil
}

func (s *PinService) getPinValueUpdated(ctx context.Context, id string) (model.PinValue, error) {
	item := model.PinValue{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutputPinValue(output *pb.PinValueUpdated, item *model.PinValue) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.SourceId = item.SourceID
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()
}

// write

func (s *PinService) GetWrite(ctx context.Context, in *pb.Id) (*pb.PinValue, error) {
	var err error
	var output pb.PinValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	output.Id = in.GetId()

	item2, err := s.getPinWriteUpdated(ctx, in.GetId())
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item2.Value
	output.Updated = item2.Updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// pin
	item, err := s.ViewFromCacheByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	if item.Access != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Access != WRITE")
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	// validation node and source
	{
		// node
		{
			node, err := s.cs.GetNode().ViewFromCacheByID(ctx, item.NodeID)
			if err != nil {
				return &output, err
			}

			if node.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
			}
		}

		// source
		{
			source, err := s.cs.GetSource().ViewFromCacheByID(ctx, item.SourceID)
			if err != nil {
				return &output, err
			}

			if source.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Source.Status != ON")
			}
		}
	}

	if err = s.setPinWriteUpdated(ctx, &item, in.GetValue(), time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateWrite(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) GetWriteByName(ctx context.Context, in *cores.PinGetValueByNameRequest) (*cores.PinNameValue, error) {
	var err error
	var output cores.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetNodeId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.GetNodeId(), in.GetName())
	if err != nil {
		return &output, err
	}

	output.NodeId = in.GetNodeId()
	output.Id = item.ID
	output.Name = in.GetName()

	item2, err := s.getPinWriteUpdated(ctx, item.ID)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item2.Value
	output.Updated = item2.Updated.UnixMicro()

	return &output, nil
}

func (s *PinService) SetWriteByName(ctx context.Context, in *cores.PinNameValue) (*pb.MyBool, error) {
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
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// node
	node, err := s.cs.GetNode().ViewFromCacheByID(ctx, in.GetNodeId())
	if err != nil {
		return &output, err
	}

	if node.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
	}

	// name
	nodeName := consts.DEFAULT_NODE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		nodeName = splits[0]
		itemName = splits[1]
	}

	// source
	source, err := s.cs.GetSource().ViewFromCacheByNodeIDAndName(ctx, node.ID, nodeName)
	if err != nil {
		return &output, err
	}

	if source.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Source.Status != ON")
	}

	// pin
	item, err := s.ViewFromCacheBySourceIDAndName(ctx, source.ID, itemName)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	if item.Access != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Access != WRITE")
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	if err = s.setPinWriteUpdated(ctx, &item, in.GetValue(), time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateWrite(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) getPinWrite(ctx context.Context, id string) (string, error) {
	item2, err := s.getPinWriteUpdated(ctx, id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return "", nil
			}
		}

		return "", err
	}

	return item2.Value, nil
}

func (s *PinService) afterUpdateWrite(ctx context.Context, item *model.Pin, _ string) error {
	var err error

	err = s.cs.GetSync().setPinWriteUpdated(ctx, s.cs.GetDB(), item.NodeID, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setPinWriteUpdated: %v", err)
	}

	return nil
}

// sync value

func (s *PinService) ViewWrite(ctx context.Context, in *pb.Id) (*pb.PinValueUpdated, error) {
	var output pb.PinValueUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.getPinWriteUpdated(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	s.copyModelToOutputPinWrite(&output, &item)

	return &output, nil
}

func (s *PinService) DeleteWrite(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}
	}

	item, err := s.getPinWriteUpdated(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	_, err = s.cs.GetDB().NewDelete().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Delete: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) PullWrite(ctx context.Context, in *cores.PinPullValueRequest) (*cores.PinPullValueResponse, error) {
	var err error
	var output cores.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.PinWrite, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.GetNodeId() != "" {
		query.Where("node_id = ?", in.GetNodeId())
	}

	if len(in.GetSourceId()) > 0 {
		query.Where("source_id = ?", in.GetSourceId())
	}

	err = query.Where("updated > ?", time.UnixMicro(in.GetAfter())).Order("updated ASC").Limit(int(in.GetLimit())).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := 0; i < len(items); i++ {
		item := pb.PinValueUpdated{}

		s.copyModelToOutputPinWrite(&item, &items[i])

		output.Pin = append(output.Pin, &item)
	}

	return &output, nil
}

func (s *PinService) SyncWrite(ctx context.Context, in *pb.PinValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetId() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value.Updated")
		}
	}

	// pin
	item, err := s.ViewByID(ctx, in.GetId())
	if err != nil {
		return &output, err
	}

	_, err = datatype.DecodeNsonValue(in.GetValue(), item.ValueTag())
	if err != nil {
		return &output, status.Errorf(codes.InvalidArgument, "DecodeValue: %v", err)
	}

	value, err := s.getPinWriteUpdated(ctx, in.GetId())
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				goto UPDATED
			}
		}

		return &output, err
	}

	if in.GetUpdated() <= value.Updated.UnixMicro() {
		return &output, nil
	}

UPDATED:
	if err = s.setPinWriteUpdated(ctx, &item, in.GetValue(), time.UnixMicro(in.GetUpdated())); err != nil {
		return &output, err
	}

	if err = s.afterUpdateWrite(ctx, &item, in.GetValue()); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) setPinWriteUpdated(ctx context.Context, item *model.Pin, value string, updated time.Time) error {
	var err error

	item2 := model.PinWrite{
		ID:       item.ID,
		NodeID:   item.NodeID,
		SourceID: item.SourceID,
		Value:    value,
		Updated:  updated,
	}

	ret, err := s.cs.GetDB().NewUpdate().Model(&item2).WherePK().WhereAllWithDeleted().Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Update: %v", err)
	}

	n, err := ret.RowsAffected()
	if err != nil {
		return status.Errorf(codes.Internal, "RowsAffected: %v", err)
	}

	if n < 1 {
		_, err = s.cs.GetDB().NewInsert().Model(&item2).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	return nil
}

func (s *PinService) getPinWriteUpdated(ctx context.Context, id string) (model.PinWrite, error) {
	item := model.PinWrite{
		ID: id,
	}

	err := s.cs.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutputPinWrite(output *pb.PinValueUpdated, item *model.PinWrite) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.SourceId = item.SourceID
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()
}

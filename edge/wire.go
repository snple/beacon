package edge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"github.com/snple/beacon/util"
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WireService struct {
	es *EdgeService

	cache *cache.Cache[string, model.Wire]

	edges.UnimplementedWireServiceServer
}

func newWireService(es *EdgeService) *WireService {
	return &WireService{
		es:    es,
		cache: cache.NewCache[string, model.Wire](nil),
	}
}

func (s *WireService) Create(ctx context.Context, in *pb.Wire) (*pb.Wire, error) {
	var output pb.Wire
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	item := model.Wire{
		ID:       in.Id,
		Name:     in.Name,
		Clusters: in.Clusters,
		Status:   in.Status,
		Created:  time.Now(),
		Updated:  time.Now(),
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Wire.Name min 2 character")
		}

		err = s.es.GetDB().NewSelect().Model(&model.Wire{}).Where("name = ?", in.Name).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Wire.Name must be unique")
		}
	}

	if item.ID == "" {
		item.ID = util.RandomID()
	}

	_, err = s.es.GetDB().NewInsert().Model(&item).Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Insert: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *WireService) Update(ctx context.Context, in *pb.Wire) (*pb.Wire, error) {
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

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Wire.Name min 2 character")
		}

		modelItem := model.Wire{}
		err = s.es.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			if modelItem.ID != item.ID {
				return &output, status.Error(codes.AlreadyExists, "Wire.Name must be unique")
			}
		}
	}

	item.Name = in.Name
	item.Clusters = in.Clusters
	item.Status = in.Status
	item.Updated = time.Now()

	_, err = s.es.GetDB().NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
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

func (s *WireService) Name(ctx context.Context, in *pb.Name) (*pb.Wire, error) {
	var output pb.Wire
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}
	}

	item, err := s.ViewByName(ctx, in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *WireService) Delete(ctx context.Context, in *pb.Id) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

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

	item.Updated = time.Now()
	item.Deleted = time.Now()

	_, err = s.es.GetDB().NewUpdate().Model(&item).Column("updated", "deleted").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Delete: %v", err)
	}

	if err = s.afterDelete(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *WireService) List(ctx context.Context, in *edges.WireListRequest) (*edges.WireListResponse, error) {
	var err error
	var output edges.WireListResponse

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

	query := s.es.GetDB().NewSelect().Model(&items)

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

func (s *WireService) Clone(ctx context.Context, in *edges.WireCloneRequest) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
		}
	}

	tx, err := s.es.GetDB().BeginTx(ctx, nil)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BeginTx: %v", err)
	}
	var done bool
	defer func() {
		if !done {
			_ = tx.Rollback()
		}
	}()

	err = s.es.getClone().wire(ctx, tx, in.Id)
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

func (s *WireService) ViewByID(ctx context.Context, id string) (model.Wire, error) {
	item := model.Wire{
		ID: id,
	}

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Wire.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *WireService) ViewByName(ctx context.Context, name string) (model.Wire, error) {
	item := model.Wire{
		Name: name,
	}

	err := s.es.GetDB().NewSelect().Model(&item).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Wire.Name: %v", err, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *WireService) copyModelToOutput(output *pb.Wire, item *model.Wire) {
	output.Id = item.ID
	output.Name = item.Name
	output.Clusters = item.Clusters
	output.Status = item.Status
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Deleted.UnixMicro()
}

func (s *WireService) afterUpdate(ctx context.Context, _ *model.Wire) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.es.GetSync().setWireUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setWireUpdated: %v", err)
	}

	return nil
}

func (s *WireService) afterDelete(ctx context.Context, _ *model.Wire) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.es.GetSync().setWireUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setWireUpdated: %v", err)
	}

	return nil
}

// sync

func (s *WireService) ViewWithDeleted(ctx context.Context, in *pb.Id) (*pb.Wire, error) {
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

	item, err := s.viewWithDeleted(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *WireService) viewWithDeleted(ctx context.Context, id string) (model.Wire, error) {
	item := model.Wire{
		ID: id,
	}

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Wire.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *WireService) Pull(ctx context.Context, in *edges.WirePullRequest) (*edges.WirePullResponse, error) {
	var err error
	var output edges.WirePullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.After
	output.Limit = in.Limit

	var items []model.Wire

	query := s.es.GetDB().NewSelect().Model(&items)

	if in.Source != "" {
		query = query.Where(`source = ?`, in.Source)
	}

	err = query.Where("updated > ?", time.UnixMicro(in.After)).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.Limit)).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
		item := pb.Wire{}

		s.copyModelToOutput(&item, &items[i])

		output.Wires = append(output.Wires, &item)
	}

	return &output, nil
}

func (s *WireService) Sync(ctx context.Context, in *pb.Wire) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.ID")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Name")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Wire.Updated")
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

	// insert
	if insert {
		// name validation
		{
			if len(in.Name) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Wire.Name min 2 character")
			}

			err = s.es.GetDB().NewSelect().Model(&model.Wire{}).Where("name = ?", in.Name).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				return &output, status.Error(codes.AlreadyExists, "Wire.Name must be unique")
			}
		}

		item := model.Wire{
			ID:       in.Id,
			Name:     in.Name,
			Clusters: in.Clusters,
			Status:   in.Status,
			Created:  time.UnixMicro(in.Created),
			Updated:  time.UnixMicro(in.Updated),
			Deleted:  time.UnixMicro(in.Deleted),
		}

		_, err = s.es.GetDB().NewInsert().Model(&item).Exec(ctx)
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
				return &output, status.Error(codes.InvalidArgument, "Wire.Name min 2 character")
			}

			modelItem := model.Wire{}
			err = s.es.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				if modelItem.ID != item.ID {
					return &output, status.Error(codes.AlreadyExists, "Wire.Name must be unique")
				}
			}
		}

		item.Name = in.Name
		item.Clusters = in.Clusters
		item.Status = in.Status
		item.Updated = time.UnixMicro(in.Updated)
		item.Deleted = time.UnixMicro(in.Deleted)

		_, err = s.es.GetDB().NewUpdate().Model(&item).WherePK().WhereAllWithDeleted().Exec(ctx)
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

func (s *WireService) GC() {
	s.cache.GC()
}

func (s *WireService) ViewFromCacheByID(ctx context.Context, id string) (model.Wire, error) {
	if !s.es.dopts.cache {
		return s.ViewByID(ctx, id)
	}

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByID(ctx, id)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.es.dopts.cacheTTL)

	return item, nil
}

func (s *WireService) ViewFromCacheByName(ctx context.Context, name string) (model.Wire, error) {
	if !s.es.dopts.cache {
		return s.ViewByName(ctx, name)
	}

	if option := s.cache.Get(name); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByName(ctx, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(name, item, s.es.dopts.cacheTTL)

	return item, nil
}

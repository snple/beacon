package edge

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/snple/beacon/consts"
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"github.com/snple/beacon/util"
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ConstService struct {
	es *EdgeService

	cache *cache.Cache[string, model.Const]

	edges.UnimplementedConstServiceServer
}

func newConstService(es *EdgeService) *ConstService {
	return &ConstService{
		es:    es,
		cache: cache.NewCache[string, model.Const](nil),
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

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if !dt.ValidateType(in.Type) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Type")
		}

		if !dt.ValidateValue(in.Value, in.Type) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
		}

		err = s.es.GetDB().NewSelect().Model(&model.Const{}).Where("name = ?", in.Name).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
		}
	}

	item := model.Const{
		ID:      in.Id,
		Name:    in.Name,
		Desc:    in.Desc,
		Tags:    in.Tags,
		Type:    in.Type,
		Value:   in.Value,
		Config:  in.Config,
		Status:  in.Status,
		Created: time.Now(),
		Updated: time.Now(),
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

func (s *ConstService) Update(ctx context.Context, in *pb.Const) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if !dt.ValidateType(in.Type) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Type")
		}

		if !dt.ValidateValue(in.Value, in.Type) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	// name validation
	{
		if len(in.Name) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
		}

		modelItem := model.Const{}
		err = s.es.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
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

	item.Name = in.Name
	item.Desc = in.Desc
	item.Tags = in.Tags
	item.Type = in.Type
	item.Value = in.Value
	item.Config = in.Config
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

func (s *ConstService) View(ctx context.Context, in *pb.Id) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *ConstService) Name(ctx context.Context, in *pb.Name) (*pb.Const, error) {
	var output pb.Const
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}
	}

	item, err := s.ViewByName(ctx, in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
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

func (s *ConstService) List(ctx context.Context, in *edges.ConstListRequest) (*edges.ConstListResponse, error) {
	var err error
	var output edges.ConstListResponse

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

		output.Consts = append(output.Consts, &item)
	}

	return &output, nil
}

func (s *ConstService) Clone(ctx context.Context, in *edges.ConstCloneRequest) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}
	}

	err = s.es.getClone().const_(ctx, s.es.GetDB(), in.Id)
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

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Const.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) ViewByName(ctx context.Context, name string) (model.Const, error) {
	item := model.Const{}

	err := s.es.GetDB().NewSelect().Model(&item).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Const.Name: %v", err, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) copyModelToOutput(output *pb.Const, item *model.Const) {
	output.Id = item.ID
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

func (s *ConstService) afterUpdate(ctx context.Context, _ *model.Const) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	err = s.es.GetSync().setConstUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	return nil
}

func (s *ConstService) afterDelete(ctx context.Context, _ *model.Const) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
	}

	err = s.es.GetSync().setConstUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Insert: %v", err)
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}
	}

	item, err := s.viewWithDeleted(ctx, in.Id)
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

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Const.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *ConstService) Pull(ctx context.Context, in *edges.ConstPullRequest) (*edges.ConstPullResponse, error) {
	var err error
	var output edges.ConstPullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.After
	output.Limit = in.Limit

	items := make([]model.Const, 0, 10)

	query := s.es.GetDB().NewSelect().Model(&items)

	err = query.Where("updated > ?", time.UnixMicro(in.After)).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.Limit)).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
		item := pb.Const{}

		s.copyModelToOutput(&item, &items[i])

		output.Consts = append(output.Consts, &item)
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Updated")
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
				return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
			}

			err = s.es.GetDB().NewSelect().Model(&model.Const{}).Where("name = ?", in.Name).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				return &output, status.Error(codes.AlreadyExists, "Const.Name must be unique")
			}
		}

		item := model.Const{
			ID:      in.Id,
			Name:    in.Name,
			Desc:    in.Desc,
			Tags:    in.Tags,
			Type:    in.Type,
			Value:   in.Value,
			Config:  in.Config,
			Status:  in.Status,
			Created: time.UnixMicro(in.Created),
			Updated: time.UnixMicro(in.Updated),
			Deleted: time.UnixMicro(in.Deleted),
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
				return &output, status.Error(codes.InvalidArgument, "Const.Name min 2 character")
			}

			modelItem := model.Const{}
			err = s.es.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
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

		item.Name = in.Name
		item.Desc = in.Desc
		item.Tags = in.Tags
		item.Type = in.Type
		item.Value = in.Value
		item.Config = in.Config
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

func (s *ConstService) GC() {
	s.cache.GC()
}

func (s *ConstService) ViewFromCacheByID(ctx context.Context, id string) (model.Const, error) {
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

func (s *ConstService) ViewFromByName(ctx context.Context, name string) (model.Const, error) {
	if !s.es.dopts.cache {
		return s.ViewByName(ctx, name)
	}

	if option := s.cache.Get(name); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByID(ctx, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(name, item, s.es.dopts.cacheTTL)

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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	output.Id = in.Id
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Id")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Const.Status != ON")
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Const.Value")
	}

	item.Value = in.Value

	_, err = s.es.GetDB().NewUpdate().Model(&item).Column("value").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *ConstService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.ConstNameValue, error) {
	var err error
	var output pb.ConstNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}
	}

	item, err := s.ViewByName(ctx, in.Name)
	if err != nil {
		return &output, err
	}

	output.Id = item.ID
	output.Name = in.Name
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()

	return &output, nil
}

func (s *ConstService) SetValueByName(ctx context.Context, in *pb.ConstNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Name")
		}

		if in.Value == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Value")
		}
	}

	item, err := s.ViewByName(ctx, in.Name)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Const.Status != ON")
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Const.Value")
	}

	item.Value = in.Value
	item.Updated = time.Now()

	_, err = s.es.GetDB().NewUpdate().Model(&item).Column("value", "updated").WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

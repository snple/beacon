package edge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
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

type PinService struct {
	es *EdgeService

	cache *cache.Cache[model.Pin]

	edges.UnimplementedPinServiceServer
}

func newPinService(es *EdgeService) *PinService {
	return &PinService{
		es:    es,
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

		if len(in.GetWireId()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.WireID")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Type")
		}
	}

	// wire validation
	{
		_, err = s.es.GetWire().ViewByID(ctx, in.GetWireId())
		if err != nil {
			return &output, err
		}
	}

	// name validation
	{
		if len(in.GetName()) < 2 {
			return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
		}

		err = s.es.GetDB().NewSelect().Model(&model.Pin{}).Where("name = ?", in.GetName()).Where("wire_id = ?", in.GetWireId()).Scan(ctx)
		if err != nil {
			if err != sql.ErrNoRows {
				return &output, status.Errorf(codes.Internal, "Query: %v", err)
			}
		} else {
			return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
		}
	}

	item := model.Pin{
		ID:      in.GetId(),
		WireID:  in.GetWireId(),
		Name:    in.GetName(),
		Desc:    in.GetDesc(),
		Tags:    in.GetTags(),
		Type:    in.GetType(),
		Addr:    in.GetAddr(),
		Config:  in.GetConfig(),
		Status:  in.GetStatus(),
		Rw:      in.GetRw(),
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

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Type")
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
		err = s.es.GetDB().NewSelect().Model(&modelItem).Where("wire_id = ?", item.WireID).Where("name = ?", in.GetName()).Scan(ctx)
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
	item.Type = in.GetType()
	item.Addr = in.GetAddr()
	item.Config = in.GetConfig()
	item.Status = in.GetStatus()
	item.Rw = in.GetRw()
	item.Updated = time.Now()

	_, err = s.es.GetDB().NewUpdate().Model(&item).WherePK().Exec(ctx)
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

func (s *PinService) Name(ctx context.Context, in *pb.Name) (*pb.Pin, error) {
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

	item, err := s.ViewByName(ctx, in.GetName())
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

func (s *PinService) List(ctx context.Context, in *edges.PinListRequest) (*edges.PinListResponse, error) {
	var err error
	var output edges.PinListResponse

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

	query := s.es.GetDB().NewSelect().Model(&items)

	if len(in.GetWireId()) > 0 {
		query.Where("wire_id = ?", in.GetWireId())
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

	for i := range items {
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

func (s *PinService) Clone(ctx context.Context, in *edges.PinCloneRequest) (*pb.MyBool, error) {
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

	err = s.es.getClone().pin(ctx, s.es.GetDB(), in.GetId(), in.GetWireId())
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

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) ViewByName(ctx context.Context, name string) (model.Pin, error) {
	item := model.Pin{}

	wireName := consts.DEFAULT_WIRE
	itemName := name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return item, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		wireName = splits[0]
		itemName = splits[1]
	}

	wire, err := s.es.GetWire().ViewByName(ctx, wireName)
	if err != nil {
		return item, err
	}

	return s.ViewByWireIDAndName(ctx, wire.ID, itemName)
}

func (s *PinService) ViewByWireIDAndName(ctx context.Context, wireID, name string) (model.Pin, error) {
	item := model.Pin{}

	err := s.es.GetDB().NewSelect().Model(&item).Where("wire_id = ?", wireID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, WireID: %v, Name: %v", err, wireID, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) ViewByWireIDAndAddress(ctx context.Context, wireID, address string) (model.Pin, error) {
	item := model.Pin{}

	err := s.es.GetDB().NewSelect().Model(&item).Where("wire_id = ?", wireID).Where("address = ?", address).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, WireID: %v, Address: %v", err, wireID, address)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutput(output *pb.Pin, item *model.Pin) {
	output.Id = item.ID
	output.WireId = item.WireID
	output.Name = item.Name
	output.Desc = item.Desc
	output.Tags = item.Tags
	output.Type = item.Type
	output.Addr = item.Addr
	output.Config = item.Config
	output.Status = item.Status
	output.Rw = item.Rw
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Deleted.UnixMicro()
}

func (s *PinService) afterUpdate(ctx context.Context, _ *model.Pin) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.es.GetSync().setPinUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setPinUpdated: %v", err)
	}

	return nil
}

func (s *PinService) afterDelete(ctx context.Context, _ *model.Pin) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	err = s.es.GetSync().setPinUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setPinUpdated: %v", err)
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

	err := s.es.GetDB().NewSelect().Model(&item).WherePK().WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Pin.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) Pull(ctx context.Context, in *edges.PinPullRequest) (*edges.PinPullResponse, error) {
	var err error
	var output edges.PinPullResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.Pin, 0, 10)

	query := s.es.GetDB().NewSelect().Model(&items)

	if in.GetWireId() != "" {
		query.Where("wire_id = ?", in.GetWireId())
	}

	err = query.Where("updated > ?", time.UnixMicro(in.GetAfter())).WhereAllWithDeleted().Order("updated ASC").Limit(int(in.GetLimit())).Scan(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
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

		if !dt.ValidateType(in.GetType()) {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Type")
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
		// name validation
		{
			if len(in.GetName()) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
			}

			err = s.es.GetDB().NewSelect().Model(&model.Pin{}).Where("name = ?", in.GetName()).Where("wire_id = ?", in.GetWireId()).Scan(ctx)
			if err != nil {
				if err != sql.ErrNoRows {
					return &output, status.Errorf(codes.Internal, "Query: %v", err)
				}
			} else {
				return &output, status.Error(codes.AlreadyExists, "Pin.Name must be unique")
			}
		}

		item := model.Pin{
			ID:      in.GetId(),
			WireID:  in.GetWireId(),
			Name:    in.GetName(),
			Desc:    in.GetDesc(),
			Tags:    in.GetTags(),
			Type:    in.GetType(),
			Addr:    in.GetAddr(),
			Config:  in.GetConfig(),
			Status:  in.GetStatus(),
			Rw:      in.GetRw(),
			Created: time.UnixMicro(in.GetCreated()),
			Updated: time.UnixMicro(in.GetUpdated()),
			Deleted: time.UnixMicro(in.GetDeleted()),
		}

		// wire validation
		{
			_, err = s.es.GetWire().viewWithDeleted(ctx, in.GetWireId())
			if err != nil {
				return &output, err
			}
		}

		_, err = s.es.GetDB().NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	// update
	if update {
		if in.GetUpdated() <= item.Updated.UnixMicro() {
			return &output, nil
		}

		// name validation
		{
			if len(in.GetName()) < 2 {
				return &output, status.Error(codes.InvalidArgument, "Pin.Name min 2 character")
			}

			modelItem := model.Pin{}
			err = s.es.GetDB().NewSelect().Model(&modelItem).Where("wire_id = ?", item.WireID).Where("name = ?", in.GetName()).Scan(ctx)
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
		item.Type = in.GetType()
		item.Addr = in.GetAddr()
		item.Config = in.GetConfig()
		item.Status = in.GetStatus()
		item.Rw = in.GetRw()
		item.Updated = time.UnixMicro(in.GetUpdated())
		item.Deleted = time.UnixMicro(in.GetDeleted())

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

func (s *PinService) GC() {
	s.cache.GC()
}

func (s *PinService) ViewFromCacheByID(ctx context.Context, id string) (model.Pin, error) {
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

func (s *PinService) ViewFromCacheByName(ctx context.Context, name string) (model.Pin, error) {
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

func (s *PinService) ViewFromCacheByWireIDAndName(ctx context.Context, wireID, name string) (model.Pin, error) {
	if !s.es.dopts.cache {
		return s.ViewByWireIDAndName(ctx, wireID, name)
	}

	id := wireID + name

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByWireIDAndName(ctx, wireID, name)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.es.dopts.cacheTTL)

	return item, nil
}

func (s *PinService) ViewFromCacheByWireIDAndAddress(ctx context.Context, wireID, address string) (model.Pin, error) {
	if !s.es.dopts.cache {
		return s.ViewByWireIDAndAddress(ctx, wireID, address)
	}

	id := wireID + address

	if option := s.cache.Get(id); option.IsSome() {
		return option.Unwrap(), nil
	}

	item, err := s.ViewByWireIDAndAddress(ctx, wireID, address)
	if err != nil {
		return item, err
	}

	s.cache.Set(id, item, s.es.dopts.cacheTTL)

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

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	// validation wire and wire
	{
		// wire
		{
			wire, err := s.es.GetWire().ViewFromCacheByID(ctx, item.WireID)
			if err != nil {
				return &output, err
			}

			if wire.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Wire.Status != ON")
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

func (s *PinService) GetValueByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByName(ctx, in.GetName())
	if err != nil {
		return &output, err
	}

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

func (s *PinService) SetValueByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// name
	wireName := consts.DEFAULT_WIRE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		wireName = splits[0]
		itemName = splits[1]
	}

	// wire
	wire, err := s.es.GetWire().ViewFromCacheByName(ctx, wireName)
	if err != nil {
		return &output, err
	}

	if wire.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Wire.Status != ON")
	}

	// pin
	item, err := s.ViewFromCacheByWireIDAndName(ctx, wire.ID, itemName)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
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

func (s *PinService) afterUpdateValue(ctx context.Context, _ *model.Pin, _ string) error {
	var err error

	err = s.es.GetSync().setPinValueUpdated(ctx, time.Now())
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

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	ts := uint64(time.Now().UnixMicro())

	txn := s.es.GetBadgerDB().NewTransactionAt(ts, true)
	defer txn.Discard()

	err = txn.Delete(append([]byte(model.PIN_VALUE_PREFIX), idb...))
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BadgerDB Delete: %v", err)
	}

	err = txn.CommitAt(ts, nil)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BadgerDB CommitAt: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) PullValue(ctx context.Context, in *edges.PinPullValueRequest) (*edges.PinPullValueResponse, error) {
	var err error
	var output edges.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.PinValue, 0, 10)

	{
		after := time.UnixMicro(in.GetAfter())

		txn := s.es.GetBadgerDB().NewTransactionAt(uint64(time.Now().UnixMicro()), false)
		defer txn.Discard()

		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		opts.SinceTs = uint64(in.GetAfter())

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(model.PIN_VALUE_PREFIX)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			dbitem := it.Item()

			item := model.PinValue{}
			err = dbitem.Value(func(val []byte) error {
				return json.Unmarshal(val, &item)
			})
			if err != nil {
				return &output, status.Errorf(codes.Internal, "BadgerDB view value: %v", err)
			}

			if !item.Updated.After(after) {
				continue
			}

			if in.GetWireId() != "" && in.GetWireId() != item.WireID {
				continue
			}

			items = append(items, item)
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].Updated.Before(items[j].Updated)
		})

		if len(items) > int(in.GetLimit()) {
			items = items[0:in.GetLimit()]
		}
	}

	for i := range items {
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

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
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

func (s *PinService) setPinValueUpdated(_ context.Context, item *model.Pin, value string, updated time.Time) error {
	item2 := model.PinValue{
		ID:      item.ID,
		WireID:  item.WireID,
		Value:   value,
		Updated: updated,
	}

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	data, err := json.Marshal(item2)
	if err != nil {
		return status.Errorf(codes.Internal, "json.Marshal: %v", err)
	}

	{
		ts := uint64(updated.UnixMicro())

		txn := s.es.GetBadgerDB().NewTransactionAt(ts, true)
		defer txn.Discard()

		err = txn.Set(append([]byte(model.PIN_VALUE_PREFIX), idb...), data)
		if err != nil {
			return status.Errorf(codes.Internal, "BadgerDB Set: %v", err)
		}

		err = txn.CommitAt(ts, nil)
		if err != nil {
			return status.Errorf(codes.Internal, "BadgerDB CommitAt: %v", err)
		}
	}

	return nil
}

func (s *PinService) getPinValueUpdated(_ context.Context, id string) (model.PinValue, error) {
	item := model.PinValue{
		ID: id,
	}

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return item, status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	txn := s.es.GetBadgerDB().NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	dbitem, err := txn.Get(append([]byte(model.PIN_VALUE_PREFIX), idb...))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return item, status.Errorf(codes.NotFound, "Pin.ID: %v", item.ID)
		}
		return item, status.Errorf(codes.Internal, "BadgerDB Get: %v", err)
	}

	err = dbitem.Value(func(val []byte) error {
		return json.Unmarshal(val, &item)
	})
	if err != nil {
		return item, status.Errorf(codes.Internal, "BadgerDB Get Value: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutputPinValue(output *pb.PinValueUpdated, item *model.PinValue) {
	output.Id = item.ID
	output.WireId = item.WireID
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

	if item.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	// validation wire and wire
	{
		// wire
		{
			wire, err := s.es.GetWire().ViewFromCacheByID(ctx, item.WireID)
			if err != nil {
				return &output, err
			}

			if wire.Status != consts.ON {
				return &output, status.Errorf(codes.FailedPrecondition, "Wire.Status != ON")
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

func (s *PinService) GetWriteByName(ctx context.Context, in *pb.Name) (*pb.PinNameValue, error) {
	var err error
	var output pb.PinNameValue

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByName(ctx, in.GetName())
	if err != nil {
		return &output, err
	}

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

func (s *PinService) SetWriteByName(ctx context.Context, in *pb.PinNameValue) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetName() == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if len(in.GetValue()) == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// name
	wireName := consts.DEFAULT_WIRE
	itemName := in.GetName()

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		wireName = splits[0]
		itemName = splits[1]
	}

	// wire
	wire, err := s.es.GetWire().ViewFromCacheByName(ctx, wireName)
	if err != nil {
		return &output, err
	}

	if wire.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Wire.Status != ON")
	}

	// pin
	item, err := s.ViewFromCacheByWireIDAndName(ctx, wire.ID, itemName)
	if err != nil {
		return &output, err
	}

	if item.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Status != ON")
	}

	if item.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
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

func (s *PinService) afterUpdateWrite(ctx context.Context, _ *model.Pin, _ string) error {
	var err error

	err = s.es.GetSync().setPinWriteUpdated(ctx, time.Now())
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

	s.copyModelToOutputPinValue(&output, &item)

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

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	ts := uint64(time.Now().UnixMicro())

	txn := s.es.GetBadgerDB().NewTransactionAt(ts, true)
	defer txn.Discard()

	err = txn.Delete(append([]byte(model.PIN_WRITE_PREFIX), idb...))
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BadgerDB Delete: %v", err)
	}

	err = txn.CommitAt(ts, nil)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "BadgerDB CommitAt: %v", err)
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) PullWrite(ctx context.Context, in *edges.PinPullValueRequest) (*edges.PinPullValueResponse, error) {
	var err error
	var output edges.PinPullValueResponse

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	output.After = in.GetAfter()
	output.Limit = in.GetLimit()

	items := make([]model.PinValue, 0, 10)

	{
		after := time.UnixMicro(in.GetAfter())

		txn := s.es.GetBadgerDB().NewTransactionAt(uint64(time.Now().UnixMicro()), false)
		defer txn.Discard()

		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		opts.SinceTs = uint64(in.GetAfter())

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(model.PIN_WRITE_PREFIX)

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			dbitem := it.Item()

			item := model.PinValue{}
			err = dbitem.Value(func(val []byte) error {
				return json.Unmarshal(val, &item)
			})
			if err != nil {
				return &output, status.Errorf(codes.Internal, "BadgerDB view value: %v", err)
			}

			if !item.Updated.After(after) {
				continue
			}

			if in.GetWireId() != "" && in.GetWireId() != item.WireID {
				continue
			}

			items = append(items, item)
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].Updated.Before(items[j].Updated)
		})

		if len(items) > int(in.GetLimit()) {
			items = items[0:in.GetLimit()]
		}
	}

	for i := range items {
		item := pb.PinValueUpdated{}

		s.copyModelToOutputPinValue(&item, &items[i])

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

	if !dt.ValidateValue(in.GetValue(), item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
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

func (s *PinService) setPinWriteUpdated(_ context.Context, item *model.Pin, value string, updated time.Time) error {
	item2 := model.PinValue{
		ID:      item.ID,
		WireID:  item.WireID,
		Value:   value,
		Updated: updated,
	}

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	data, err := json.Marshal(item2)
	if err != nil {
		return status.Errorf(codes.Internal, "json.Marshal: %v", err)
	}

	{
		ts := uint64(updated.UnixMicro())

		txn := s.es.GetBadgerDB().NewTransactionAt(ts, true)
		defer txn.Discard()

		err = txn.Set(append([]byte(model.PIN_WRITE_PREFIX), idb...), data)
		if err != nil {
			return status.Errorf(codes.Internal, "BadgerDB Set: %v", err)
		}

		err = txn.CommitAt(ts, nil)
		if err != nil {
			return status.Errorf(codes.Internal, "BadgerDB CommitAt: %v", err)
		}
	}

	return nil
}

func (s *PinService) getPinWriteUpdated(_ context.Context, id string) (model.PinValue, error) {
	item := model.PinValue{
		ID: id,
	}

	idb, err := nson.IdFromHex(item.ID)
	if err != nil {
		return item, status.Errorf(codes.Internal, "IdFromHex: %v", err)
	}

	txn := s.es.GetBadgerDB().NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	dbitem, err := txn.Get(append([]byte(model.PIN_WRITE_PREFIX), idb...))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return item, status.Errorf(codes.NotFound, "Pin.ID: %v", item.ID)
		}
		return item, status.Errorf(codes.Internal, "BadgerDB Get: %v", err)
	}

	err = dbitem.Value(func(val []byte) error {
		return json.Unmarshal(val, &item)
	})
	if err != nil {
		return item, status.Errorf(codes.Internal, "BadgerDB Get Value: %v", err)
	}

	return item, nil
}

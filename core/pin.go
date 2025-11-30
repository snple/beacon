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
	"github.com/snple/beacon/dt"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/snple/types/cache"
	"github.com/uptrace/bun"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PinService struct {
	cs *CoreService

	cache *cache.Cache[string, model.Pin]

	cores.UnimplementedPinServiceServer
}

func newPinService(cs *CoreService) *PinService {
	return &PinService{
		cs:    cs,
		cache: cache.NewCache[string, model.Pin](nil),
	}
}

func (s *PinService) View(ctx context.Context, in *pb.Id) (*pb.Pin, error) {
	var output pb.Pin
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	item, err := s.ViewByID(ctx, in.Id)
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

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewByNodeIDAndName(ctx, in.NodeId, in.Name)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Name = in.Name

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

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	nodeName := consts.DEFAULT_NODE
	wireName := consts.DEFAULT_WIRE
	itemName := in.Name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")

		switch len(splits) {
		case 2:
			wireName = splits[0]
			itemName = splits[1]
		case 3:
			nodeName = splits[0]
			wireName = splits[1]
			itemName = splits[2]
		default:
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	node, err := s.cs.GetNode().ViewByName(ctx, nodeName)
	if err != nil {
		return &output, err
	}

	wire, err := s.cs.GetWire().ViewByNodeIDAndName(ctx, node.ID, wireName)
	if err != nil {
		return &output, err
	}

	item, err := s.ViewByWireIDAndName(ctx, wire.ID, itemName)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	output.Name = in.Name

	output.Value, err = s.getPinValue(ctx, item.ID)
	if err != nil {
		return &output, err
	}

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

	if in.NodeId != "" {
		query.Where("node_id = ?", in.NodeId)
	}

	if in.WireId != "" {
		query.Where("wire_id = ?", in.WireId)
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
		item := pb.Pin{}

		s.copyModelToOutput(&item, &items[i])

		item.Value, err = s.getPinValue(ctx, items[i].ID)
		if err != nil {
			return &output, err
		}

		output.Pins = append(output.Pins, &item)
	}

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

	wire, err := s.cs.GetWire().ViewByNodeIDAndName(ctx, nodeID, wireName)
	if err != nil {
		return item, err
	}

	return s.ViewByWireIDAndName(ctx, wire.ID, itemName)
}

func (s *PinService) ViewByWireIDAndName(ctx context.Context, wireID, name string) (model.Pin, error) {
	item := model.Pin{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("wire_id = ?", wireID).Where("name = ?", name).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, WireID: %v, Pin.Name: %v", err, wireID, name)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) ViewByWireIDAndAddress(ctx context.Context, wireID, address string) (model.Pin, error) {
	item := model.Pin{}

	err := s.cs.GetDB().NewSelect().Model(&item).Where("wire_id = ?", wireID).Where("address = ?", address).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, WireID: %v, Pin.Address: %v", err, wireID, address)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *PinService) copyModelToOutput(output *pb.Pin, item *model.Pin) {
	output.Id = item.ID
	output.NodeId = item.NodeID
	output.WireId = item.WireID
	output.Name = item.Name
	output.Type = item.Type
	output.Addr = item.Addr
	output.Rw = item.Rw
	output.Updated = item.Updated.UnixMicro()
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

// Push 接收 stream 数据并插入 Pin
func (s *PinService) Push(stream grpc.ClientStreamingServer[pb.Pin, pb.MyBool]) error {
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
				return status.Error(codes.InvalidArgument, "Please supply valid Pin.ID")
			}

			if in.NodeId == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Pin.NodeId")
			}

			if in.WireId == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Pin.WireId")
			}

			if in.Name == "" {
				return status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
			}

			if !dt.ValidateType(in.Type) {
				return status.Error(codes.InvalidArgument, "Please supply valid Pin.Type")
			}
		}

		// wire validation
		{
			wire, err := s.cs.GetWire().ViewByID(ctx, in.WireId)
			if err != nil {
				return err
			}

			if wire.NodeID != in.NodeId {
				return status.Error(codes.NotFound, "Query: wire.NodeID != in.NodeId")
			}
		}

		item := model.Pin{
			ID:      in.Id,
			NodeID:  in.NodeId,
			WireID:  in.WireId,
			Name:    in.Name,
			Type:    in.Type,
			Addr:    in.Addr,
			Rw:      in.Rw,
			Updated: time.UnixMicro(in.Updated),
		}

		_, err = s.cs.GetDB().NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}
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

func (s *PinService) ViewFromCacheByWireIDAndName(ctx context.Context, wireID, name string) (model.Pin, error) {
	if !s.cs.dopts.cache {
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

	s.cache.Set(id, item, s.cs.dopts.cacheTTL)

	return item, nil
}

func (s *PinService) ViewFromCacheByWireIDAndAddress(ctx context.Context, wireID, address string) (model.Pin, error) {
	if !s.cs.dopts.cache {
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	output.Id = in.Id

	item, err := s.getPinValueUpdated(ctx, in.Id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()

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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		if in.Value == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// pin
	item, err := s.ViewFromCacheByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	// validation node and wire
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
	}

	if err = s.setPinValueUpdated(ctx, &item, in.Value, time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateValue(ctx, &item, in.Value); err != nil {
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

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.NodeId, in.Name)
	if err != nil {
		return &output, err
	}

	output.NodeId = in.NodeId
	output.Id = item.ID
	output.Name = in.Name

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

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if in.Value == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// node
	node, err := s.cs.GetNode().ViewFromCacheByID(ctx, in.NodeId)
	if err != nil {
		return &output, err
	}

	if node.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
	}

	// name
	wireName := consts.DEFAULT_WIRE
	itemName := in.Name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		wireName = splits[0]
		itemName = splits[1]
	}

	// wire
	wire, err := s.cs.GetWire().ViewFromCacheByNodeIDAndName(ctx, node.ID, wireName)
	if err != nil {
		return &output, err
	}

	// pin
	item, err := s.ViewFromCacheByWireIDAndName(ctx, wire.ID, itemName)
	if err != nil {
		return &output, err
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	if err = s.setPinValueUpdated(ctx, &item, in.Value, time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateValue(ctx, &item, in.Value); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) getPinValue(ctx context.Context, id string) (string, error) {
	item, err := s.getPinValueUpdated(ctx, id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return "", nil
			}
		}

		return "", err
	}

	return item.Value, nil
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	item, err := s.getPinValueUpdated(ctx, in.Id)
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	item, err := s.getPinValueUpdated(ctx, in.Id)
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

func (s *PinService) PushValue(stream grpc.ClientStreamingServer[pb.PinValue, pb.MyBool]) error {
	var output pb.MyBool
	ctx := stream.Context()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&output)
		}
		if err != nil {
			return err
		}

		// basic validation
		if in.Id == "" {
			return status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		// get pin
		item, err := s.ViewByID(ctx, in.Id)
		if err != nil {
			return err
		}

		// insert pin value
		item2 := model.PinValue{
			ID:      item.ID,
			NodeID:  item.NodeID,
			WireID:  item.WireID,
			Value:   in.Value,
			Updated: time.UnixMicro(in.Updated),
		}

		_, err = s.cs.GetDB().NewInsert().Model(&item2).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}
}

func (s *PinService) setPinValueUpdated(ctx context.Context, item *model.Pin, value string, updated time.Time) error {
	var err error

	item2 := model.PinValue{
		ID:      item.ID,
		NodeID:  item.NodeID,
		WireID:  item.WireID,
		Value:   value,
		Updated: updated,
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	output.Id = in.Id

	item, err := s.getPinWriteUpdated(ctx, in.Id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return &output, nil
			}
		}

		return &output, err
	}

	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()

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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		if in.Value == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// pin
	item, err := s.ViewFromCacheByID(ctx, in.Id)
	if err != nil {
		return &output, err
	}

	if item.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	if err = s.setPinWriteUpdated(ctx, &item, in.Value, time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateWrite(ctx, &item, in.Value); err != nil {
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

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}
	}

	item, err := s.ViewFromCacheByNodeIDAndName(ctx, in.NodeId, in.Name)
	if err != nil {
		return &output, err
	}

	output.NodeId = in.NodeId
	output.Id = item.ID
	output.Name = in.Name

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

		if in.NodeId == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		if in.Value == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value")
		}
	}

	// node
	node, err := s.cs.GetNode().ViewFromCacheByID(ctx, in.NodeId)
	if err != nil {
		return &output, err
	}

	if node.Status != consts.ON {
		return &output, status.Errorf(codes.FailedPrecondition, "Node.Status != ON")
	}

	// name
	wireName := consts.DEFAULT_WIRE
	itemName := in.Name

	if strings.Contains(itemName, ".") {
		splits := strings.Split(itemName, ".")
		if len(splits) != 2 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Name")
		}

		wireName = splits[0]
		itemName = splits[1]
	}

	// wire
	wire, err := s.cs.GetWire().ViewFromCacheByNodeIDAndName(ctx, node.ID, wireName)
	if err != nil {
		return &output, err
	}

	// pin
	item, err := s.ViewFromCacheByWireIDAndName(ctx, wire.ID, itemName)
	if err != nil {
		return &output, err
	}

	if item.Rw != consts.WRITE {
		return &output, status.Errorf(codes.FailedPrecondition, "Pin.Rw != WRITE")
	}

	if !dt.ValidateValue(in.Value, item.Type) {
		return &output, status.Errorf(codes.InvalidArgument, "Please supply valid Pin.Value")
	}

	if err = s.setPinWriteUpdated(ctx, &item, in.Value, time.Now()); err != nil {
		return &output, err
	}

	if err = s.afterUpdateWrite(ctx, &item, in.Value); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *PinService) getPinWrite(ctx context.Context, id string) (string, error) {
	item, err := s.getPinWriteUpdated(ctx, id)
	if err != nil {
		if code, ok := status.FromError(err); ok {
			if code.Code() == codes.NotFound {
				return "", nil
			}
		}

		return "", err
	}

	return item.Value, nil
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	item, err := s.getPinWriteUpdated(ctx, in.Id)
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

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}
	}

	item, err := s.getPinWriteUpdated(ctx, in.Id)
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

func (s *PinService) PullWrite(in *cores.PinPullWriteRequest, stream grpc.ServerStreamingServer[pb.PinValueUpdated]) error {
	ctx := stream.Context()

	// basic validation
	if in == nil {
		return status.Error(codes.InvalidArgument, "Please supply valid argument")
	}

	items := make([]model.PinWrite, 0, 10)

	query := s.cs.GetDB().NewSelect().Model(&items)

	if in.NodeId != "" {
		query.Where("node_id = ?", in.NodeId)
	}

	if in.WireId != "" {
		query.Where("wire_id = ?", in.WireId)
	}

	err := query.Where("updated > ?", time.UnixMicro(in.After)).Order("updated ASC").Limit(int(in.Limit)).Scan(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Query: %v", err)
	}

	for i := range items {
		item := pb.PinValueUpdated{}
		s.copyModelToOutputPinWrite(&item, &items[i])

		if err := stream.Send(&item); err != nil {
			return err
		}
	}

	return nil
}

func (s *PinService) setPinWriteUpdated(ctx context.Context, item *model.Pin, value string, updated time.Time) error {
	var err error

	item2 := model.PinWrite{
		ID:      item.ID,
		NodeID:  item.NodeID,
		WireID:  item.WireID,
		Value:   value,
		Updated: updated,
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
	output.WireId = item.WireID
	output.Value = item.Value
	output.Updated = item.Updated.UnixMicro()
}

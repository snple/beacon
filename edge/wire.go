package edge

import (
	"context"
	"database/sql"
	"time"

	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"github.com/snple/types/cache"
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

	query := s.es.GetDB().NewSelect().Model(&items).Order("id ASC")

	if in.Source != "" {
		query = query.Where(`source = ?`, in.Source)
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
	output.Updated = item.Updated.UnixMicro()
}

func (s *WireService) afterUpdate(ctx context.Context, _ *model.Wire) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	return nil
}

func (s *WireService) afterDelete(ctx context.Context, _ *model.Wire) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
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

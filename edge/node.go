package edge

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"github.com/snple/types/cache"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeService struct {
	es *EdgeService

	cache *cache.Value[model.Node]
	lock  sync.RWMutex

	edges.UnimplementedNodeServiceServer
}

func newNodeService(es *EdgeService) *NodeService {
	return &NodeService{
		es: es,
	}
}

/*
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

			err = s.es.GetDB().NewSelect().Model(&model.Node{}).Where("name = ?", in.Name).Scan(ctx)
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
			Desc:    in.Desc,
			Tags:    in.Tags,
			Config:  in.Config,
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
*/

func (s *NodeService) Update(ctx context.Context, in *pb.Node) (*pb.Node, error) {
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
	}

	item, err := s.ViewByID(ctx)
	if err != nil {
		return &output, err
	}

	item.Name = in.Name
	item.Updated = time.Now()

	_, err = s.es.GetDB().NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return &output, status.Errorf(codes.Internal, "Update: %v", err)
	}

	// update node id
	if len(in.Id) > 0 && in.Id != item.ID {
		_, err = s.es.GetDB().NewUpdate().Model(&item).Set("id = ?", in.Id).WherePK().Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Update: %v", err)
		}
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) View(ctx context.Context, in *pb.MyEmpty) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	item, err := s.ViewByID(ctx)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) Destory(ctx context.Context, in *pb.MyEmpty) (*pb.MyBool, error) {
	var err error
	var output pb.MyBool

	err = func() error {
		models := []any{
			(*model.Wire)(nil),
			(*model.Pin)(nil),
		}

		tx, err := s.es.GetDB().BeginTx(ctx, nil)
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
			_, err = tx.NewDelete().Model(model).Where("1 = 1").ForceDelete().Exec(ctx)
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

func (s *NodeService) ViewByID(ctx context.Context) (model.Node, error) {
	item := model.Node{}

	err := s.es.GetDB().NewSelect().Model(&item).Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v", err)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
}

func (s *NodeService) copyModelToOutput(output *pb.Node, item *model.Node) {
	output.Id = item.ID
	output.Name = item.Name
	output.Created = item.Created.UnixMicro()
	output.Updated = item.Updated.UnixMicro()
	output.Deleted = item.Updated.UnixMicro()
}

func (s *NodeService) afterUpdate(ctx context.Context, _ *model.Node) error {
	var err error

	err = s.es.GetSync().setNodeUpdated(ctx, time.Now())
	if err != nil {
		return status.Errorf(codes.Internal, "Sync.setNodeUpdated: %v", err)
	}

	return nil
}

func (s *NodeService) ViewWithDeleted(ctx context.Context, in *pb.MyEmpty) (*pb.Node, error) {
	var output pb.Node
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	item, err := s.viewWithDeleted(ctx)
	if err != nil {
		return &output, err
	}

	s.copyModelToOutput(&output, &item)

	return &output, nil
}

func (s *NodeService) viewWithDeleted(ctx context.Context) (model.Node, error) {
	item := model.Node{}

	err := s.es.GetDB().NewSelect().Model(&item).WhereAllWithDeleted().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			return item, status.Errorf(codes.NotFound, "Query: %v, Node.ID: %v", err, item.ID)
		}

		return item, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item, nil
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
			return &output, status.Error(codes.InvalidArgument, "Please supply valid NodeId")
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

	item, err := s.viewWithDeleted(ctx)
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

			err = s.es.GetDB().NewSelect().Model(&model.Node{}).Where("name = ?", in.Name).Scan(ctx)
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
				return &output, status.Error(codes.InvalidArgument, "Node.Name min 2 character")
			}

			modelItem := model.Node{}
			err = s.es.GetDB().NewSelect().Model(&modelItem).Where("name = ?", in.Name).Scan(ctx)
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
		item.Updated = time.UnixMicro(in.Updated)
		item.Deleted = time.UnixMicro(in.Deleted)

		_, err = s.es.GetDB().NewUpdate().Model(&item).WherePK().WhereAllWithDeleted().Exec(ctx)
		if err != nil {
			return &output, status.Errorf(codes.Internal, "Update: %v", err)
		}

		// update node id
		if in.Id != "" && in.Id != item.ID {
			_, err = s.es.GetDB().NewUpdate().Model(&item).Set("id = ?", in.Id).WherePK().WhereAllWithDeleted().Exec(ctx)
			if err != nil {
				return &output, status.Errorf(codes.Internal, "Update: %v", err)
			}
		}
	}

	if err = s.afterUpdate(ctx, &item); err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

// cache

func (s *NodeService) ViewFromCacheByID(ctx context.Context) (model.Node, error) {
	if !s.es.dopts.cache {
		return s.ViewByID(ctx)
	}

	s.lock.RLock()
	if s.cache.Alive() {
		s.lock.RUnlock()
		return s.cache.Data, nil
	}
	s.lock.RUnlock()

	item, err := s.ViewByID(ctx)
	if err != nil {
		return item, err
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	value := cache.NewValue(item, s.es.dopts.cacheTTL)
	s.cache = &value

	return item, nil
}

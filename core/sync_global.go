package core

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	"github.com/uptrace/bun"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SyncGlobalService struct {
	cs *CoreService

	lock  sync.RWMutex
	waits map[string]map[chan struct{}]struct{}

	cores.UnimplementedSyncGlobalServiceServer
}

func newSyncGlobalService(cs *CoreService) *SyncGlobalService {
	return &SyncGlobalService{
		cs:    cs,
		waits: make(map[string]map[chan struct{}]struct{}),
	}
}

func (s *SyncGlobalService) SetUpdated(ctx context.Context, in *cores.SyncGlobalUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Name")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Updated")
		}
	}

	err = s.setUpdated(ctx, s.cs.GetDB(), in.Name, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncGlobalService) GetUpdated(ctx context.Context, in *pb.Name) (*cores.SyncGlobalUpdated, error) {
	var output cores.SyncGlobalUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Name")
		}
	}

	output.Name = in.Name

	t, err := s.getUpdated(ctx, s.cs.GetDB(), in.Name)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncGlobalService) WaitUpdated(in *pb.Name,
	stream cores.SyncGlobalService_WaitUpdatedServer) error {

	return s.waitUpdated(in, stream)
}

func (s *SyncGlobalService) getUpdated(ctx context.Context, db bun.IDB, name string) (time.Time, error) {
	item := model.SyncGlobal{
		Name: name,
	}

	err := db.NewSelect().Model(&item).WherePK().Scan(ctx)
	if err != nil {
		if err == sql.ErrNoRows {
			_, err = db.NewInsert().Model(&item).Exec(ctx)
			if err != nil {
				return time.Time{}, status.Errorf(codes.Internal, "Insert: %v", err)
			}

			return time.Time{}, nil
		}

		return time.Time{}, status.Errorf(codes.Internal, "Query: %v", err)
	}

	return item.Updated, nil
}

func (s *SyncGlobalService) setUpdated(ctx context.Context, db bun.IDB, name string, updated time.Time) error {
	item := model.SyncGlobal{
		Name:    name,
		Updated: updated,
	}

	ret, err := db.NewUpdate().Model(&item).WherePK().Exec(ctx)
	if err != nil {
		return status.Errorf(codes.Internal, "Update: %v", err)
	}

	n, err := ret.RowsAffected()
	if err != nil {
		return status.Errorf(codes.Internal, "RowsAffected: %v", err)
	}

	if n < 1 {
		_, err = db.NewInsert().Model(&item).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Insert: %v", err)
		}
	}

	s.notifyUpdated(name)

	return nil
}

func (s *SyncGlobalService) notifyUpdated(name string) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	if waits, ok := s.waits[name]; ok {
		for wait := range waits {
			select {
			case wait <- struct{}{}:
			default:
			}
		}
	}
}

func (s *SyncGlobalService) Notify(name string) *GlobalNotify {
	ch := make(chan struct{}, 1)

	s.lock.Lock()
	if waits, ok := s.waits[name]; ok {
		waits[ch] = struct{}{}
	} else {
		waits := map[chan struct{}]struct{}{
			ch: {},
		}
		s.waits[name] = waits
	}
	s.lock.Unlock()

	n := &GlobalNotify{
		name,
		s,
		ch,
	}

	n.notify()

	return n
}

type GlobalNotify struct {
	name string
	ss   *SyncGlobalService
	ch   chan struct{}
}

func (n *GlobalNotify) notify() {
	select {
	case n.ch <- struct{}{}:
	default:
	}
}

func (w *GlobalNotify) Wait() <-chan struct{} {
	return w.ch
}

func (n *GlobalNotify) Close() {
	n.ss.lock.Lock()
	defer n.ss.lock.Unlock()

	if waits, ok := n.ss.waits[n.name]; ok {
		delete(waits, n.ch)

		if len(waits) == 0 {
			delete(n.ss.waits, n.name)
		}
	}
}

func (n *GlobalNotify) Name() string {
	return n.name
}

type globalWaitUpdatedStream interface {
	Send(*pb.MyBool) error
	grpc.ServerStream
}

func (s *SyncGlobalService) waitUpdated(in *pb.Name, stream globalWaitUpdatedStream) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Name == "" {
			return status.Error(codes.InvalidArgument, "Please supply valid Name")
		}
	}

	notify := s.Notify(in.Name)
	defer notify.Close()

	for {
		select {
		case <-notify.Wait():
			err = stream.Send(&pb.MyBool{Bool: true})
			if err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

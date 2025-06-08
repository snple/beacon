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

type SyncService struct {
	cs *CoreService

	lock    sync.RWMutex
	waits   map[string]map[chan struct{}]struct{}
	waitsPV map[string]map[chan struct{}]struct{}
	waitsPW map[string]map[chan struct{}]struct{}

	cores.UnimplementedSyncServiceServer
}

func newSyncService(cs *CoreService) *SyncService {
	return &SyncService{
		cs:      cs,
		waits:   make(map[string]map[chan struct{}]struct{}),
		waitsPV: make(map[string]map[chan struct{}]struct{}),
		waitsPW: make(map[string]map[chan struct{}]struct{}),
	}
}

func (s *SyncService) SetNodeUpdated(ctx context.Context, in *cores.SyncUpdated) (*pb.MyBool, error) {
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

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Updated")
		}
	}

	err = s.setNodeUpdated(ctx, s.cs.GetDB(), in.Id, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetNodeUpdated(ctx context.Context, in *pb.Id) (*cores.SyncUpdated, error) {
	var output cores.SyncUpdated
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

	output.Id = in.Id

	t, err := s.getNodeUpdated(ctx, s.cs.GetDB(), in.Id)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitNodeUpdated(in *pb.Id,
	stream cores.SyncService_WaitNodeUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY)
}

func (s *SyncService) SetPinValueUpdated(ctx context.Context, in *cores.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Tag.Value.Updated")
		}
	}

	err = s.setPinValueUpdated(ctx, s.cs.GetDB(), in.Id, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context, in *pb.Id) (*cores.SyncUpdated, error) {
	var output cores.SyncUpdated
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

	output.Id = in.Id

	t, err := s.getPinValueUpdated(ctx, s.cs.GetDB(), in.Id)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitPinValueUpdated(in *pb.Id,
	stream cores.SyncService_WaitPinValueUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY_PV)
}

func (s *SyncService) SetPinWriteUpdated(ctx context.Context, in *cores.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Id")
		}

		if in.Updated == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Write.Updated")
		}
	}

	err = s.setPinWriteUpdated(ctx, s.cs.GetDB(), in.Id, time.UnixMicro(in.Updated))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context, in *pb.Id) (*cores.SyncUpdated, error) {
	var output cores.SyncUpdated
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

	output.Id = in.Id

	t, err := s.getPinWriteUpdated(ctx, s.cs.GetDB(), in.Id)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitPinWriteUpdated(in *pb.Id,
	stream cores.SyncService_WaitPinWriteUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY_PW)
}

func (s *SyncService) getNodeUpdated(ctx context.Context, db bun.IDB, id string) (time.Time, error) {
	return s.getUpdated(ctx, db, id+model.SYNC_NODE_SUFFIX)
}

func (s *SyncService) setNodeUpdated(ctx context.Context, db bun.IDB, id string, updated time.Time) error {
	err := s.setUpdated(ctx, db, id+model.SYNC_NODE_SUFFIX, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(id, NOTIFY)

	return nil
}

func (s *SyncService) getPinValueUpdated(ctx context.Context, db bun.IDB, id string) (time.Time, error) {
	return s.getUpdated(ctx, db, id+model.SYNC_PIN_VALUE_SUFFIX)
}

func (s *SyncService) setPinValueUpdated(ctx context.Context, db bun.IDB, id string, updated time.Time) error {
	err := s.setUpdated(ctx, db, id+model.SYNC_PIN_VALUE_SUFFIX, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(id, NOTIFY_PV)

	return nil
}

func (s *SyncService) getPinWriteUpdated(ctx context.Context, db bun.IDB, id string) (time.Time, error) {
	return s.getUpdated(ctx, db, id+model.SYNC_PIN_WRITE_SUFFIX)
}

func (s *SyncService) setPinWriteUpdated(ctx context.Context, db bun.IDB, id string, updated time.Time) error {
	err := s.setUpdated(ctx, db, id+model.SYNC_PIN_WRITE_SUFFIX, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(id, NOTIFY_PW)

	return nil
}

func (s *SyncService) getUpdated(ctx context.Context, db bun.IDB, id string) (time.Time, error) {
	item := model.Sync{
		ID: id,
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

func (s *SyncService) setUpdated(ctx context.Context, db bun.IDB, id string, updated time.Time) error {
	item := model.Sync{
		ID:      id,
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

	return nil
}

type NotifyType int

const (
	NOTIFY    NotifyType = 0
	NOTIFY_PV NotifyType = 1
	NOTIFY_PW NotifyType = 2
)

func (s *SyncService) notifyUpdated(id string, nt NotifyType) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	switch nt {
	case NOTIFY:
		if waits, ok := s.waits[id]; ok {
			for wait := range waits {
				select {
				case wait <- struct{}{}:
				default:
				}
			}
		}
	case NOTIFY_PV:
		if waits, ok := s.waitsPV[id]; ok {
			for wait := range waits {
				select {
				case wait <- struct{}{}:
				default:
				}
			}
		}
	case NOTIFY_PW:
		if waits, ok := s.waitsPW[id]; ok {
			for wait := range waits {
				select {
				case wait <- struct{}{}:
				default:
				}
			}
		}
	}
}

func (s *SyncService) Notify(id string, nt NotifyType) *Notify {
	ch := make(chan struct{}, 1)

	s.lock.Lock()

	switch nt {
	case NOTIFY:
		if waits, ok := s.waits[id]; ok {
			waits[ch] = struct{}{}
		} else {
			waits := map[chan struct{}]struct{}{
				ch: {},
			}
			s.waits[id] = waits
		}
	case NOTIFY_PV:
		if waits, ok := s.waitsPV[id]; ok {
			waits[ch] = struct{}{}
		} else {
			waits := map[chan struct{}]struct{}{
				ch: {},
			}
			s.waitsPV[id] = waits
		}
	case NOTIFY_PW:
		if waits, ok := s.waitsPW[id]; ok {
			waits[ch] = struct{}{}
		} else {
			waits := map[chan struct{}]struct{}{
				ch: {},
			}
			s.waitsPW[id] = waits
		}
	}

	s.lock.Unlock()

	n := &Notify{
		id,
		s,
		ch,
		nt,
	}

	n.notify()

	return n
}

type Notify struct {
	id string
	ss *SyncService
	ch chan struct{}
	nt NotifyType
}

func (n *Notify) notify() {
	select {
	case n.ch <- struct{}{}:
	default:
	}
}

func (w *Notify) Wait() <-chan struct{} {
	return w.ch
}

func (n *Notify) Close() {
	n.ss.lock.Lock()
	defer n.ss.lock.Unlock()

	switch n.nt {
	case NOTIFY:
		if waits, ok := n.ss.waits[n.id]; ok {
			delete(waits, n.ch)

			if len(waits) == 0 {
				delete(n.ss.waits, n.id)
			}
		}
	case NOTIFY_PV:
		if waits, ok := n.ss.waitsPV[n.id]; ok {
			delete(waits, n.ch)

			if len(waits) == 0 {
				delete(n.ss.waitsPV, n.id)
			}
		}
	case NOTIFY_PW:
		if waits, ok := n.ss.waitsPW[n.id]; ok {
			delete(waits, n.ch)

			if len(waits) == 0 {
				delete(n.ss.waitsPW, n.id)
			}
		}
	}
}

func (n *Notify) Id() string {
	return n.id
}

type waitUpdatedStream interface {
	Send(*pb.MyBool) error
	grpc.ServerStream
}

func (s *SyncService) waitUpdated(in *pb.Id, stream waitUpdatedStream, nt NotifyType) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.Id == "" {
			return status.Error(codes.InvalidArgument, "Please supply valid NodeId")
		}
	}

	notify := s.Notify(in.Id, nt)
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

func (s *SyncService) destory(ctx context.Context, db bun.IDB, id string) error {
	ids := []string{id, id + model.SYNC_PIN_VALUE_SUFFIX, id + model.SYNC_PIN_WRITE_SUFFIX}

	for _, id := range ids {
		_, err := db.NewDelete().Model(&model.Sync{}).Where("id = ?", id).Exec(ctx)
		if err != nil {
			return status.Errorf(codes.Internal, "Delete: %v", err)
		}
	}

	return nil
}

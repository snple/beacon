package edge

import (
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/edge/model"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/edges"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SyncService struct {
	es *EdgeService

	lock    sync.RWMutex
	waits   map[chan struct{}]struct{}
	waitsPV map[chan struct{}]struct{}
	waitsPW map[chan struct{}]struct{}

	edges.UnimplementedSyncServiceServer
}

func newSyncService(es *EdgeService) *SyncService {
	return &SyncService{
		es:      es,
		waits:   make(map[chan struct{}]struct{}),
		waitsPV: make(map[chan struct{}]struct{}),
		waitsPW: make(map[chan struct{}]struct{}),
	}
}

func (s *SyncService) SetNodeUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Node.Updated")
		}
	}

	err = s.setNodeUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetNodeUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getNodeUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitNodeUpdated(in *pb.MyEmpty,
	stream edges.SyncService_WaitNodeUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY)
}

func (s *SyncService) SetWireUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Source.Updated")
		}
	}

	err = s.setWireUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetWireUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getWireUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) SetPinUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Updated")
		}
	}

	err = s.setPinUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetPinUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getPinUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) SetConstUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Const.Updated")
		}
	}

	err = s.setConstUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetConstUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getConstUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) SetPinValueUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Value.Updated")
		}
	}

	err = s.setPinValueUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getPinValueUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitPinValueUpdated(in *pb.MyEmpty,
	stream edges.SyncService_WaitPinValueUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY_PV)
}

func (s *SyncService) SetPinWriteUpdated(ctx context.Context, in *edges.SyncUpdated) (*pb.MyBool, error) {
	var output pb.MyBool
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}

		if in.GetUpdated() == 0 {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid Pin.Write.Updated")
		}
	}

	err = s.setPinWriteUpdated(ctx, time.UnixMicro(in.GetUpdated()))
	if err != nil {
		return &output, err
	}

	output.Bool = true

	return &output, nil
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context, in *pb.MyEmpty) (*edges.SyncUpdated, error) {
	var output edges.SyncUpdated
	var err error

	// basic validation
	{
		if in == nil {
			return &output, status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	t, err := s.getPinWriteUpdated(ctx)
	if err != nil {
		return &output, err
	}

	output.Updated = t.UnixMicro()

	return &output, nil
}

func (s *SyncService) WaitPinWriteUpdated(in *pb.MyEmpty,
	stream edges.SyncService_WaitPinWriteUpdatedServer) error {

	return s.waitUpdated(in, stream, NOTIFY_PW)
}

func (s *SyncService) getNodeUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_NODE)
}

func (s *SyncService) setNodeUpdated(ctx context.Context, updated time.Time) error {
	err := s.setUpdated(ctx, model.SYNC_NODE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY)

	return nil
}

func (s *SyncService) getWireUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_WIRE)
}

func (s *SyncService) setWireUpdated(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_WIRE, updated)
}

func (s *SyncService) getPinUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN)
}

func (s *SyncService) setPinUpdated(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_PIN, updated)
}

func (s *SyncService) getConstUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_CONST)
}

func (s *SyncService) setConstUpdated(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_CONST, updated)
}

func (s *SyncService) getPinValueUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_VALUE)
}

func (s *SyncService) setPinValueUpdated(ctx context.Context, updated time.Time) error {
	err := s.setUpdated(ctx, model.SYNC_PIN_VALUE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY_PV)

	return nil
}

func (s *SyncService) getPinWriteUpdated(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_WRITE)
}

func (s *SyncService) setPinWriteUpdated(ctx context.Context, updated time.Time) error {
	err := s.setUpdated(ctx, model.SYNC_PIN_WRITE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY_PW)

	return nil
}

// node

func (s *SyncService) getNodeUpdatedRemoteToLocal(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_NODE_REMOTE_TO_LOCAL)
}

func (s *SyncService) setNodeUpdatedRemoteToLocal(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_NODE_REMOTE_TO_LOCAL, updated)
}

func (s *SyncService) getNodeUpdatedLocalToRemote(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_NODE_LOCAL_TO_REMOTE)
}

func (s *SyncService) setNodeUpdatedLocalToRemote(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_NODE_LOCAL_TO_REMOTE, updated)
}

// value

func (s *SyncService) getPinValueUpdatedRemoteToLocal(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_VALUE_REMOTE_TO_LOCAL)
}

func (s *SyncService) setPinValueUpdatedRemoteToLocal(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_PIN_VALUE_REMOTE_TO_LOCAL, updated)
}

func (s *SyncService) getPinValueUpdatedLocalToRemote(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_VALUE_LOCAL_TO_REMOTE)
}

func (s *SyncService) setPinValueUpdatedLocalToRemote(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_PIN_VALUE_LOCAL_TO_REMOTE, updated)
}

// write

func (s *SyncService) getPinWriteUpdatedRemoteToLocal(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_WRITE_REMOTE_TO_LOCAL)
}

func (s *SyncService) setPinWriteUpdatedRemoteToLocal(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_PIN_WRITE_REMOTE_TO_LOCAL, updated)
}

func (s *SyncService) getPinWriteUpdatedLocalToRemote(ctx context.Context) (time.Time, error) {
	return s.getUpdated(ctx, model.SYNC_PIN_WRITE_LOCAL_TO_REMOTE)
}

func (s *SyncService) setPinWriteUpdatedLocalToRemote(ctx context.Context, updated time.Time) error {
	return s.setUpdated(ctx, model.SYNC_PIN_WRITE_LOCAL_TO_REMOTE, updated)
}

func (s *SyncService) getUpdated(_ context.Context, key string) (time.Time, error) {
	txn := s.es.GetBadgerDB().NewTransactionAt(uint64(time.Now().UnixMicro()), false)
	defer txn.Discard()

	dbitem, err := txn.Get([]byte(key))
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, status.Errorf(codes.Internal, "BadgerDB Get: %v", err)
	}

	return time.UnixMicro(int64(dbitem.Version())), nil
}

func (s *SyncService) setUpdated(_ context.Context, key string, updated time.Time) error {
	ts := uint64(updated.UnixMicro())

	txn := s.es.GetBadgerDB().NewTransactionAt(ts, true)
	defer txn.Discard()

	err := txn.Set([]byte(key), []byte{})
	if err != nil {
		return status.Errorf(codes.Internal, "BadgerDB Set: %v", err)
	}

	err = txn.CommitAt(ts, nil)
	if err != nil {
		return status.Errorf(codes.Internal, "BadgerDB CommitAt: %v", err)
	}

	return nil
}

type NotifyType int

const (
	NOTIFY    NotifyType = 0
	NOTIFY_PV NotifyType = 1
	NOTIFY_PW NotifyType = 2
)

func (s *SyncService) notifyUpdated(nt NotifyType) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	switch nt {
	case NOTIFY:
		for wait := range s.waits {
			select {
			case wait <- struct{}{}:
			default:
			}
		}
	case NOTIFY_PV:
		for wait := range s.waitsPV {
			select {
			case wait <- struct{}{}:
			default:
			}
		}
	case NOTIFY_PW:
		for wait := range s.waitsPW {
			select {
			case wait <- struct{}{}:
			default:
			}
		}
	}
}

func (s *SyncService) Notify(nt NotifyType) *Notify {
	ch := make(chan struct{}, 1)

	s.lock.Lock()
	switch nt {
	case NOTIFY:
		s.waits[ch] = struct{}{}
	case NOTIFY_PV:
		s.waitsPV[ch] = struct{}{}
	case NOTIFY_PW:
		s.waitsPW[ch] = struct{}{}
	}
	s.lock.Unlock()

	n := &Notify{
		s,
		ch,
		nt,
	}

	n.notify()

	return n
}

type Notify struct {
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
		delete(n.ss.waits, n.ch)
	case NOTIFY_PV:
		delete(n.ss.waitsPV, n.ch)
	case NOTIFY_PW:
		delete(n.ss.waitsPW, n.ch)
	}
}

type waitUpdatedStream interface {
	Send(*pb.MyBool) error
	grpc.ServerStream
}

func (s *SyncService) waitUpdated(in *pb.MyEmpty, stream waitUpdatedStream, nt NotifyType) error {
	var err error

	// basic validation
	{
		if in == nil {
			return status.Error(codes.InvalidArgument, "Please supply valid argument")
		}
	}

	notify := s.Notify(nt)
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

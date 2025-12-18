package edge

import (
	"context"
	"sync"
	"time"

	"github.com/snple/beacon/edge/storage"
)

type SyncService struct {
	es *EdgeService

	lock    sync.RWMutex
	waits   map[chan struct{}]struct{}
	waitsPV map[chan struct{}]struct{}
	waitsPW map[chan struct{}]struct{}
}

func newSyncService(es *EdgeService) *SyncService {
	return &SyncService{
		es:      es,
		waits:   make(map[chan struct{}]struct{}),
		waitsPV: make(map[chan struct{}]struct{}),
		waitsPW: make(map[chan struct{}]struct{}),
	}
}

func (s *SyncService) SetNodeUpdated(ctx context.Context, updated time.Time) error {
	return s.setNodeUpdated(ctx, updated)
}

func (s *SyncService) GetNodeUpdated(ctx context.Context) (time.Time, error) {
	return s.getNodeUpdated(ctx)
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context) (time.Time, error) {
	return s.getPinValueUpdated(ctx)
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context) (time.Time, error) {
	return s.getPinWriteUpdated(ctx)
}

func (s *SyncService) getNodeUpdated(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_NODE)
}

func (s *SyncService) setNodeUpdated(ctx context.Context, updated time.Time) error {
	err := s.es.GetStorage().SetSyncTime(storage.SYNC_NODE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY)

	return nil
}

func (s *SyncService) getPinValueUpdated(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_PIN_VALUE)
}

func (s *SyncService) setPinValueUpdated(ctx context.Context, updated time.Time) error {
	err := s.es.GetStorage().SetSyncTime(storage.SYNC_PIN_VALUE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY_PV)

	return nil
}

func (s *SyncService) getPinWriteUpdated(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_PIN_WRITE)
}

func (s *SyncService) setPinWriteUpdated(ctx context.Context, updated time.Time) error {
	err := s.es.GetStorage().SetSyncTime(storage.SYNC_PIN_WRITE, updated)
	if err != nil {
		return err
	}

	s.notifyUpdated(NOTIFY_PW)

	return nil
}

// 单向同步状态跟踪方法

// 配置数据同步状态 (Edge → Core)
func (s *SyncService) getNodeToRemote(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_NODE_TO_REMOTE)
}

func (s *SyncService) setNodeToRemote(ctx context.Context, updated time.Time) error {
	return s.es.GetStorage().SetSyncTime(storage.SYNC_NODE_TO_REMOTE, updated)
}

// PinValue 同步状态 (Edge → Core)
func (s *SyncService) getPinValueToRemote(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_PIN_VALUE_TO_REMOTE)
}

func (s *SyncService) setPinValueToRemote(ctx context.Context, updated time.Time) error {
	return s.es.GetStorage().SetSyncTime(storage.SYNC_PIN_VALUE_TO_REMOTE, updated)
}

// PinWrite 同步状态 (Core → Edge)
func (s *SyncService) getPinWriteFromRemote(ctx context.Context) (time.Time, error) {
	return s.es.GetStorage().GetSyncTime(storage.SYNC_PIN_WRITE_FROM_REMOTE)
}

func (s *SyncService) setPinWriteFromRemote(ctx context.Context, updated time.Time) error {
	return s.es.GetStorage().SetSyncTime(storage.SYNC_PIN_WRITE_FROM_REMOTE, updated)
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

package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type SyncService struct {
	cs *CoreService

	lock    sync.RWMutex
	waits   map[string]map[chan struct{}]struct{}
	waitsPV map[string]map[chan struct{}]struct{}
	waitsPW map[string]map[chan struct{}]struct{}

	// 更新时间缓存
	nodeUpdated     map[string]time.Time
	pinValueUpdated map[string]time.Time
	pinWriteUpdated map[string]time.Time
}

func newSyncService(cs *CoreService) *SyncService {
	return &SyncService{
		cs:              cs,
		waits:           make(map[string]map[chan struct{}]struct{}),
		waitsPV:         make(map[string]map[chan struct{}]struct{}),
		waitsPW:         make(map[string]map[chan struct{}]struct{}),
		nodeUpdated:     make(map[string]time.Time),
		pinValueUpdated: make(map[string]time.Time),
		pinWriteUpdated: make(map[string]time.Time),
	}
}

func (s *SyncService) GetNodeUpdated(ctx context.Context, in *Id) (*SyncUpdated, error) {
	var output SyncUpdated

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	output.NodeId = in.Id

	s.lock.RLock()
	t, ok := s.nodeUpdated[in.Id]
	s.lock.RUnlock()

	if ok {
		output.Updated = t.UnixMicro()
	}

	return &output, nil
}

func (s *SyncService) GetPinValueUpdated(ctx context.Context, in *Id) (*SyncUpdated, error) {
	var output SyncUpdated

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	output.NodeId = in.Id

	s.lock.RLock()
	t, ok := s.pinValueUpdated[in.Id]
	s.lock.RUnlock()

	if ok {
		output.Updated = t.UnixMicro()
	}

	return &output, nil
}

func (s *SyncService) GetPinWriteUpdated(ctx context.Context, in *Id) (*SyncUpdated, error) {
	var output SyncUpdated

	// basic validation
	if in == nil || in.Id == "" {
		return &output, fmt.Errorf("please supply valid Node.ID")
	}

	output.NodeId = in.Id

	s.lock.RLock()
	t, ok := s.pinWriteUpdated[in.Id]
	s.lock.RUnlock()

	if ok {
		output.Updated = t.UnixMicro()
	}

	return &output, nil
}

// 内部方法

func (s *SyncService) setNodeUpdated(nodeID string, updated time.Time) {
	s.lock.Lock()
	s.nodeUpdated[nodeID] = updated
	s.lock.Unlock()

	s.notifyUpdated(nodeID, NOTIFY)
}

func (s *SyncService) setPinValueUpdated(nodeID string, updated time.Time) {
	s.lock.Lock()
	s.pinValueUpdated[nodeID] = updated
	s.lock.Unlock()

	s.notifyUpdated(nodeID, NOTIFY_PV)
}

func (s *SyncService) setPinWriteUpdated(nodeID string, updated time.Time) {
	s.lock.Lock()
	s.pinWriteUpdated[nodeID] = updated
	s.lock.Unlock()

	s.notifyUpdated(nodeID, NOTIFY_PW)
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

func (n *Notify) Wait() <-chan struct{} {
	return n.ch
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

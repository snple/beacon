package core

import (
	"context"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type BadgerService struct {
	cs *CoreService

	badger *badger.DB

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup
}

func newBadgerService(cs *CoreService) (*BadgerService, error) {
	badger, err := badger.OpenManaged(cs.dopts.BadgerOptions)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(cs.Context())

	s := &BadgerService{
		cs:     cs,
		badger: badger,
		ctx:    ctx,
		cancel: cancel,
	}

	return s, nil
}

func (s *BadgerService) start() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	s.cs.Logger().Sugar().Info("badger service started")

	ticker := time.NewTicker(s.cs.dopts.BadgerGCOptions.GC)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			{
			again:
				err := s.badger.RunValueLogGC(s.cs.dopts.BadgerGCOptions.GCDiscardRatio)
				if err == nil {
					goto again
				}
			}
		}
	}
}

func (s *BadgerService) stop() {
	s.cancel()
	s.closeWG.Wait()

	s.badger.Close()
}

func (s *BadgerService) GetDB() *badger.DB {
	return s.badger
}

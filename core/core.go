package core

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb/cores"
	bErrors "github.com/snple/beacon/util/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreService struct {
	storage *storage.Storage

	sync     *SyncService
	node     *NodeService
	wire     *WireService
	pin      *PinService
	pinValue *PinValueService
	pinWrite *PinWriteService

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts coreOptions
}

func Core(db *badger.DB, opts ...CoreOption) (*CoreService, error) {
	return CoreContext(context.Background(), db, opts...)
}

func CoreContext(ctx context.Context, db *badger.DB, opts ...CoreOption) (*CoreService, error) {
	ctx, cancel := context.WithCancel(ctx)

	if db == nil {
		cancel()
		return nil, bErrors.ErrInvalidDatabase
	}

	cs := &CoreService{
		storage: storage.New(db),
		ctx:     ctx,
		cancel:  cancel,
		dopts:   defaultCoreOptions(),
	}

	for _, opt := range extraCoreOptions {
		opt.apply(&cs.dopts)
	}

	for _, opt := range opts {
		opt.apply(&cs.dopts)
	}

	cs.sync = newSyncService(cs)
	cs.node = newNodeService(cs)
	cs.wire = newWireService(cs)
	cs.pin = newPinService(cs)
	cs.pinValue = newPinValueService(cs)
	cs.pinWrite = newPinWriteService(cs)

	return cs, nil
}

func (cs *CoreService) Start() error {
	// 加载存储数据
	return cs.storage.Load(cs.ctx)
}

func (cs *CoreService) Stop() {
	cs.cancel()

	cs.closeWG.Wait()
	cs.dopts.logger.Sync()
}

func (cs *CoreService) GetStorage() *storage.Storage {
	return cs.storage
}

func (cs *CoreService) GetSync() *SyncService {
	return cs.sync
}

func (cs *CoreService) GetNode() *NodeService {
	return cs.node
}

func (cs *CoreService) GetWire() *WireService {
	return cs.wire
}

func (cs *CoreService) GetPin() *PinService {
	return cs.pin
}

func (cs *CoreService) GetPinValue() *PinValueService {
	return cs.pinValue
}

func (cs *CoreService) GetPinWrite() *PinWriteService {
	return cs.pinWrite
}

func (cs *CoreService) Context() context.Context {
	return cs.ctx
}

func (cs *CoreService) Logger() *zap.Logger {
	return cs.dopts.logger
}

func (cs *CoreService) Register(server *grpc.Server) {
	cores.RegisterSyncServiceServer(server, cs.sync)
	cores.RegisterNodeServiceServer(server, cs.node)
	cores.RegisterWireServiceServer(server, cs.wire)
	cores.RegisterPinServiceServer(server, cs.pin)
	cores.RegisterPinValueServiceServer(server, cs.pinValue)
	cores.RegisterPinWriteServiceServer(server, cs.pinWrite)
}

type coreOptions struct {
	logger  *zap.Logger
	linkTTL time.Duration
}

func defaultCoreOptions() coreOptions {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("zap.NewDevelopment(): %v", err)
	}

	return coreOptions{
		logger:  logger,
		linkTTL: 3 * time.Minute,
	}
}

type CoreOption interface {
	apply(*coreOptions)
}

var extraCoreOptions []CoreOption

type funcCoreOption struct {
	f func(*coreOptions)
}

func (fdo *funcCoreOption) apply(do *coreOptions) {
	fdo.f(do)
}

func newFuncCoreOption(f func(*coreOptions)) *funcCoreOption {
	return &funcCoreOption{
		f: f,
	}
}

func WithLogger(logger *zap.Logger) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.logger = logger
	})
}

func WithLinkTTL(d time.Duration) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.linkTTL = d
	})
}

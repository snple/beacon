package core

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb/cores"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreService struct {
	badger  *BadgerService
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

func Core(opts ...CoreOption) (*CoreService, error) {
	return CoreContext(context.Background(), opts...)
}

func CoreContext(ctx context.Context, opts ...CoreOption) (*CoreService, error) {
	ctx, cancel := context.WithCancel(ctx)

	cs := &CoreService{
		ctx:    ctx,
		cancel: cancel,
		dopts:  defaultCoreOptions(),
	}

	for _, opt := range extraCoreOptions {
		opt.apply(&cs.dopts)
	}

	for _, opt := range opts {
		opt.apply(&cs.dopts)
	}

	if err := cs.dopts.check(); err != nil {
		cancel()
		return nil, err
	}

	badgerSvc, err := newBadgerService(cs)
	if err != nil {
		cancel()
		return nil, err
	}
	cs.badger = badgerSvc

	// 创建存储
	cs.storage = storage.New(badgerSvc.GetDB())

	cs.sync = newSyncService(cs)
	cs.node = newNodeService(cs)
	cs.wire = newWireService(cs)
	cs.pin = newPinService(cs)
	cs.pinValue = newPinValueService(cs)
	cs.pinWrite = newPinWriteService(cs)

	return cs, nil
}

func (cs *CoreService) Start() {
	// 加载存储数据
	if err := cs.storage.Load(cs.ctx); err != nil {
		cs.Logger().Sugar().Errorf("load storage: %v", err)
	}

	cs.closeWG.Add(1)
	go func() {
		defer cs.closeWG.Done()

		cs.badger.start()
	}()
}

func (cs *CoreService) Stop() {
	cs.badger.stop()

	cs.cancel()
	cs.closeWG.Wait()
	cs.dopts.logger.Sync()
}

func (cs *CoreService) GetBadgerDB() *badger.DB {
	return cs.badger.GetDB()
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
	logger          *zap.Logger
	linkTTL         time.Duration
	BadgerOptions   badger.Options
	BadgerGCOptions BadgerGCOptions
}

type BadgerGCOptions struct {
	GC             time.Duration
	GCDiscardRatio float64
}

func defaultCoreOptions() coreOptions {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("zap.NewDevelopment(): %v", err)
	}

	return coreOptions{
		logger:        logger,
		linkTTL:       3 * time.Minute,
		BadgerOptions: badger.DefaultOptions("").WithInMemory(true),
		BadgerGCOptions: BadgerGCOptions{
			GC:             time.Hour,
			GCDiscardRatio: 0.7,
		},
	}
}

func (o *coreOptions) check() error {
	return nil
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

func WithBadger(options badger.Options) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.BadgerOptions = options
	})
}

func WithBadgerGC(options *BadgerGCOptions) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		if options.GC > 0 {
			o.BadgerGCOptions.GC = options.GC
		}

		if options.GCDiscardRatio > 0 {
			o.BadgerGCOptions.GCDiscardRatio = options.GCDiscardRatio
		}
	})
}

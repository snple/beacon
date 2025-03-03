package core

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/snple/beacon/core/model"
	"github.com/snple/beacon/pb/cores"
	"github.com/uptrace/bun"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type CoreService struct {
	db *bun.DB

	status      *StatusService
	sync        *SyncService
	sync_global *SyncGlobalService
	node        *NodeService
	slot        *SlotService
	source      *SourceService
	pin         *PinService
	constant    *ConstService

	clone *cloneService

	auth *AuthService
	user *UserService

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts coreOptions
}

func Core(db *bun.DB, opts ...CoreOption) (*CoreService, error) {
	return CoreContext(context.Background(), db, opts...)
}

func CoreContext(ctx context.Context, db *bun.DB, opts ...CoreOption) (*CoreService, error) {
	ctx, cancel := context.WithCancel(ctx)

	if db == nil {
		panic("db == nil")
	}

	cs := &CoreService{
		db:     db,
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

	cs.status = newStatusService(cs)
	cs.sync = newSyncService(cs)
	cs.sync_global = newSyncGlobalService(cs)
	cs.node = newNodeService(cs)
	cs.slot = newSlotService(cs)
	cs.source = newSourceService(cs)
	cs.pin = newPinService(cs)
	cs.constant = newConstService(cs)

	cs.clone = newCloneService(cs)

	cs.auth = newAuthService(cs)
	cs.user = newUserService(cs)

	return cs, nil
}

func (cs *CoreService) Start() {
	if cs.dopts.cache {
		go cs.cacheGC()
	}
}

func (cs *CoreService) Stop() {
	cs.cancel()

	cs.closeWG.Wait()
	cs.dopts.logger.Sync()
}

func (cs *CoreService) GetDB() *bun.DB {
	return cs.db
}

func (cs *CoreService) GetStatus() *StatusService {
	return cs.status
}

func (cs *CoreService) GetSync() *SyncService {
	return cs.sync
}

func (cs *CoreService) GetSyncGlobal() *SyncGlobalService {
	return cs.sync_global
}

func (cs *CoreService) GetNode() *NodeService {
	return cs.node
}

func (cs *CoreService) GetSlot() *SlotService {
	return cs.slot
}

func (cs *CoreService) GetSource() *SourceService {
	return cs.source
}

func (cs *CoreService) GetPin() *PinService {
	return cs.pin
}

func (cs *CoreService) GetConst() *ConstService {
	return cs.constant
}

func (cs *CoreService) getClone() *cloneService {
	return cs.clone
}

func (cs *CoreService) GetAuth() *AuthService {
	return cs.auth
}

func (cs *CoreService) GetUser() *UserService {
	return cs.user
}

func (cs *CoreService) Context() context.Context {
	return cs.ctx
}

func (cs *CoreService) Logger() *zap.Logger {
	return cs.dopts.logger
}

func (cs *CoreService) cacheGC() {
	cs.closeWG.Add(1)
	defer cs.closeWG.Done()

	cs.Logger().Sugar().Info("cache gc started")

	ticker := time.NewTicker(cs.dopts.cacheGCTTL)
	defer ticker.Stop()

	for {
		select {
		case <-cs.ctx.Done():
			return
		case <-ticker.C:
			{
				cs.GetNode().GC()
				cs.GetSource().GC()
				cs.GetPin().GC()
				cs.GetConst().GC()
				cs.GetUser().GC()
			}
		}
	}
}

func (cs *CoreService) Register(server *grpc.Server) {
	cores.RegisterSyncServiceServer(server, cs.sync)
	cores.RegisterSyncGlobalServiceServer(server, cs.sync_global)
	cores.RegisterNodeServiceServer(server, cs.node)
	cores.RegisterSlotServiceServer(server, cs.slot)
	cores.RegisterSourceServiceServer(server, cs.source)
	cores.RegisterPinServiceServer(server, cs.pin)
	cores.RegisterConstServiceServer(server, cs.constant)

	cores.RegisterAuthServiceServer(server, cs.auth)
	cores.RegisterUserServiceServer(server, cs.user)
}

func CreateSchema(db bun.IDB) error {
	models := []any{
		(*model.Sync)(nil),
		(*model.SyncGlobal)(nil),
		(*model.Node)(nil),
		(*model.Slot)(nil),
		(*model.Source)(nil),
		(*model.Pin)(nil),
		(*model.Const)(nil),
		(*model.PinValue)(nil),
		(*model.PinWrite)(nil),
		(*model.User)(nil),
	}

	for _, model := range models {
		_, err := db.NewCreateTable().Model(model).IfNotExists().Exec(context.Background())
		if err != nil {
			return err
		}
	}
	return nil
}

type coreOptions struct {
	logger       *zap.Logger
	linkTTL      time.Duration
	cache        bool
	cacheTTL     time.Duration
	cacheGCTTL   time.Duration
	saveInterval time.Duration
}

func defaultCoreOptions() coreOptions {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("zap.NewDevelopment(): %v", err)
	}

	return coreOptions{
		logger:       logger,
		linkTTL:      3 * time.Minute,
		cache:        true,
		cacheTTL:     3 * time.Second,
		cacheGCTTL:   3 * time.Hour,
		saveInterval: time.Minute,
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

func WithCache(enable bool) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.cache = enable
	})
}

func WithCacheTTL(d time.Duration) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.cacheTTL = d
	})
}

func WithCacheGCTTL(d time.Duration) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.cacheGCTTL = d
	})
}

func WithSaveInterval(d time.Duration) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.saveInterval = d
	})
}

package core

import (
	"context"
	"crypto/tls"
	"log"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core/storage"
	"github.com/snple/beacon/pb/cores"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	queen "snple.com/queen/core"
)

type CoreService struct {
	badger  *BadgerService
	storage *storage.Storage

	// Queen 通信层
	broker         *queen.Broker
	internalClient *queen.InternalClient

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

	// 初始化 Queen broker (如果启用)
	if cs.dopts.queenEnable {
		if err := cs.initQueenBroker(); err != nil {
			cancel()
			return nil, err
		}
	}

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

	// 启动 Queen broker
	if cs.broker != nil {
		if err := cs.broker.Start(); err != nil {
			cs.Logger().Sugar().Errorf("Failed to start queen broker: %v", err)
		} else {
			cs.Logger().Sugar().Infof("Queen broker started on %s", cs.dopts.queenAddr)
		}
	}

	// 启动消息处理协程
	if cs.internalClient != nil {
		cs.closeWG.Add(1)
		go cs.processQueenMessages()
	}
}

func (cs *CoreService) Stop() {
	// 关闭内部客户端
	if cs.internalClient != nil {
		cs.internalClient.Close()
	}

	// 停止 Queen broker
	if cs.broker != nil {
		if err := cs.broker.Stop(); err != nil {
			cs.Logger().Sugar().Errorf("Failed to stop queen broker: %v", err)
		}
	}

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

func (cs *CoreService) GetBroker() *queen.Broker {
	return cs.broker
}

func (cs *CoreService) GetInternalClient() *queen.InternalClient {
	return cs.internalClient
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

	// Queen broker 选项
	queenEnable    bool
	queenAddr      string
	queenTLSConfig *tls.Config
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
		queenEnable: false,
		queenAddr:   ":3883",
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

// WithQueenBroker 启用并配置 Queen broker
func WithQueenBroker(addr string, tlsConfig *tls.Config) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		o.queenEnable = true
		if addr != "" {
			o.queenAddr = addr
		}
		o.queenTLSConfig = tlsConfig
	})
}

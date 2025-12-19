package edge

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/device"
	"github.com/snple/beacon/edge/storage"
	"github.com/snple/types"
	"go.uber.org/zap"
)

type EdgeService struct {
	badger  *BadgerService
	storage *storage.Storage
	sync    *SyncService

	queenUp types.Option[*QueenUpService]

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts edgeOptions
}

func Edge(opts ...EdgeOption) (*EdgeService, error) {
	ctx := context.Background()
	return EdgeContext(ctx, opts...)
}

func EdgeContext(ctx context.Context, opts ...EdgeOption) (*EdgeService, error) {
	ctx, cancel := context.WithCancel(ctx)

	es := &EdgeService{
		ctx:    ctx,
		cancel: cancel,
		dopts:  defaultEdgeOptions(),
	}

	for _, opt := range extraEdgeOptions {
		opt.apply(&es.dopts)
	}

	for _, opt := range opts {
		opt.apply(&es.dopts)
	}

	if err := es.dopts.check(); err != nil {
		return nil, err
	}

	badgerSvc, err := newBadgerService(es)
	if err != nil {
		return nil, err
	}
	es.badger = badgerSvc

	if es.dopts.device == nil {
		return nil, fmt.Errorf("WithDeviceTemplate is required")
	}

	dev := *es.dopts.device
	if strings.TrimSpace(dev.ID) == "" {
		return nil, fmt.Errorf("device id is required")
	}

	// Creating edge with a device requires a stable nodeID.
	if strings.TrimSpace(es.dopts.nodeID) == "" {
		return nil, fmt.Errorf("please supply WithNodeID when WithDeviceTemplate is set")
	}

	// Build node from template (generated fresh each time).
	name := es.dopts.name
	if name == "" {
		name = dev.Name
	}
	nodeConfig, err := buildNodeFromTemplate(es.dopts.nodeID, name, dev)
	if err != nil {
		return nil, err
	}

	// 创建存储（传入 node 配置，之后不可修改）
	es.storage = storage.New(badgerSvc.GetDB(), nodeConfig)

	// 加载数据（仅加载 secret 等持久化数据）
	if err := es.storage.Load(ctx); err != nil {
		return nil, err
	}

	es.sync = newSyncService(es)

	// 根据配置选择通信方式
	if es.dopts.NodeOptions.Enable {
		// 使用 Queen 通信
		queenUp, err := newQueenUpService(es)
		if err != nil {
			return nil, err
		}
		es.queenUp = types.Some(queenUp)
	}

	return es, nil
}

func (es *EdgeService) Start() {
	es.closeWG.Add(1)
	go func() {
		defer es.closeWG.Done()

		es.badger.start()
	}()

	if es.queenUp.IsSome() {
		es.closeWG.Add(1)
		go func() {
			defer es.closeWG.Done()

			es.queenUp.Unwrap().start()
		}()
	}
}

func (es *EdgeService) Stop() {
	if es.queenUp.IsSome() {
		es.queenUp.Unwrap().stop()
	}

	es.badger.stop()

	es.cancel()
	es.closeWG.Wait()
	es.dopts.logger.Sync()
}

func (es *EdgeService) Push() error {
	if es.queenUp.IsSome() {
		return es.queenUp.Unwrap().push()
	}

	return errors.New("node not enable")
}

func (es *EdgeService) GetStorage() *storage.Storage {
	return es.storage
}

func (es *EdgeService) GetBadgerDB() *badger.DB {
	return es.badger.GetDB()
}

func (es *EdgeService) GetSync() *SyncService {
	return es.sync
}

func (es *EdgeService) Context() context.Context {
	return es.ctx
}

func (es *EdgeService) Logger() *zap.Logger {
	return es.dopts.logger
}

type edgeOptions struct {
	logger *zap.Logger
	nodeID string
	name   string
	secret string
	device *device.Device

	NodeOptions     NodeOptions
	SyncOptions     SyncOptions
	BadgerOptions   badger.Options
	BadgerGCOptions BadgerGCOptions

	linkTTL time.Duration
}

type NodeOptions struct {
	Enable    bool
	QueenAddr string // Queen broker 地址
	QueenTLS  *tls.Config
}

type SyncOptions struct {
	TokenRefresh time.Duration
	Link         time.Duration
	Interval     time.Duration
	Realtime     bool
	Retry        time.Duration
}

type BadgerGCOptions struct {
	GC             time.Duration
	GCDiscardRatio float64
}

func defaultEdgeOptions() edgeOptions {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("zap.NewDevelopment(): %v", err)
	}

	return edgeOptions{
		logger:      logger,
		NodeOptions: NodeOptions{},
		SyncOptions: SyncOptions{
			TokenRefresh: 3 * time.Minute,
			Link:         time.Minute,
			Interval:     time.Minute,
			Realtime:     false,
			Retry:        time.Second * 3,
		},
		BadgerOptions: badger.DefaultOptions("").WithInMemory(true),
		BadgerGCOptions: BadgerGCOptions{
			GC:             time.Hour,
			GCDiscardRatio: 0.7,
		},
		linkTTL: 3 * time.Minute,
	}
}

func (o *edgeOptions) check() error {
	return nil
}

type EdgeOption interface {
	apply(*edgeOptions)
}

var extraEdgeOptions []EdgeOption

type funcEdgeOption struct {
	f func(*edgeOptions)
}

func (fdo *funcEdgeOption) apply(do *edgeOptions) {
	fdo.f(do)
}

func newFuncEdgeOption(f func(*edgeOptions)) *funcEdgeOption {
	return &funcEdgeOption{
		f: f,
	}
}

func WithLogger(logger *zap.Logger) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.logger = logger
	})
}

func WithNodeID(id, secret string) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.nodeID = id
		o.secret = secret
	})
}

func WithName(name string) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.name = name
	})
}

func WithDeviceTemplate(dev device.Device) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.device = &dev
	})
}

func WithNode(options NodeOptions) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.NodeOptions = options
	})
}

func WithSync(options SyncOptions) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		if options.TokenRefresh > 0 {
			o.SyncOptions.TokenRefresh = options.TokenRefresh
		}

		if options.Link > 0 {
			o.SyncOptions.Link = options.Link
		}

		if options.Interval > 0 {
			o.SyncOptions.Interval = options.Interval
		}

		o.SyncOptions.Realtime = options.Realtime
	})
}

func WithBadger(options badger.Options) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.BadgerOptions = options
	})
}

func WithBadgerGC(options *BadgerGCOptions) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		if options.GC > 0 {
			o.BadgerGCOptions.GC = options.GC
		}

		if options.GCDiscardRatio > 0 {
			o.BadgerGCOptions.GCDiscardRatio = options.GCDiscardRatio
		}
	})
}

func WithLinkTTL(d time.Duration) EdgeOption {
	return newFuncEdgeOption(func(o *edgeOptions) {
		o.linkTTL = d
	})
}

package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"sync"
	"time"

	"github.com/danclive/nson-go"
	"github.com/dgraph-io/badger/v4"
	"github.com/snple/beacon/core/storage"
	"go.uber.org/zap"
	queen "snple.com/queen/core"
	"snple.com/queen/packet"
)

type CoreService struct {
	badger  *BadgerService
	storage *storage.Storage

	// Queen 通信层
	broker         *queen.Broker
	internalClient *queen.InternalClient

	node     *NodeService
	wire     *WireService
	pin      *PinService
	pinValue *PinValueService
	pinWrite *PinWriteService

	// PinWrite 批量通知
	pinWriteChanges chan PinWriteChange

	ctx     context.Context
	cancel  func()
	closeWG sync.WaitGroup

	dopts coreOptions
}

// PinWriteChange PinWrite 变更通知
type PinWriteChange struct {
	NodeID string
	PinID  string
	Name   string
	Value  nson.Value
}

func Core(opts ...CoreOption) (*CoreService, error) {
	return CoreContext(context.Background(), opts...)
}

func CoreContext(ctx context.Context, opts ...CoreOption) (*CoreService, error) {
	ctx, cancel := context.WithCancel(ctx)

	cs := &CoreService{
		ctx:             ctx,
		cancel:          cancel,
		dopts:           defaultCoreOptions(),
		pinWriteChanges: make(chan PinWriteChange, 1000),
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

		// 启动批量 PinWrite 通知
		cs.closeWG.Add(1)
		go cs.batchNotifyPinWrite()
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

type coreOptions struct {
	logger          *zap.Logger
	linkTTL         time.Duration
	BadgerOptions   badger.Options
	BadgerGCOptions BadgerGCOptions

	// Queen broker 选项
	queenEnable    bool
	queenAddr      string
	queenTLSConfig *tls.Config

	// 批量通知间隔（默认 1 秒）
	batchNotifyInterval time.Duration
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
		queenEnable:         false,
		queenAddr:           ":3883",
		batchNotifyInterval: time.Second, // 默认 1 秒批量发送一次
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

// WithBatchNotifyInterval 设置批量通知间隔
func WithBatchNotifyInterval(interval time.Duration) CoreOption {
	return newFuncCoreOption(func(o *coreOptions) {
		if interval > 0 {
			o.batchNotifyInterval = interval
		}
	})
}

// NotifyPinWrite 通知 PinWrite 变更（非阻塞）
func (cs *CoreService) NotifyPinWrite(nodeID, pinID string, value nson.Value, realtime bool) error {
	if realtime {
		// 实时模式，直接发送
		return cs.publishPinWritesToNode(nodeID, []PinWriteChange{
			{
				NodeID: nodeID,
				PinID:  pinID,
				Value:  value,
			},
		})
	}

	select {
	case cs.pinWriteChanges <- PinWriteChange{
		NodeID: nodeID,
		PinID:  pinID,
		Value:  value,
	}:
		return nil
	default:
		// 通道满，记录警告但不阻塞
		cs.Logger().Sugar().Warnf("PinWrite changes channel is full, dropping notification for pin %s", pinID)
		return nil
	}
}

// batchNotifyPinWrite 批量发送 PinWrite 通知
func (cs *CoreService) batchNotifyPinWrite() {
	defer cs.closeWG.Done()

	ticker := time.NewTicker(cs.dopts.batchNotifyInterval)
	defer ticker.Stop()

	// 按 NodeID 分组的 PinWrite 变更
	changes := make(map[string][]PinWriteChange)

	sendBatch := func() {
		if len(changes) == 0 {
			return
		}

		for nodeID, pinWrites := range changes {
			if err := cs.publishPinWritesToNode(nodeID, pinWrites); err != nil {
				cs.Logger().Sugar().Errorf("Failed to publish pin writes to node %s: %v", nodeID, err)
			}
		}

		// 清空
		changes = make(map[string][]PinWriteChange)
	}

	for {
		select {
		case <-cs.ctx.Done():
			// 发送剩余的变更
			sendBatch()
			return

		case <-ticker.C:
			sendBatch()

		case change := <-cs.pinWriteChanges:
			changes[change.NodeID] = append(changes[change.NodeID], change)
		}
	}
}

// publishPinWritesToNode 发布 PinWrite 到指定节点
func (cs *CoreService) publishPinWritesToNode(nodeID string, pinWrites []PinWriteChange) error {
	if cs.internalClient == nil {
		return nil
	}

	type PinWriteMessage struct {
		Id    string     `nson:"id,omitempty"`
		Name  string     `nson:"name,omitempty"`
		Value nson.Value `nson:"value,omitempty"`
	}

	// 去重：同一个 Pin 只保留最后一次变更
	pinMap := make(map[string]PinWriteMessage)
	for _, pw := range pinWrites {
		pinMap[pw.PinID] = PinWriteMessage{
			Id:    pw.PinID,
			Name:  pw.Name,
			Value: pw.Value,
		}
	}

	if len(pinMap) == 0 {
		return nil
	}

	// 批量发送
	for _, msg := range pinMap {
		m, err := nson.Marshal(msg)
		if err != nil {
			continue
		}

		buf := new(bytes.Buffer)
		if err := nson.EncodeMap(m, buf); err != nil {
			continue
		}

		// 发布到节点特定的主题，指定目标客户端
		if err := cs.broker.PublishToClient(nodeID, "beacon/pin/write", buf.Bytes(), queen.PublishOptions{
			QoS: packet.QoS1,
		}); err != nil {
			return err
		}
	}

	cs.Logger().Sugar().Debugf("Published %d pin writes to node %s", len(pinMap), nodeID)
	return nil
}

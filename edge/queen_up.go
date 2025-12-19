package edge

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/danclive/nson-go"
	queen "snple.com/queen/client"
	"snple.com/queen/packet"
)

// QueenUpService 使用 Queen 协议与 Core 通信的服务
//
// 通讯约定：
// 1. REQUEST/RESPONSE：需要立即反馈的操作
//
//   - beacon.pin.write.sync: 连接时全量同步待写入数据
//
//     2. PUBLISH/SUBSCRIBE：数据同步和命令下发
//     Edge → Core: beacon/push, beacon/pin/value
//     Core → Edge: beacon/pin/write
type QueenUpService struct {
	es *EdgeService

	client *queen.Client

	ctx       context.Context
	cancel    func()
	closeWG   sync.WaitGroup
	tasksOnce sync.Once // 确保后台任务只启动一次
}

func newQueenUpService(es *EdgeService) (*QueenUpService, error) {
	es.Logger().Sugar().Infof("connecting to queen broker: %v", es.dopts.NodeOptions.QueenAddr)

	ctx, cancel := context.WithCancel(es.Context())

	s := &QueenUpService{
		es:     es,
		ctx:    ctx,
		cancel: cancel,
	}

	opts := []queen.Option{
		queen.WithBroker(es.dopts.NodeOptions.QueenAddr),
		queen.WithClientID(es.dopts.nodeID),
		queen.WithAuth("plain", []byte(es.dopts.secret)),
		queen.WithCleanSession(false),
		queen.WithKeepAlive(60),
		queen.WithConnectTimeout(10 * time.Second),
		queen.WithOnConnect(func(props map[string]string) {
			s.onConnect(props)
		}),
		queen.WithOnDisconnect(func(err error) {
			s.onDisconnect(err)
		}),
	}
	if es.dopts.NodeOptions.QueenTLS != nil {
		opts = append(opts, queen.WithTLSConfig(es.dopts.NodeOptions.QueenTLS))
	}

	s.client = queen.New(opts...)

	return s, nil
}

func (s *QueenUpService) start() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	s.es.Logger().Sugar().Info("queen up service started")

	// 使用 backoff 进行初始连接，直到成功
	operation := func() error {
		err := s.client.Connect()
		if err != nil {
			s.es.Logger().Sugar().Infof("queen connect failed: %v", err)
		}
		return err
	}

	err := backoff.Retry(operation, backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx))
	if err != nil {
		s.es.Logger().Sugar().Errorf("failed to connect after retries: %v", err)
		return
	}

	s.es.Logger().Sugar().Info("queen client connected successfully")

	// 连接成功后，client 会自动重连，我们只需等待服务停止
	<-s.ctx.Done()
	s.es.Logger().Sugar().Info("queen up service stopped")
}

func (s *QueenUpService) stop() {
	s.cancel()

	if s.client != nil {
		s.client.Disconnect()
	}

	s.closeWG.Wait()
}

func (s *QueenUpService) push() error {
	// 使用 backoff 进行初始连接
	operation := func() error {
		err := s.client.Connect()
		if err != nil {
			s.es.Logger().Sugar().Errorf("queen connect: %v", err)
		}
		return err
	}

	err := backoff.Retry(operation, backoff.WithContext(backoff.NewExponentialBackOff(), s.ctx))
	if err != nil {
		s.es.Logger().Sugar().Errorf("backoff.Retry: %v", err)
		return err
	}

	s.es.Logger().Sugar().Info("queen connect success")

	s.es.GetSync().setNodeToRemote(s.ctx, time.Time{})

	// 同步配置数据 Edge → Core
	if err := s.syncLocalToRemote(s.ctx); err != nil {
		return err
	}

	// 同步运行数据 Edge → Core
	if err := s.syncPinValueToRemote(s.ctx); err != nil {
		return err
	}

	s.es.Logger().Sugar().Info("push success")

	return nil
}

// onConnect 连接成功回调：执行订阅、全量同步和启动定时任务
func (s *QueenUpService) onConnect(_ map[string]string) {
	s.es.Logger().Sugar().Info("queen client connected")

	// 订阅 Pin Write 通知
	if err := s.subscribePinWrite(); err != nil {
		s.es.Logger().Sugar().Errorf("subscribe pin write: %v", err)
		return
	}

	// 全量同步待写入数据（重连或首次连接时获取所有待写入命令）
	if err := s.syncPinWriteFromRemote(s.ctx); err != nil {
		s.es.Logger().Sugar().Errorf("sync pin write from remote failed: %v", err)
	}

	// 执行初始同步
	if err := s.sync(s.ctx); err != nil {
		s.es.Logger().Sugar().Errorf("initial sync failed: %v", err)
	}

	// 启动定时同步任务（只启动一次）
	go s.startBackgroundTasks()
}

// onDisconnect 断开连接回调
func (s *QueenUpService) onDisconnect(err error) {
	if err != nil {
		s.es.Logger().Sugar().Warnf("queen client disconnected: %v", err)
	} else {
		s.es.Logger().Sugar().Info("queen client disconnected")
	}
}

// startBackgroundTasks 启动后台同步任务（只启动一次）
func (s *QueenUpService) startBackgroundTasks() {
	// 使用 sync.Once 确保只启动一次
	s.tasksOnce.Do(func() {
		// 启动定时同步
		go s.ticker()

		// 启动实时同步
		if s.es.dopts.SyncOptions.Realtime {
			go s.waitLocalNodeUpdated()
			go s.waitLocalPinValueUpdated()
		}
	})
}

func (s *QueenUpService) ticker() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	syncTicker := time.NewTicker(s.es.dopts.SyncOptions.Interval)
	defer syncTicker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-syncTicker.C:
			// 只在连接时同步
			if !s.client.IsConnected() {
				continue
			}
			if err := s.sync(s.ctx); err != nil {
				s.es.Logger().Sugar().Errorf("sync: %v", err)
			}
		}
	}
}

// subscribePinWrite 订阅 Pin 写入通知
// 接收 Core 的 Pin 写入命令
func (s *QueenUpService) subscribePinWrite() error {
	handler := func(msg *queen.Message) error {
		if err := s.handlePinWrite(msg.Payload); err != nil {
			s.es.Logger().Sugar().Errorf("handle pin write: %v", err)
		}
		return nil
	}

	return s.client.Subscribe("beacon/pin/write", handler, queen.WithSubQoS(packet.QoS1))
}

// handlePinWrite 处理 Core 发来的 Pin 写入通知
func (s *QueenUpService) handlePinWrite(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON Map
	buf := bytes.NewBuffer(payload)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		return fmt.Errorf("invalid pin write format: %w", err)
	}

	type PinWriteMessage struct {
		Id    string     `nson:"id,omitempty"`
		Name  string     `nson:"name,omitempty"`
		Value nson.Value `nson:"value,omitempty"`
	}

	var msg PinWriteMessage
	if err := nson.Unmarshal(m, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal pin write: %w", err)
	}

	// 保存到本地存储
	ctx := context.Background()
	pin, err := s.es.GetStorage().GetPinByID(msg.Id)
	if err != nil {
		return fmt.Errorf("get pin by id: %w", err)
	}

	// 基本类型校验（按 Pin.Type）
	if msg.Value != nil && uint32(msg.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	updated := time.Now()
	if err := s.es.GetStorage().SetPinWrite(ctx, msg.Id, msg.Value, updated); err != nil {
		return fmt.Errorf("set pin write: %w", err)
	}

	// 记录 PinWrite 更新时间（用于后续同步/通知）
	if err := s.es.GetSync().setPinWriteUpdated(ctx, updated); err != nil {
		return fmt.Errorf("set pin write updated: %w", err)
	}

	s.es.Logger().Sugar().Debugf("pin write applied: %s", msg.Id)
	return nil
}

// waitLocalNodeUpdated: Edge 配置数据变化时同步到 Core
func (s *QueenUpService) waitLocalNodeUpdated() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	notify := s.es.GetSync().Notify(NOTIFY)
	defer notify.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-notify.Wait():
			// 只在连接时同步
			if !s.client.IsConnected() {
				continue
			}
			err := s.syncLocalToRemote(s.ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncLocalToRemote: %v", err)
			}
		}
	}
}

// waitLocalPinValueUpdated: Edge PinValue 变化时同步到 Core
func (s *QueenUpService) waitLocalPinValueUpdated() {
	s.closeWG.Add(1)
	defer s.closeWG.Done()

	notify := s.es.GetSync().Notify(NOTIFY_PV)
	defer notify.Close()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-notify.Wait():
			// 只在连接时同步
			if !s.client.IsConnected() {
				continue
			}
			err := s.syncPinValueToRemote(s.ctx)
			if err != nil {
				s.es.Logger().Sugar().Errorf("syncPinValueToRemote: %v", err)
			}
		}
	}
}

func (s *QueenUpService) sync(ctx context.Context) error {
	// Edge → Core: 配置数据同步
	if err := s.syncLocalToRemote(ctx); err != nil {
		return err
	}

	// Edge → Core: PinValue 同步
	return s.syncPinValueToRemote(ctx)
}

// syncLocalToRemote: Edge 配置数据同步到 Core
// 使用 Publish 发送（数据同步，不需要立即响应）
func (s *QueenUpService) syncLocalToRemote(ctx context.Context) error {
	nodeUpdated, err := s.es.GetSync().getNodeUpdated(ctx)
	if err != nil {
		return err
	}

	nodeUpdated2, err := s.es.GetSync().getNodeToRemote(ctx)
	if err != nil {
		return err
	}

	if nodeUpdated.UnixMicro() <= nodeUpdated2.UnixMicro() {
		return nil
	}

	// 获取节点配置并导出为 NSON
	_, err = s.es.GetStorage().GetNode()
	if err != nil {
		return err
	}

	// 导出配置为 NSON 字节
	data, err := s.es.GetStorage().ExportConfig()
	if err != nil {
		return err
	}

	// 通过 Queen 发布到 beacon/push 主题
	if err := s.client.Publish("beacon/push", data, queen.WithQoS(packet.QoS1)); err != nil {
		return fmt.Errorf("publish config failed: %w", err)
	}

	return s.es.GetSync().setNodeToRemote(ctx, nodeUpdated)
}

// syncPinValueToRemote: Edge PinValue 同步到 Core
// 使用 Publish 批量发送（数据同步，不需要立即响应）
func (s *QueenUpService) syncPinValueToRemote(ctx context.Context) error {
	pinValueUpdated, err := s.es.GetSync().getPinValueUpdated(ctx)
	if err != nil {
		return err
	}

	pinValueUpdated2, err := s.es.GetSync().getPinValueToRemote(ctx)
	if err != nil {
		return err
	}

	if pinValueUpdated.UnixMicro() <= pinValueUpdated2.UnixMicro() {
		return nil
	}

	after := pinValueUpdated2
	limit := 100

	type PinValueMessage struct {
		Id    string     `nson:"id,omitempty"`
		Name  string     `nson:"name,omitempty"`
		Value nson.Value `nson:"value,omitempty"`
	}

	var messages []PinValueMessage

	// 从本地存储读取 PinValue
	for {
		items, err := s.es.GetStorage().ListPinValues(after, limit)
		if err != nil {
			return err
		}

		for _, item := range items {
			if item.Updated.After(pinValueUpdated) {
				goto DONE
			}

			if len(item.Value) > 0 {
				val, err := nson.DecodeValue(bytes.NewBuffer(item.Value))
				if err != nil {
					s.es.Logger().Sugar().Errorf("DecodeValue: %v", err)
					continue
				}

				messages = append(messages, PinValueMessage{Id: item.ID, Value: val})
			}

			after = item.Updated
		}

		if len(items) < limit {
			break
		}
	}

DONE:
	if len(messages) == 0 {
		return s.es.GetSync().setPinValueToRemote(ctx, pinValueUpdated)
	}

	// 序列化为 NSON Array
	arr := make(nson.Array, 0, len(messages))
	for _, msg := range messages {
		m, err := nson.Marshal(msg)
		if err != nil {
			continue
		}
		arr = append(arr, m)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeArray(arr, buf); err != nil {
		return err
	}

	// 通过 Queen 发布
	if err := s.client.Publish("beacon/pin/value", buf.Bytes(), queen.WithQoS(packet.QoS1)); err != nil {
		return fmt.Errorf("publish pin values failed: %w", err)
	}

	return s.es.GetSync().setPinValueToRemote(ctx, pinValueUpdated)
}

// syncPinWriteFromRemote: 从 Core 全量同步 PinWrite 写命令
// 使用 Request/Response 机制（需要立即响应，确保可靠获取）
func (s *QueenUpService) syncPinWriteFromRemote(ctx context.Context) error {
	// 使用 Request 拉取所有 pin writes
	response, err := s.client.Request(ctx, "beacon.pin.write.sync", nil, &queen.RequestOptions{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("request pin writes failed: %w", err)
	}

	if response.ReasonCode != packet.ReasonSuccess {
		return fmt.Errorf("request failed with reason: %s", response.ReasonCode)
	}

	if len(response.Payload) == 0 {
		// 没有待写入的数据
		return nil
	}

	// 解析 NSON Array
	buf := bytes.NewBuffer(response.Payload)
	arr, err := nson.DecodeArray(buf)
	if err != nil {
		return fmt.Errorf("invalid pin writes format: %w", err)
	}

	type PinWriteMessage struct {
		Id    string     `nson:"id,omitempty"`
		Name  string     `nson:"name,omitempty"`
		Value nson.Value `nson:"value,omitempty"`
	}

	// 应用每个 pin write
	for _, item := range arr {
		itemMap, ok := item.(nson.Map)
		if !ok {
			continue
		}

		var msg PinWriteMessage
		if err := nson.Unmarshal(itemMap, &msg); err != nil {
			continue
		}

		// 保存到本地存储
		pin, err := s.es.GetStorage().GetPinByID(msg.Id)
		if err != nil {
			s.es.Logger().Sugar().Errorf("GetPinByID: %v", err)
			continue
		}

		if msg.Value != nil && uint32(msg.Value.DataType()) != pin.Type {
			s.es.Logger().Sugar().Errorf("invalid value for Pin.Type")
			continue
		}

		updated := time.Now()
		if err := s.es.GetStorage().SetPinWrite(ctx, msg.Id, msg.Value, updated); err != nil {
			s.es.Logger().Sugar().Errorf("SetPinWrite: %v", err)
			continue
		}

		if err := s.es.GetSync().setPinWriteUpdated(ctx, updated); err != nil {
			s.es.Logger().Sugar().Errorf("setPinWriteUpdated: %v", err)
			continue
		}
	}

	s.es.Logger().Sugar().Infof("Synced %d pin writes from remote", len(arr))
	return nil
}

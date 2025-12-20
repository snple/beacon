package edge

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
	queen "snple.com/queen/client"
	"snple.com/queen/packet"
)

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

	// 同步配置数据和 PinValue 到 Core
	if err := s.syncToRemote(s.ctx); err != nil {
		return err
	}

	s.es.Logger().Sugar().Info("push success")

	return nil
}

// onConnect 连接成功回调：订阅、全量同步和启动批量发送
func (s *QueenUpService) onConnect(_ map[string]string) {
	s.es.Logger().Sugar().Info("queen client connected")

	// 订阅 Pin Write 通知（从 Core 接收）
	if err := s.subscribePinWrite(); err != nil {
		s.es.Logger().Sugar().Errorf("subscribe pin write: %v", err)
		return
	}

	// 订阅批量 Pin Write 通知
	if err := s.subscribePinWriteBatch(); err != nil {
		s.es.Logger().Sugar().Errorf("subscribe pin write batch: %v", err)
		return
	}

	// 全量同步待写入数据（重连或首次连接时获取所有待写入命令）
	if err := s.syncPinWriteFromRemote(s.ctx); err != nil {
		s.es.Logger().Sugar().Errorf("sync pin write from remote failed: %v", err)
	}

	// 执行初始同步（推送配置和 PinValue）
	if err := s.syncToRemote(s.ctx); err != nil {
		s.es.Logger().Sugar().Errorf("initial sync failed: %v", err)
	}

	// 启动批量发送 PinValue （只启动一次）
	go s.startBatchNotify()
}

// onDisconnect 断开连接回调
func (s *QueenUpService) onDisconnect(err error) {
	if err != nil {
		s.es.Logger().Sugar().Warnf("queen client disconnected: %v", err)
	} else {
		s.es.Logger().Sugar().Info("queen client disconnected")
	}
}

// startBatchNotify 启动批量发送 PinValue
func (s *QueenUpService) startBatchNotify() {
	s.tasksOnce.Do(func() {
		s.closeWG.Add(1)
		defer s.closeWG.Done()

		s.es.Logger().Sugar().Info("starting batch notify for PinValue")

		ticker := time.NewTicker(s.es.dopts.batchNotifyInterval)
		defer ticker.Stop()

		// 收集的 PinValue 变更（去重）
		changes := make(map[string]PinValueChange)

		sendBatch := func() {
			if len(changes) == 0 {
				return
			}

			// 只在连接时才发送
			if !s.client.IsConnected() {
				return
			}

			// 转换为数组
			var pinValues []PinValueChange
			for _, change := range changes {
				pinValues = append(pinValues, change)
			}

			if err := s.publishPinValueBatch(s.ctx, pinValues); err != nil {
				s.es.Logger().Sugar().Errorf("publish pin value batch: %v", err)
			} else {
				s.es.Logger().Sugar().Debugf("Published %d pin values", len(pinValues))
			}

			// 清空
			changes = make(map[string]PinValueChange)
		}

		for {
			select {
			case <-s.ctx.Done():
				// 发送剩余的变更
				sendBatch()
				return

			case <-ticker.C:
				sendBatch()

			case change := <-s.es.pinValueChanges:
				// 同一个 Pin 只保留最后一次变更
				changes[change.PinID] = change
			}
		}
	})
}

// subscribePinWrite 订阅 Pin 写入通知（单个，realtime模式）
// 接收 Core 的 Pin 写入命令
func (s *QueenUpService) subscribePinWrite() error {
	handler := func(msg *queen.Message) error {
		if err := s.handlePinWrite(msg.Payload); err != nil {
			s.es.Logger().Sugar().Errorf("handle pin write: %v", err)
		}
		return nil
	}

	return s.client.Subscribe(dt.TopicPinWrite, handler, queen.WithSubQoS(packet.QoS1))
}

// subscribePinWriteBatch 订阅批量 Pin 写入通知（ticker模式）
func (s *QueenUpService) subscribePinWriteBatch() error {
	handler := func(msg *queen.Message) error {
		if err := s.handlePinWriteBatch(msg.Payload); err != nil {
			s.es.Logger().Sugar().Errorf("handle pin write batch: %v", err)
		}
		return nil
	}

	return s.client.Subscribe(dt.TopicPinWriteBatch, handler, queen.WithSubQoS(packet.QoS1))
}

// handlePinWriteSingle 处理 Core 发来的单个 Pin 写入通知（realtime模式）
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

	var msg dt.PinValueMessage
	if err := nson.Unmarshal(m, &msg); err != nil {
		return fmt.Errorf("failed to unmarshal pin write: %w", err)
	}

	return s.applyPinWrite(msg)
}

// handlePinWriteBatch 处理 Core 发来的批量 Pin 写入通知（ticker模式）
func (s *QueenUpService) handlePinWriteBatch(payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON Array
	buf := bytes.NewBuffer(payload)
	arr, err := nson.DecodeArray(buf)
	if err != nil {
		return fmt.Errorf("invalid pin write batch format: %w", err)
	}

	for _, item := range arr {
		itemMap, ok := item.(nson.Map)
		if !ok {
			continue
		}

		var msg dt.PinValueMessage
		if err := nson.Unmarshal(itemMap, &msg); err != nil {
			continue
		}

		if err := s.applyPinWrite(msg); err != nil {
			s.es.Logger().Sugar().Warnf("apply pin write failed: %s, error=%v", msg.ID, err)
		}
	}

	s.es.Logger().Sugar().Debugf("Applied %d pin writes", len(arr))
	return nil
}

// applyPinWrite 应用 Pin 写入
func (s *QueenUpService) applyPinWrite(msg dt.PinValueMessage) error {
	// msg.ID 形如 "nodeID.WireName.pinName"，需要去掉 `nodeID.` 前缀
	nodeID := s.es.GetStorage().GetNodeID()
	prefix := nodeID + "."
	var pinID string
	if strings.HasPrefix(msg.ID, prefix) {
		pinID = strings.TrimPrefix(msg.ID, prefix)
	} else {
		pinID = msg.ID
	}

	// 保存到本地存储
	ctx := context.Background()
	pin, err := s.es.GetStorage().GetPinByID(pinID)
	if err != nil {
		return fmt.Errorf("get pin by id: %w", err)
	}

	// 基本类型校验（按 Pin.Type）
	if msg.Value != nil && uint32(msg.Value.DataType()) != pin.Type {
		return fmt.Errorf("invalid value for Pin.Type")
	}

	if err := s.es.GetStorage().SetPinWrite(ctx, dt.PinValue{
		ID:      pinID,
		Value:   msg.Value,
		Updated: time.Now(),
	}); err != nil {
		return fmt.Errorf("set pin write: %w", err)
	}

	s.es.Logger().Sugar().Debugf("pin write applied: %s", msg.ID)
	return nil
}

// syncToRemote: Edge → Core 同步（配置 + PinValue）
func (s *QueenUpService) syncToRemote(ctx context.Context) error {
	// 同步配置数据
	if err := s.publishConfig(ctx); err != nil {
		return err
	}

	// 同步 PinValue
	return s.publishPinValueAll(ctx)
}

// publishConfig: 发布 Edge 配置数据到 Core
func (s *QueenUpService) publishConfig(_ context.Context) error {
	node := s.es.GetStorage().GetNode()

	// 构建用于发布的 Node 结构
	nodeBuilt := buildNode(&node)

	payload, err := dt.EncodeNode(nodeBuilt)
	if err != nil {
		return err
	}

	// 通过 Queen 发布到 beacon/push 主题
	if err := s.client.Publish(dt.TopicPush, payload, queen.WithQoS(packet.QoS1)); err != nil {
		return fmt.Errorf("publish config failed: %w", err)
	}

	return nil
}

// publishPinValue: 发布 PinValue 到 Core（全量同步）
func (s *QueenUpService) publishPinValueAll(_ context.Context) error {
	limit := 100

	nodeID := s.es.GetStorage().GetNodeID()

	var messages []dt.PinValueMessage
	after := time.Time{}

	// 从本地存储读取所有 PinValue
	for {
		items, err := s.es.GetStorage().ListPinValues(after, limit)
		if err != nil {
			return err
		}

		for _, item := range items {
			messages = append(messages, dt.PinValueMessage{ID: nodeID + "." + item.ID, Value: item.Value})

			after = item.Updated
		}

		if len(items) < limit {
			break
		}
	}

	if len(messages) == 0 {
		return nil
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

	return nil
}

// publishPinValueBatch: 批量发布 PinValue 到 Core（ticker模式）
func (s *QueenUpService) publishPinValueBatch(_ context.Context, changes []PinValueChange) error {
	if len(changes) == 0 {
		return nil
	}

	var messages []dt.PinValueMessage
	for _, change := range changes {
		messages = append(messages, dt.PinValueMessage{
			ID:    change.PinID,
			Value: change.Value,
		})
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

	// 通过 Queen 发布到批量主题
	if err := s.client.Publish(dt.TopicPinValueBatch, buf.Bytes(), queen.WithQoS(packet.QoS1)); err != nil {
		return fmt.Errorf("publish pin values failed: %w", err)
	}

	return nil
}

// PublishPinValueSingle: 发布单个 PinValue 到 Core（realtime模式）
func (s *QueenUpService) PublishPinValue(ctx context.Context, value dt.PinValueMessage) error {
	nodeID := s.es.GetStorage().GetNodeID()
	// 添加 NodeID 前缀
	value.ID = nodeID + "." + value.ID

	// 序列化为 NSON Map
	m, err := nson.Marshal(value)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	// 通过 Queen 发布到单个主题
	if err := s.client.Publish(dt.TopicPinValue, buf.Bytes(), queen.WithQoS(packet.QoS1)); err != nil {
		return fmt.Errorf("publish pin value failed: %w", err)
	}

	return nil
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

	// 应用每个 pin write
	for _, item := range arr {
		itemMap, ok := item.(nson.Map)
		if !ok {
			continue
		}

		var msg dt.PinValueMessage
		if err := nson.Unmarshal(itemMap, &msg); err != nil {
			continue
		}

		// 保存到本地存储
		pin, err := s.es.GetStorage().GetPinByID(msg.ID)
		if err != nil {
			s.es.Logger().Sugar().Errorf("GetPinByID: %v", err)
			continue
		}

		if msg.Value != nil && uint32(msg.Value.DataType()) != pin.Type {
			s.es.Logger().Sugar().Errorf("invalid value for Pin.Type")
			continue
		}

		if err := s.applyPinWrite(msg); err != nil {
			s.es.Logger().Sugar().Warnf("apply pin write failed: %s, error=%v", msg.ID, err)
		}
	}

	s.es.Logger().Sugar().Infof("Synced %d pin writes from remote", len(arr))
	return nil
}

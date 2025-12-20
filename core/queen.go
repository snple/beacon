package core

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
	queen "snple.com/queen/core"
	"snple.com/queen/packet"
)

// initQueenBroker 初始化 Queen broker
func (cs *CoreService) initQueenBroker() error {
	brokerOpts := []queen.BrokerOption{
		queen.WithAddress(cs.dopts.queenAddr),
	}

	if cs.dopts.queenTLSConfig != nil {
		brokerOpts = append(brokerOpts, queen.WithTLS(cs.dopts.queenTLSConfig))
		cs.Logger().Sugar().Infof("Queen broker TLS enabled")
	}

	// 设置认证器，使用 beacon 的 node 认证
	brokerOpts = append(brokerOpts, queen.WithAuthFunc(func(info *queen.AuthInfo) error {
		return cs.authenticateBrokerClient(info)
	}))

	broker, err := queen.New(brokerOpts...)
	if err != nil {
		return err
	}

	cs.broker = broker

	// 创建内部客户端并注册必要的 action 处理器（非致命）
	icCfg := queen.InternalClientConfig{ClientID: "beacon-core", BufferSize: 200}
	ic, err := cs.broker.NewInternalClient(icCfg)
	if err != nil {
		cs.Logger().Sugar().Warnf("Failed to create internal client: %v", err)
		// 仍然返回 broker 可用，但不阻塞服务启动
		return nil
	}
	cs.internalClient = ic

	// 注册内部 action 处理器
	if err := cs.registerQueenHandlers(); err != nil {
		cs.Logger().Sugar().Warnf("Failed to register internal handlers: %v", err)
	}

	// 订阅所有 beacon 主题
	if err := cs.subscribeQueenTopics(); err != nil {
		cs.Logger().Sugar().Warnf("Failed to subscribe topics: %v", err)
	}

	return nil
}

// authenticateBrokerClient 认证 broker 客户端
func (cs *CoreService) authenticateBrokerClient(info *queen.AuthInfo) error {
	ctx := context.Background()

	// ClientID 作为 Node ID 或 Name
	clientID := info.ClientID
	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	// AuthData 作为 Secret
	secret := string(info.AuthData)
	if secret == "" {
		return fmt.Errorf("secret is required")
	}

	// 尝试通过 ID 或 Name 获取 Node
	var node *dt.Node
	var err error

	node, err = cs.GetNode().View(ctx, clientID)
	if err != nil {
		return fmt.Errorf("node not found: %s", clientID)
	}

	// 验证密钥
	nodeSecret, err := cs.GetNode().GetSecret(ctx, node.ID)
	if err != nil {
		return fmt.Errorf("getSecret failed: %w", err)
	}

	if nodeSecret != secret {
		return fmt.Errorf("invalid secret")
	}

	cs.Logger().Sugar().Infof("Queen broker client authenticated: %s, remote: %s", node.ID, info.RemoteAddr)

	return nil
}

// IsNodeOnline 检查节点是否在线
func (cs *CoreService) IsNodeOnline(nodeID string) bool {
	if cs.broker == nil {
		return false
	}
	return cs.broker.IsClientOnline(nodeID)
}

// PublishToNode 向指定节点发布消息
func (cs *CoreService) PublishToNode(nodeID string, topic string, payload []byte, qos int) error {
	if cs.broker == nil {
		return fmt.Errorf("queen broker not enabled")
	}

	return cs.broker.PublishToClient(nodeID, topic, payload, queen.PublishOptions{
		QoS: packet.QoS(qos),
	})
}

// subscribeQueenTopics 订阅所有 beacon 主题
// 用于接收 Edge 的数据同步消息（不需要立即响应）
func (cs *CoreService) subscribeQueenTopics() error {
	if cs.internalClient == nil {
		return fmt.Errorf("internal client not initialized")
	}

	// 订阅所有 beacon 相关主题（数据同步类消息）
	topics := []string{
		dt.TopicPush,          // Edge -> Core: 配置数据推送
		dt.TopicPinValue,      // Edge -> Core: Pin 值推送（单个，realtime模式）
		dt.TopicPinValueBatch, // Edge -> Core: Pin 值批量推送（批量，ticker模式）
	}

	for _, topic := range topics {
		if err := cs.internalClient.Subscribe(topic, packet.QoS1); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", topic, err)
		}
	}

	cs.Logger().Sugar().Infof("Internal client subscribed to beacon topics")
	return nil
}

// registerQueenHandlers 注册内部 action 处理器
// 用于处理需要立即响应的 Request
func (cs *CoreService) registerQueenHandlers() error {
	if cs.internalClient == nil {
		return fmt.Errorf("internal client not initialized")
	}

	// beacon.pin.write.sync -> 返回全量待写入的 pin writes（NSON Array）
	// Edge 在连接或重连时调用，确保获取所有待写入命令
	_ = cs.internalClient.RegisterHandler(dt.ActionPinWriteSync, func(req *queen.InternalRequest) ([]byte, packet.ReasonCode, error) {
		data, err := cs.buildPinWritesPayload(req.SourceClientID)
		if err != nil {
			return nil, packet.ReasonImplementationError, err
		}
		if data == nil {
			return nil, packet.ReasonSuccess, nil
		}
		return data, packet.ReasonSuccess, nil
	}, 2)

	return nil
}

// processQueenMessages 处理内部客户端消息的后台协程
func (cs *CoreService) processQueenMessages() {
	defer cs.closeWG.Done()

	for {
		msg, err := cs.internalClient.Receive()
		if err != nil {
			// 内部客户端关闭
			return
		}

		// 从消息中提取 clientID（即 nodeID）
		clientID := msg.SourceClientID
		topic := msg.Topic
		payload := msg.Payload

		cs.Logger().Sugar().Debugf("Processing message: topic=%s, clientID=%s, payload_len=%d",
			topic, clientID, len(payload))

		// 根据主题路由消息
		var err2 error
		switch topic {
		case dt.TopicPush:
			err2 = cs.handlePush(clientID, payload)

		case dt.TopicPinValue:
			err2 = cs.handlePinValue(clientID, payload)

		case dt.TopicPinValueBatch:
			err2 = cs.handlePinValueBatch(clientID, payload)

		default:
			// 未知主题，忽略
			cs.Logger().Sugar().Debugf("Unknown topic: %s", topic)
		}

		if err2 != nil {
			cs.Logger().Sugar().Errorf("Failed to handle message: topic=%s, clientID=%s, error=%v",
				topic, clientID, err2)
		}
	}
}

// handlePush 处理 NSON 数据推送
func (cs *CoreService) handlePush(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	node, err := dt.DecodeNode(payload)
	if err != nil {
		return fmt.Errorf("decode node: %w", err)
	}

	err = cs.GetNode().Push(context.Background(), node)
	if err != nil {
		cs.Logger().Sugar().Errorf("Push failed: nodeID=%s, error=%v", nodeID, err)
		return err
	}

	cs.Logger().Sugar().Debugf("Push success: nodeID=%s, payload_len=%d", nodeID, len(payload))
	return nil
}

// handlePinValue 处理单个 Pin 值推送（realtime模式）
func (cs *CoreService) handlePinValue(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON Map
	buf := bytes.NewBuffer(payload)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		cs.Logger().Sugar().Errorf("Invalid pin value format: %v", err)
		return err
	}

	v := dt.PinValueMessage{}
	if err := nson.Unmarshal(m, &v); err != nil {
		cs.Logger().Sugar().Errorf("Failed to unmarshal pin value: %v", err)
		return err
	}

	ctx := context.Background()
	if err := cs.setPinValue(ctx, nodeID, v); err != nil {
		cs.Logger().Sugar().Warnf("Set pin value failed: nodeID=%s, pin=%s, error=%v",
			nodeID, v.ID, err)
		return err
	}

	return nil
}

// handlePinValueBatch 处理批量 Pin 值推送（ticker模式）
func (cs *CoreService) handlePinValueBatch(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON 数组
	buf := bytes.NewBuffer(payload)
	arr, err := nson.DecodeArray(buf)
	if err != nil {
		cs.Logger().Sugar().Errorf("Invalid pin value batch format: %v", err)
		return err
	}

	ctx := context.Background()

	for _, item := range arr {
		itemMap, ok := item.(nson.Map)
		if !ok {
			continue
		}

		v := dt.PinValueMessage{}
		if err := nson.Unmarshal(itemMap, &v); err != nil {
			continue
		}

		if err := cs.setPinValue(ctx, nodeID, v); err != nil {
			cs.Logger().Sugar().Warnf("Set pin value failed: nodeID=%s, pin=%s, error=%v",
				nodeID, v.ID, err)
		}
	}

	return nil
}

// setPinValue 设置 Pin 值
func (cs *CoreService) setPinValue(ctx context.Context, nodeID string, v dt.PinValueMessage) error {
	// 先验证 Pin 属于当前节点
	_, err := cs.GetPin().View(ctx, nodeID, v.ID)
	if err != nil {
		return err
	}

	err = cs.GetPinValue().SetValue(ctx, dt.PinValue{
		ID:      v.ID,
		Value:   v.Value,
		Updated: time.Now(),
	})

	return err
}

// PublishPinWrite 向节点发送单个 Pin 写入命令（realtime模式）
func (cs *CoreService) PublishPinWrite(nodeID string, pinWrite *dt.PinValueMessage) error {
	if cs.broker == nil {
		return fmt.Errorf("queen broker not enabled")
	}

	// 序列化为 NSON Map
	m, err := nson.Marshal(pinWrite)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	return cs.broker.PublishToClient(nodeID, dt.TopicPinWrite, buf.Bytes(), queen.PublishOptions{
		QoS: packet.QoS1,
	})
}

// PublishPinWriteBatch 向节点发送批量 Pin 写入命令（ticker模式）
func (cs *CoreService) PublishPinWriteBatch(nodeID string, pinWrites []dt.PinValueMessage) error {
	if cs.broker == nil {
		return fmt.Errorf("queen broker not enabled")
	}

	if len(pinWrites) == 0 {
		return nil
	}

	// 序列化为 NSON Array
	arr := make(nson.Array, 0, len(pinWrites))
	for _, w := range pinWrites {
		m, err := nson.Marshal(w)
		if err != nil {
			continue
		}
		arr = append(arr, m)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeArray(arr, buf); err != nil {
		return err
	}

	return cs.broker.PublishToClient(nodeID, dt.TopicPinWriteBatch, buf.Bytes(), queen.PublishOptions{
		QoS: packet.QoS1,
	})
}

// buildPinWritesPayload 构建节点所有待写入的 pin write 列表（NSON Array）
func (cs *CoreService) buildPinWritesPayload(nodeID string) ([]byte, error) {
	ctx := context.Background()

	// 获取节点的所有 Pin
	pins, err := cs.GetPin().List(ctx, nodeID, "")
	if err != nil {
		return nil, err
	}

	// 收集有写入值的 Pin
	var writes []dt.PinValueMessage
	for _, pin := range pins {
		writeValue, _, err := cs.GetPinWrite().GetWrite(ctx, pin.ID)
		if err != nil {
			continue // 忽略没有写入值的 Pin
		}
		if writeValue == nil {
			continue
		}

		writes = append(writes, dt.PinValueMessage{
			ID:    pin.ID,
			Value: writeValue,
		})
	}

	if len(writes) == 0 {
		return nil, nil
	}

	// 序列化为 NSON Array
	arr := make(nson.Array, 0, len(writes))
	for _, w := range writes {
		m, err := nson.Marshal(w)
		if err != nil {
			continue
		}
		arr = append(arr, m)
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeArray(arr, buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

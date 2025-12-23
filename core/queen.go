package core

import (
	"bytes"
	"context"
	"fmt"
	"strings"
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

	// 设置认证处理器，使用 beacon 的 node 认证
	brokerOpts = append(brokerOpts, queen.WithAuthHandler(&queen.AuthHandlerFunc{
		ConnectFunc: func(ctx *queen.AuthContext) error {
			return cs.authenticateBrokerClient(ctx)
		},
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
	if err := cs.registerHandlers(); err != nil {
		cs.Logger().Sugar().Warnf("Failed to register internal handlers: %v", err)
	}

	// 订阅所有 beacon 主题
	if err := cs.subscribeTopics(); err != nil {
		cs.Logger().Sugar().Warnf("Failed to subscribe topics: %v", err)
	}

	return nil
}

// authenticateBrokerClient 认证 broker 客户端
func (cs *CoreService) authenticateBrokerClient(ctx *queen.AuthContext) error {
	// ClientID 作为 Node ID
	clientID := ctx.ClientID
	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	// AuthData 作为 Secret
	secret := string(ctx.AuthData)
	if secret == "" {
		return fmt.Errorf("secret is required")
	}

	// 验证密钥（不需要 Node 已存在，只要 Secret 已设置即可）
	ctxBg := context.Background()
	nodeSecret, err := cs.GetNode().GetSecret(ctxBg, clientID)
	if err != nil {
		return fmt.Errorf("node secret not found for: %s", clientID)
	}

	if nodeSecret != secret {
		return fmt.Errorf("invalid secret")
	}

	cs.Logger().Sugar().Infof("Queen broker client authenticated: %s, remote: %s", clientID, ctx.RemoteAddr)

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
func (cs *CoreService) subscribeTopics() error {
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
func (cs *CoreService) registerHandlers() error {
	if cs.internalClient == nil {
		return fmt.Errorf("internal client not initialized")
	}

	// beacon.pin.write.sync -> 返回全量待写入的 pin writes（NSON Array）
	// Edge 在连接或重连时调用，确保获取所有待写入命令
	if err := cs.internalClient.RegisterHandler(dt.ActionPinWriteSync, func(req *queen.InternalRequest) ([]byte, packet.ReasonCode, error) {
		data, err := cs.buildPinWritesPayload(req.SourceClientID)
		if err != nil {
			return nil, packet.ReasonImplementationError, err
		}
		if data == nil {
			return nil, packet.ReasonSuccess, nil
		}
		return data, packet.ReasonSuccess, nil
	}, 2); err != nil {
		return fmt.Errorf("failed to register handler %s: %w", dt.ActionPinWriteSync, err)
	}

	cs.Logger().Sugar().Infof("Registered internal action handlers")

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

	cs.Logger().Sugar().Debugf("Push received: nodeID=%s, payload=%v", nodeID, node)

	err = cs.GetNode().Push(node)
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

	if err := cs.setPinValue(nodeID, v); err != nil {
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

	cs.Logger().Sugar().Debugf("Push received: nodeID=%s, payload=%v", nodeID, arr)

	for _, item := range arr {
		itemMap, ok := item.(nson.Map)
		if !ok {
			continue
		}

		v := dt.PinValueMessage{}
		if err := nson.Unmarshal(itemMap, &v); err != nil {
			continue
		}

		if err := cs.setPinValue(nodeID, v); err != nil {
			cs.Logger().Sugar().Warnf("Set pin value failed: nodeID=%s, pin=%s, error=%v",
				nodeID, v.ID, err)
		}
	}

	return nil
}

// setPinValue 设置 Pin 值
func (cs *CoreService) setPinValue(nodeID string, v dt.PinValueMessage) error {
	// 验证 v.ID 的格式，必须是 "NodeID.WireName.PinName"
	parts := strings.Split(v.ID, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid pin ID format: expected 'NodeID.WireName.PinName', got '%s'", v.ID)
	}

	// 验证 NodeID
	if parts[0] != nodeID {
		return fmt.Errorf("node ID mismatch: expected '%s', got '%s'", nodeID, parts[0])
	}

	// 使用完整的 Pin ID (与 Push 时的格式一致)
	err := cs.GetPinValue().setValue(dt.PinValue{
		ID:      v.ID, // 使用完整格式: "NodeID.WireName.PinName"
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
	// 获取节点的所有 Pin
	pins, err := cs.GetPin().List(nodeID, "")
	if err != nil {
		return nil, err
	}

	// 收集有写入值的 Pin
	var writes []dt.PinValueMessage
	for _, pin := range pins {
		writeValue, _, err := cs.GetPinWrite().GetWrite(pin.ID)
		if err != nil {
			cs.Logger().Sugar().Debugf("buildPinWritesPayload: skip pin %s due to error: %v", pin.ID, err)
			continue // 忽略没有写入值的 Pin
		}
		if writeValue == nil {
			cs.Logger().Sugar().Debugf("buildPinWritesPayload: skip pin %s due to nil value", pin.ID)
			continue
		}

		cs.Logger().Sugar().Debugf("buildPinWritesPayload: adding pin %s, value=%v (type=%T)", pin.ID, writeValue, writeValue)

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

package core

import (
	"bytes"
	"context"
	"fmt"

	"github.com/danclive/nson-go"
	queen "snple.com/queen/core"
	"snple.com/queen/packet"
)

// Queen 协议通讯约定：
// 1. REQUEST/RESPONSE：需要立即反馈的操作
//   - beacon.pin.write.sync: Edge 连接时全量同步待写入数据
//
// 2. PUBLISH/SUBSCRIBE：数据同步和命令下发
//
// Edge → Core（数据同步，使用 Publish）:
//   - beacon/push      : 配置数据推送（NSON 格式）
//   - beacon/pin/value : Pin 值批量推送（NSON Array）
//
// Core → Edge（命令下发，使用 Publish）:
//   - beacon/pin/write  : Pin 写入命令（NSON Map）
//   - beacon/node/update: 节点配置更新通知（NSON Map）
const (
	TopicPrefix     = "beacon/"
	TopicPush       = "beacon/push"
	TopicPinValue   = "beacon/pin/value"
	TopicPinWrite   = "beacon/pin/write"
	TopicNodeUpdate = "beacon/node/update"

	// Action 名称（用于 Request/Response）
	ActionPinWriteSync = "beacon.pin.write.sync"
)

// PinValueMessage Pin 值消息
type PinValueMessage struct {
	Id    string     `nson:"id,omitempty"`    // Pin ID
	Name  string     `nson:"name,omitempty"`  // Pin 名称 (wire.pin 格式)
	Value nson.Value `nson:"value,omitempty"` // NSON 值
}

// PinWriteMessage Pin 写入消息
type PinWriteMessage struct {
	Id    string     `nson:"id,omitempty"`    // Pin ID
	Name  string     `nson:"name,omitempty"`  // Pin 名称
	Value nson.Value `nson:"value,omitempty"` // NSON 值
}

// subscribeQueenTopics 订阅所有 beacon 主题
// 用于接收 Edge 的数据同步消息（不需要立即响应）
func (cs *CoreService) subscribeQueenTopics() error {
	if cs.internalClient == nil {
		return fmt.Errorf("internal client not initialized")
	}

	// 订阅所有 beacon 相关主题（数据同步类消息）
	topics := []string{
		TopicPush,     // Edge -> Core: 配置数据推送
		TopicPinValue, // Edge -> Core: Pin 值批量推送
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
	_ = cs.internalClient.RegisterHandler(ActionPinWriteSync, func(req *queen.InternalRequest) ([]byte, packet.ReasonCode, error) {
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
		case TopicPush:
			err2 = cs.handlePush(clientID, payload)

		case TopicPinValue:
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

	ctx := context.Background()

	request := &NodePushRequest{
		Id:   nodeID,
		Nson: payload,
	}

	_, err := cs.GetNode().Push(ctx, request)
	if err != nil {
		cs.Logger().Sugar().Errorf("Push failed: nodeID=%s, error=%v", nodeID, err)
		return err
	}

	cs.Logger().Sugar().Debugf("Push success: nodeID=%s, payload_len=%d", nodeID, len(payload))
	return nil
}

// handlePinValueBatch 处理批量 Pin 值推送
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

		v := PinValueMessage{}
		if err := nson.Unmarshal(itemMap, &v); err != nil {
			continue
		}

		if err := cs.setPinValue(ctx, nodeID, &v); err != nil {
			cs.Logger().Sugar().Warnf("Set pin value failed: nodeID=%s, pin=%s, error=%v",
				nodeID, v.Id, err)
		}
	}

	return nil
}

// setPinValue 设置 Pin 值
func (cs *CoreService) setPinValue(ctx context.Context, nodeID string, v *PinValueMessage) error {

	if v.Name != "" {
		// 通过名称设置
		request := &PinNameValueRequest{
			NodeId: nodeID,
			Name:   v.Name,
			Value:  v.Value,
		}
		_, err := cs.GetPinValue().SetValueByName(ctx, request)
		return err
	}

	if v.Id != "" {
		// 通过 ID 设置
		// 先验证 Pin 属于当前节点
		pinRequest := &PinViewRequest{NodeId: nodeID, PinId: v.Id}
		_, err := cs.GetPin().View(ctx, pinRequest)
		if err != nil {
			return err
		}

		pinValue := &PinValue{
			Id:    v.Id,
			Value: v.Value,
		}
		_, err = cs.GetPinValue().SetValue(ctx, pinValue)
		return err
	}

	return nil
}

// buildPinWritesPayload 构建节点所有待写入的 pin write 列表（NSON Array）
func (cs *CoreService) buildPinWritesPayload(nodeID string) ([]byte, error) {
	ctx := context.Background()

	// 获取节点的所有 Pin
	pinsRequest := &PinListRequest{NodeId: nodeID}
	pinsReply, err := cs.GetPin().List(ctx, pinsRequest)
	if err != nil {
		return nil, err
	}

	// 收集有写入值的 Pin
	var writes []PinWriteMessage
	for _, pin := range pinsReply.Pins {
		writeValue, err := cs.GetPinWrite().GetWrite(ctx, &Id{Id: pin.Id})
		if err != nil {
			continue // 忽略没有写入值的 Pin
		}
		if writeValue.Value == nil {
			continue
		}

		writes = append(writes, PinWriteMessage{
			Id:    pin.Id,
			Name:  pin.Name,
			Value: writeValue.Value,
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

// NotifyPinWrite 通知节点有新的 Pin 写入
// 使用 Publish 发送命令到 Edge
func (cs *CoreService) NotifyPinWrite(nodeID string, pinID string, pinName string, value nson.Value) error {
	if cs.broker == nil {
		return nil
	}

	// 检查节点是否在线
	if !cs.broker.IsClientOnline(nodeID) {
		cs.Logger().Sugar().Debugf("Node not online, skip pin write notify: nodeID=%s", nodeID)
		return nil
	}

	msg := PinWriteMessage{
		Id:    pinID,
		Name:  pinName,
		Value: value,
	}

	// 序列化为 NSON
	m, err := nson.Marshal(msg)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	err = cs.broker.PublishToClient(nodeID, TopicPinWrite, buf.Bytes(), queen.PublishOptions{
		QoS: 1,
	})
	if err != nil {
		cs.Logger().Sugar().Warnf("Notify pin write failed: nodeID=%s, pinID=%s, error=%v",
			nodeID, pinID, err)
	}

	return err
}

// NotifyNodeUpdate 通知节点配置更新
func (cs *CoreService) NotifyNodeUpdate(nodeID string) error {
	if cs.broker == nil {
		return nil
	}

	if !cs.broker.IsClientOnline(nodeID) {
		return nil
	}

	// 获取节点最新配置
	ctx := context.Background()
	node, err := cs.GetNode().View(ctx, &Id{Id: nodeID})
	if err != nil {
		return err
	}

	// 序列化为 NSON
	m, err := nson.Marshal(node)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := nson.EncodeMap(m, buf); err != nil {
		return err
	}

	return cs.broker.PublishToClient(nodeID, TopicNodeUpdate, buf.Bytes(), queen.PublishOptions{
		QoS: 1,
	})
}

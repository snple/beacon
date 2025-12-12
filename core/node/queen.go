package node

import (
	"bytes"
	"context"
	"strings"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/pb"
	"github.com/snple/beacon/pb/cores"
	queen "snple.com/queen/core"
)

// Queen 协议主题定义
// 主题格式: beacon/{action}
// action 可选值:
//   - push          : 推送 NSON 数据
//   - pin/value     : 推送 Pin 值 (批量)
//   - pin/value/set : 设置单个 Pin 值
//
// 响应主题 (broker -> 节点):
//   - beacon/pin/write : Pin 写入通知
const (
	TopicPrefix       = "beacon/"
	TopicPush         = "beacon/push"
	TopicPinValue     = "beacon/pin/value"
	TopicPinValueSet  = "beacon/pin/value/set"
	TopicPinWrite     = "beacon/pin/write"
	TopicPinWritePull = "beacon/pin/write/pull"
)

// PinValueMessage Pin 值消息
type PinValueMessage struct {
	Id    string        `nson:"id,omitempty"`    // Pin ID
	Name  string        `nson:"name,omitempty"`  // Pin 名称 (wire.pin 格式)
	Value *pb.NsonValue `nson:"value,omitempty"` // NsonValue 格式值
}

// PinWriteMessage Pin 写入消息
type PinWriteMessage struct {
	Id    string        `nson:"id,omitempty"`    // Pin ID
	Name  string        `nson:"name,omitempty"`  // Pin 名称
	Value *pb.NsonValue `nson:"value,omitempty"` // NsonValue 格式值
}

// handleQueenMessage 处理 queen 协议消息
func (ns *NodeService) handleQueenMessage(ctx *queen.PublishContext) error {
	topic := ctx.Message.Topic
	clientID := ctx.ClientID // clientID 即 nodeID

	ns.Logger().Sugar().Debugf("Queen message received: topic=%s, clientID=%s, payload_len=%d",
		topic, clientID, len(ctx.Message.Payload))

	// 根据主题路由消息
	switch topic {
	case TopicPush:
		return ns.handlePush(clientID, ctx.Message.Payload)

	case TopicPinValue:
		return ns.handlePinValueBatch(clientID, ctx.Message.Payload)

	case TopicPinValueSet:
		return ns.handlePinValueSet(clientID, ctx.Message.Payload)

	case TopicPinWritePull:
		return ns.handlePinWritePull(clientID, ctx.Message.Payload)

	default:
		// 未知主题，忽略
		ns.Logger().Sugar().Debugf("Unknown queen topic: %s", topic)
	}

	return nil
}

// handlePush 处理 NSON 数据推送
func (ns *NodeService) handlePush(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	ctx := context.Background()

	request := &cores.NodePushRequest{
		Id:   nodeID,
		Nson: payload,
	}

	_, err := ns.Core().GetNode().Push(ctx, request)
	if err != nil {
		ns.Logger().Sugar().Errorf("Push failed: nodeID=%s, error=%v", nodeID, err)
		return err
	}

	ns.Logger().Sugar().Debugf("Push success: nodeID=%s, payload_len=%d", nodeID, len(payload))
	return nil
}

// handlePinValueBatch 处理批量 Pin 值推送
func (ns *NodeService) handlePinValueBatch(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON 数组
	buf := bytes.NewBuffer(payload)
	arr, err := nson.DecodeArray(buf)
	if err != nil {
		ns.Logger().Sugar().Errorf("Invalid pin value batch format: %v", err)
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

		if err := ns.setPinValue(ctx, nodeID, &v); err != nil {
			ns.Logger().Sugar().Warnf("Set pin value failed: nodeID=%s, pin=%s, error=%v",
				nodeID, v.Id, err)
		}
	}

	return nil
}

// handlePinValueSet 处理单个 Pin 值设置
func (ns *NodeService) handlePinValueSet(nodeID string, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}

	// 解析 NSON Map
	buf := bytes.NewBuffer(payload)
	m, err := nson.DecodeMap(buf)
	if err != nil {
		ns.Logger().Sugar().Errorf("Invalid pin value format: %v", err)
		return err
	}

	var v PinValueMessage
	if err := nson.Unmarshal(m, &v); err != nil {
		ns.Logger().Sugar().Errorf("Failed to unmarshal pin value: %v", err)
		return err
	}

	ctx := context.Background()
	return ns.setPinValue(ctx, nodeID, &v)
}

// setPinValue 设置 Pin 值
func (ns *NodeService) setPinValue(ctx context.Context, nodeID string, v *PinValueMessage) error {
	if v.Name != "" {
		// 通过名称设置
		request := &cores.PinNameValueRequest{
			NodeId: nodeID,
			Name:   v.Name,
			Value:  v.Value,
		}
		_, err := ns.Core().GetPinValue().SetValueByName(ctx, request)
		return err
	}

	if v.Id != "" {
		// 通过 ID 设置
		// 先验证 Pin 属于当前节点
		pinRequest := &cores.PinViewRequest{NodeId: nodeID, PinId: v.Id}
		_, err := ns.Core().GetPin().View(ctx, pinRequest)
		if err != nil {
			return err
		}

		pinValue := &pb.PinValue{
			Id:    v.Id,
			Value: v.Value,
		}
		_, err = ns.Core().GetPinValue().SetValue(ctx, pinValue)
		return err
	}

	return nil
}

// handlePinWritePull 处理 Pin 写入拉取请求
func (ns *NodeService) handlePinWritePull(nodeID string, payload []byte) error {
	// 可以实现增量拉取逻辑
	// 简化实现：发送所有待写入的 Pin 值
	return ns.pushPinWritesToClient(nodeID)
}

// pushPinWritesToClient 向客户端推送 Pin 写入
func (ns *NodeService) pushPinWritesToClient(nodeID string) error {
	if ns.broker == nil {
		return nil
	}

	ctx := context.Background()

	// 获取节点的所有 Pin
	pinsRequest := &cores.PinListRequest{NodeId: nodeID}
	pinsReply, err := ns.Core().GetPin().List(ctx, pinsRequest)
	if err != nil {
		return err
	}

	// 收集有写入值的 Pin
	var writes []PinWriteMessage
	for _, pin := range pinsReply.Pins {
		writeValue, err := ns.Core().GetPinWrite().GetWrite(ctx, &pb.Id{Id: pin.Id})
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
		return nil
	}

	// 序列化并发送 (NSON)
	// 构建 NSON Array
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
		return err
	}

	return ns.broker.PublishToClient(nodeID, TopicPinWrite, buf.Bytes(), queen.PublishOptions{
		QoS: 1,
	})
}

// NotifyPinWrite 通知节点有新的 Pin 写入
// 当上层设置 Pin 写入值时调用此方法
func (ns *NodeService) NotifyPinWrite(nodeID string, pinID string, pinName string, value *pb.NsonValue) error {
	if ns.broker == nil {
		return nil
	}

	// 检查节点是否在线
	if !ns.broker.IsClientOnline(nodeID) {
		ns.Logger().Sugar().Debugf("Node not online, skip pin write notify: nodeID=%s", nodeID)
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

	err = ns.broker.PublishToClient(nodeID, TopicPinWrite, buf.Bytes(), queen.PublishOptions{
		QoS: 1,
	})
	if err != nil {
		ns.Logger().Sugar().Warnf("Notify pin write failed: nodeID=%s, pinID=%s, error=%v",
			nodeID, pinID, err)
	}

	return err
}

// NotifyNodeUpdate 通知节点配置更新
func (ns *NodeService) NotifyNodeUpdate(nodeID string) error {
	if ns.broker == nil {
		return nil
	}

	if !ns.broker.IsClientOnline(nodeID) {
		return nil
	}

	// 获取节点最新配置
	ctx := context.Background()
	node, err := ns.Core().GetNode().View(ctx, &pb.Id{Id: nodeID})
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

	return ns.broker.PublishToClient(nodeID, "beacon/node/update", buf.Bytes(), queen.PublishOptions{
		QoS: 1,
	})
}

// parseNodeIDFromTopic 从主题中解析节点 ID (如果需要)
func parseNodeIDFromTopic(topic string) string {
	// 主题格式: beacon/{nodeID}/...
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

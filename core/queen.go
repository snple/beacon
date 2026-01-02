package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/dt"
	queen "snple.com/queen/core"
	"snple.com/queen/packet"
)

// initQueenCore 初始化 Queen core
func (cs *CoreService) initQueenCore() error {
	coreOpts := queen.NewCoreOptions().
		WithAddress(cs.dopts.queenAddr).
		WithLogger(cs.Logger())

	if cs.dopts.queenTLSConfig != nil {
		coreOpts = coreOpts.WithTLS(cs.dopts.queenTLSConfig)
		cs.Logger().Sugar().Infof("Queen core TLS enabled")
	}

	// 设置认证处理器，使用 beacon 的 node 认证
	coreOpts = coreOpts.WithAuthHandler(&queen.AuthHandlerFunc{
		ConnectFunc: func(ctx *queen.AuthConnectContext) error {
			return cs.authenticateClient(ctx)
		},
	})

	core, err := queen.NewWithOptions(coreOpts)
	if err != nil {
		return err
	}

	cs.core = core

	return nil
}

// authenticateClient 认证客户端
func (cs *CoreService) authenticateClient(ctx *queen.AuthConnectContext) error {
	// ClientID 作为 Node ID
	clientID := ctx.ClientID
	if clientID == "" {
		return fmt.Errorf("clientID is required")
	}

	// AuthData 作为 Secret
	secret := string(ctx.Packet.Properties.AuthData)
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

	cs.Logger().Sugar().Infof("Queen client authenticated: %s, remote: %s", clientID, ctx.RemoteAddr)

	return nil
}

// IsNodeOnline 检查节点是否在线
func (cs *CoreService) IsNodeOnline(nodeID string) bool {
	if cs.core == nil {
		return false
	}
	return cs.core.IsClientOnline(nodeID)
}

// PublishToNode 向指定节点发布消息
func (cs *CoreService) PublishToNode(nodeID string, topic string, payload []byte, qos int) error {
	if cs.core == nil {
		return fmt.Errorf("queen core not enabled")
	}

	return cs.core.PublishToClient(nodeID, topic, payload, queen.PublishOptions{
		QoS: packet.QoS(qos),
	})
}

// processQueenMessages 处理消息的后台协程（使用轮询模式）
func (cs *CoreService) processQueenMessages() {
	defer cs.closeWG.Done()

	cs.Logger().Sugar().Debug("processQueenMessages: started")

	for {
		select {
		case <-cs.ctx.Done():
			cs.Logger().Sugar().Debug("processQueenMessages: context canceled, exiting")
			return
		default:
		}

		// 轮询消息（5秒超时）
		msg, err := cs.core.PollMessage(cs.ctx, 5*time.Second)
		if err != nil {
			if cs.ctx.Err() != nil {
				// context 取消，正常退出
				return
			}
			if errors.Is(err, queen.ErrPollTimeout) {
				continue // 超时，继续轮询
			}
			// core 停止，优雅退出
			if errors.Is(err, queen.ErrCoreNotRunning) || errors.Is(err, queen.ErrCoreShutdown) {
				cs.Logger().Sugar().Debugf("PollMessage: core stopped, exiting")
				return
			}
			cs.Logger().Sugar().Errorf("PollMessage error: %v", err)
			continue
		}

		if msg == nil {
			// 超时，继续轮询
			cs.Logger().Sugar().Debug("PollMessage: timeout, continuing")
			continue
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

// processQueenRequests 处理请求的后台协程（使用轮询模式）
func (cs *CoreService) processQueenRequests() {
	defer cs.closeWG.Done()

	for {
		select {
		case <-cs.ctx.Done():
			return
		default:
		}

		// 轮询请求（5秒超时）
		req, err := cs.core.PollRequest(cs.ctx, 5*time.Second)
		if err != nil {
			if cs.ctx.Err() != nil {
				// context 取消，正常退出
				return
			}
			if errors.Is(err, queen.ErrPollTimeout) {
				continue // 超时，继续轮询
			}
			// core 停止，优雅退出
			if errors.Is(err, queen.ErrCoreNotRunning) || errors.Is(err, queen.ErrCoreShutdown) {
				cs.Logger().Sugar().Debugf("PollRequest: core stopped, exiting")
				return
			}
			cs.Logger().Sugar().Errorf("PollRequest error: %v", err)
			continue
		}

		if req == nil {
			// 超时，继续轮询
			continue
		}

		cs.Logger().Sugar().Debugf("Processing request: action=%s, clientID=%s",
			req.Action, req.SourceClientID)

		// 处理请求
		switch req.Action {
		case dt.ActionPinWriteSync:
			data, err := cs.buildPinWritesPayload(req.SourceClientID)
			if err != nil {
				req.RespondError(packet.ReasonImplementationError, err.Error())
			} else {
				req.RespondSuccess(data)
			}

		default:
			req.RespondError(packet.ReasonActionNotFound, fmt.Sprintf("unknown action: %s", req.Action))
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

	cs.Logger().Sugar().Debugf("Push received: nodeID=%s, payload=%v", nodeID, v)

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

		cs.Logger().Sugar().Debugf("Push received: nodeID=%s, payload=%v", nodeID, v)

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
	if cs.core == nil {
		return fmt.Errorf("queen core not enabled")
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

	return cs.core.PublishToClient(nodeID, dt.TopicPinWrite, buf.Bytes(), queen.PublishOptions{
		QoS: packet.QoS1,
	})
}

// PublishPinWriteBatch 向节点发送批量 Pin 写入命令（ticker模式）
func (cs *CoreService) PublishPinWriteBatch(nodeID string, pinWrites []dt.PinValueMessage) error {
	if cs.core == nil {
		return fmt.Errorf("queen core not enabled")
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

	return cs.core.PublishToClient(nodeID, dt.TopicPinWriteBatch, buf.Bytes(), queen.PublishOptions{
		QoS: packet.QoS1,
	})
}

// buildPinWritesPayload 构建节点所有待写入的 pin write 列表（NSON Array）
func (cs *CoreService) buildPinWritesPayload(nodeID string) ([]byte, error) {
	// 获取节点的所有 Pin
	pins, err := cs.GetPin().List(nodeID, "")
	if err != nil {
		// 如果节点不存在或没有 Pin，返回空列表（不作为错误）
		cs.Logger().Sugar().Debugf("buildPinWritesPayload: no pins for node %s: %v", nodeID, err)
		return nil, nil
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

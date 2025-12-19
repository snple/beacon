package core

import (
	"context"
	"fmt"

	"github.com/snple/beacon/device"
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
	var node *Node
	var err error

	node, err = cs.GetNode().View(ctx, &Id{Id: clientID})
	if err != nil {
		// 尝试通过 Name 获取
		node, err = cs.GetNode().Name(ctx, &Name{Name: clientID})
		if err != nil {
			return fmt.Errorf("node not found: %s", clientID)
		}
	}

	if node.Status != device.ON {
		return fmt.Errorf("node is not enabled")
	}

	// 验证密钥
	secretReply, err := cs.GetNode().GetSecret(ctx, &Id{Id: node.Id})
	if err != nil {
		return fmt.Errorf("getSecret failed: %w", err)
	}

	if secretReply.Message != secret {
		return fmt.Errorf("invalid secret")
	}

	cs.Logger().Sugar().Infof("Queen broker client authenticated: %s, remote: %s", node.Id, info.RemoteAddr)

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

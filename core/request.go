package core

import (
	"fmt"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// ============================================================================
//  client 请求处理函数
// ============================================================================

// handleRegister 处理 REGISTER 包
func (c *Client) handleRegister(p *packet.RegisterPacket) error {
	c.core.logger.Debug("Handle REGISTER",
		zap.String("clientID", c.ID),
		zap.Strings("actions", p.Actions))

	// 获取并发处理能力
	var concurrency uint16 = 1
	if p.Properties != nil && p.Properties.Concurrency != nil {
		concurrency = *p.Properties.Concurrency
	}

	// 注册 actions
	results := c.core.actionRegistry.register(c.ID, p.Actions, concurrency)

	// 构造响应
	regResults := make([]packet.RegisterResult, 0, len(results))
	for action, code := range results {
		regResults = append(regResults, packet.RegisterResult{
			Action:     action,
			ReasonCode: code,
		})
	}

	regack := packet.NewRegackPacket(regResults)
	return c.WritePacket(regack)
}

// handleUnregister 处理 UNREGISTER 包
func (c *Client) handleUnregister(p *packet.UnregisterPacket) error {
	c.core.logger.Debug("Handle UNREGISTER",
		zap.String("clientID", c.ID),
		zap.Strings("actions", p.Actions))

	// 注销 actions
	results := c.core.actionRegistry.unregister(c.ID, p.Actions)

	// 构造响应
	unregResults := make([]packet.RegisterResult, 0, len(results))
	for action, code := range results {
		unregResults = append(unregResults, packet.RegisterResult{
			Action:     action,
			ReasonCode: code,
		})
	}

	unregack := packet.NewUnregackPacket(unregResults)
	return c.WritePacket(unregack)
}

// handleRequest 处理 REQUEST 包
func (c *Client) handleRequest(p *packet.RequestPacket) error {
	c.core.logger.Debug("Handle REQUEST",
		zap.String("clientID", c.ID),
		zap.Uint32("requestID", p.RequestID),
		zap.String("action", p.Action),
		zap.String("targetClientID", p.TargetClientID))

	// 调用 OnRequest 钩子（直接传递原始 packet）
	reqCtx := &RequestContext{
		ClientID: c.ID,
		Packet:   p,
	}
	if err := c.core.options.Hooks.callOnRequest(reqCtx); err != nil {
		c.core.logger.Debug("Request rejected by hook",
			zap.String("clientID", c.ID),
			zap.String("action", p.Action),
			zap.Error(err))
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonNotAuthorized, nil)
		resp.Properties.ReasonString = err.Error()
		return c.WritePacket(resp)
	}

	// 验证 Action 名称
	if !validateActionName(p.Action) {
		resp := packet.NewResponsePacket(
			p.RequestID,
			c.ID,
			packet.ReasonActionInvalid,
			nil,
		)
		resp.Properties.ReasonString = fmt.Sprintf("Invalid action name: %s", p.Action)
		return c.WritePacket(resp)
	}

	// 填充 SourceClientID（由 core 填充，确保可信）
	p.SourceClientID = c.ID

	// 由指定的客户端或动态选择的客户端处理
	return c.handleRequestToClient(p)
}

// handleRequestToClient 处理发送给客户端的请求（路径2：由指定或动态选择的客户端处理）
// 核心思路：纯转发模式，保持原始 RequestID 和 SourceClientID 不变
func (c *Client) handleRequestToClient(p *packet.RequestPacket) error {
	var targetClientID string
	var reasonCode packet.ReasonCode

	if p.TargetClientID != "" {
		// 指定了目标客户端，检查是否存在
		targetClientID = p.TargetClientID

		c.core.clientsMu.RLock()
		client := c.core.clients[targetClientID]
		c.core.clientsMu.RUnlock()

		if client != nil {
			reasonCode = packet.ReasonSuccess
		} else {
			reasonCode = packet.ReasonClientNotFound
		}
	} else {
		// 通过 action 路由选择处理者
		targetClientID, reasonCode = c.core.actionRegistry.selectHandler(
			p.Action,
			"",
			func(id string) *Client {
				c.core.clientsMu.RLock()
				defer c.core.clientsMu.RUnlock()
				return c.core.clients[id]
			},
		)
	}

	// 处理选择失败的情况
	if reasonCode != packet.ReasonSuccess {
		resp := packet.NewResponsePacket(p.RequestID, c.ID, reasonCode, nil)
		switch reasonCode {
		case packet.ReasonActionNotFound:
			resp.Properties.ReasonString = fmt.Sprintf("Action not found: %s", p.Action)
		case packet.ReasonClientNotFound:
			resp.Properties.ReasonString = fmt.Sprintf("Target client not found: %s", p.TargetClientID)
		case packet.ReasonNoAvailableHandler:
			resp.Properties.ReasonString = fmt.Sprintf("No available handler for action: %s", p.Action)
		}
		return c.WritePacket(resp)
	}

	// 获取目标客户端并发送请求
	c.core.clientsMu.RLock()
	targetClient := c.core.clients[targetClientID]
	c.core.clientsMu.RUnlock()

	if targetClient == nil {
		// 目标客户端已断开
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonClientNotFound, nil)
		resp.Properties.ReasonString = "Target client not found"
		return c.WritePacket(resp)
	}

	// 直接转发请求包，保持原始 RequestID 和 SourceClientID
	if err := targetClient.WritePacket(p); err != nil {
		// 发送失败，返回错误响应
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonClientNotFound, nil)
		resp.Properties.ReasonString = "Target client disconnected"
		return c.WritePacket(resp)
	}

	return nil
}

// handleResponse 处理 RESPONSE 包
// 纯转发模式：直接根据 TargetClientID 转发响应，保持原始 RequestID 不变
func (c *Client) handleResponse(p *packet.ResponsePacket) error {
	c.core.logger.Debug("Handle RESPONSE",
		zap.String("clientID", c.ID),
		zap.Uint32("requestID", p.RequestID),
		zap.String("targetClientID", p.TargetClientID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))

	// 根据 TargetClientID 查找目标客户端（即发起请求的客户端）
	c.core.clientsMu.RLock()
	targetClient := c.core.clients[p.TargetClientID]
	c.core.clientsMu.RUnlock()

	if targetClient == nil {
		c.core.logger.Debug("Target client not found for response",
			zap.String("targetClientID", p.TargetClientID),
			zap.Uint32("requestID", p.RequestID))
		return nil
	}

	// 调用 OnResponse 钩子
	respCtx := &ResponseContext{
		ClientID: p.TargetClientID,
		Packet:   p,
	}
	c.core.options.Hooks.callOnResponse(respCtx)

	// 直接转发响应包，保持原始 RequestID
	return targetClient.WritePacket(p)
}

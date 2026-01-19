package core

import (
	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// handleAuth 处理 AUTH 包（用于高级认证流程）
func (c *Client) handleAuth(p *packet.AuthPacket) error {
	c.core.logger.Debug("Received AUTH packet",
		zap.String("clientID", c.ID),
		zap.Uint8("reasonCode", uint8(p.ReasonCode)))

	// 构建认证上下文（直接引用原始 packet）
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	remoteAddr := ""
	if conn != nil && conn.conn != nil {
		remoteAddr = conn.conn.RemoteAddr().String()
	}

	ctx := &AuthContext{
		ClientID:   c.ID,
		RemoteAddr: remoteAddr,
		Packet:     p,
	}

	// 调用认证钩子
	continued, authData, err := c.core.options.Hooks.callOnAuth(ctx)

	// 构建响应 AUTH 包
	authResp := packet.NewAuthPacket(packet.ReasonSuccess)
	if authResp.Properties == nil {
		authResp.Properties = packet.NewAuthProperties()
	}

	if err != nil {
		// 认证失败
		authResp.ReasonCode = packet.ReasonNotAuthorized
		if authResp.Properties.UserProperties == nil {
			authResp.Properties.UserProperties = make(map[string]string)
		}
		authResp.Properties.UserProperties["error"] = err.Error()

		conn.writePacket(authResp)

		c.core.logger.Warn("Authentication failed",
			zap.String("clientID", c.ID),
			zap.Error(err))
		return err
	}

	if continued {
		// 需要继续认证流程
		authResp.ReasonCode = packet.ReasonContinueAuth
		authResp.Properties.AuthData = authData
	} else {
		// 认证成功
		authResp.ReasonCode = packet.ReasonSuccess
		if authData != nil {
			authResp.Properties.AuthData = authData
		}
	}

	if err := conn.writePacket(authResp); err != nil {
		c.core.logger.Error("Failed to send AUTH response",
			zap.String("clientID", c.ID),
			zap.Error(err))
		return err
	}

	c.core.logger.Debug("AUTH response sent",
		zap.String("clientID", c.ID),
		zap.Uint8("reasonCode", uint8(authResp.ReasonCode)),
		zap.Bool("continued", continued))

	return nil
}

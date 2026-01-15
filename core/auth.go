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
	ctx := &AuthContext{
		ClientID:   c.ID,
		RemoteAddr: c.conn.RemoteAddr().String(),
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
		c.WritePacket(authResp)

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

	c.WritePacket(authResp)

	c.core.logger.Debug("AUTH response sent",
		zap.String("clientID", c.ID),
		zap.Uint8("reasonCode", uint8(authResp.ReasonCode)),
		zap.Bool("continued", continued))

	return nil
}

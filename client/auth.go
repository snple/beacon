package client

import (
	"fmt"

	"github.com/snple/beacon/packet"

	"go.uber.org/zap"
)

// handleAuth 处理服务器发来的 AUTH 包
func (c *Client) handleAuth(p *packet.AuthPacket) {
	c.logger.Debug("Received AUTH packet",
		zap.String("reasonCode", p.ReasonCode.String()))

	// 如果没有设置钩子，默认不处理
	if c.options.Hooks.AuthHandler == nil {
		c.logger.Warn("Received AUTH packet but no AuthHandler configured")
		return
	}

	// 调用 OnAuth 钩子（直接传递原始 packet）
	authCtx := &AuthContext{
		ClientID: c.clientID,
		Packet:   p,
	}
	continueAuth, responseData, err := c.options.Hooks.callOnAuth(authCtx)

	// 根据回调结果决定是否发送响应 AUTH 包
	if err != nil {
		c.logger.Error("Authentication handler error", zap.Error(err))
		// 认证失败，断开连接
		c.close(fmt.Errorf("authentication failed: %w", err))
		return
	}

	if continueAuth {
		// 继续认证流程，发送 AUTH 包
		authResp := packet.NewAuthPacket(packet.ReasonContinueAuth)
		if authResp.Properties == nil {
			authResp.Properties = packet.NewAuthProperties()
		}
		authResp.Properties.AuthData = responseData

		if err := c.writePacket(authResp); err != nil {
			c.logger.Error("Failed to send AUTH response", zap.Error(err))
			c.close(err)
		} else {
			c.logger.Debug("Sent AUTH response",
				zap.String("reasonCode", packet.ReasonContinueAuth.String()))
		}
	} else {
		// 认证成功，可选地发送最终确认
		if responseData != nil {
			authResp := packet.NewAuthPacket(packet.ReasonSuccess)
			if authResp.Properties == nil {
				authResp.Properties = packet.NewAuthProperties()
			}
			authResp.Properties.AuthData = responseData

			if err := c.writePacket(authResp); err != nil {
				c.logger.Error("Failed to send final AUTH response", zap.Error(err))
			} else {
				c.logger.Debug("Sent final AUTH response",
					zap.String("reasonCode", packet.ReasonSuccess.String()))
			}
		}
		c.logger.Info("Authentication completed successfully")
	}
}

// SendAuth 发送 AUTH 包（用于主动发起认证或重新认证）
func (c *Client) SendAuth(reasonCode packet.ReasonCode, authData []byte) error {
	if !c.connected.Load() {
		return ErrNotConnected
	}

	auth := packet.NewAuthPacket(reasonCode)
	if auth.Properties == nil {
		auth.Properties = packet.NewAuthProperties()
	}
	auth.Properties.AuthData = authData

	if err := c.writePacket(auth); err != nil {
		c.logger.Error("Failed to send AUTH packet", zap.Error(err))
		return err
	}

	c.logger.Debug("Sent AUTH packet",
		zap.String("reasonCode", reasonCode.String()))
	return nil
}

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

	if conn == nil {
		// 连接已被接管或清理，无法发送 AUTH 响应，直接丢弃
		c.core.logger.Debug("AUTH packet ignored: no active connection",
			zap.String("clientID", c.ID))
		return nil
	}

	remoteAddr := ""
	if conn.conn != nil {
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

	// 认证器通过 ResponseProperties 指定回给客户端的用户属性（成功/继续/失败均生效）
	if len(ctx.ResponseProperties) > 0 {
		if authResp.Properties.UserProperties == nil {
			authResp.Properties.UserProperties = make(map[string]string, len(ctx.ResponseProperties))
		}
		for k, v := range ctx.ResponseProperties {
			authResp.Properties.UserProperties[k] = v
		}
	}

	if err != nil {
		// 认证失败
		authResp.ReasonCode = packet.ReasonNotAuthorized
		if authResp.Properties.UserProperties == nil {
			authResp.Properties.UserProperties = make(map[string]string)
		}
		authResp.Properties.UserProperties["error"] = err.Error()

		if err := conn.writePacket(authResp); err != nil {
			c.core.logger.Error("Failed to send AUTH response",
				zap.String("clientID", c.ID),
				zap.Error(err))
		}

		c.core.logger.Warn("Authentication failed",
			zap.String("clientID", c.ID),
			zap.Error(err))
		// 认证失败不是协议错误：关闭原因应上报 NotAuthorized（管理面/事件可见）。
		return &closeReasonError{reason: packet.ReasonNotAuthorized, err: err}
	}

	if continued {
		// 需要继续认证流程
		authResp.ReasonCode = packet.ReasonContinueAuth
		authResp.Properties.AuthData = authData
	} else {
		// 认证成功：重新认证给出的身份必须落到会话上，
		// 否则 ClientInfo/事件里的身份会一直是首次 CONNECT 时的旧值。
		if ctx.Identity != nil {
			c.session.setIdentity(ctx.Identity)
		}
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

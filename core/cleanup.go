package core

import (
	"time"

	"go.uber.org/zap"
)

// expiredCleanupLoop 定期清理过期消息和离线会话
func (c *Core) expiredCleanupLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.options.ExpiredCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanupExpiredMessages()
			c.cleanupExpiredSessions()
		}
	}
}

// cleanupExpiredMessages 清理所有客户端的过期消息
func (c *Core) cleanupExpiredMessages() {
	now := time.Now().Unix()
	expiredCount := 0

	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		clients = append(clients, client)
	}
	c.clientsMu.RUnlock()

	// 在锁外清理每个客户端的过期消息
	for _, client := range clients {
		count := client.CleanupExpired(now)
		expiredCount += count
	}

	if expiredCount > 0 {
		c.logger.Debug("Cleaned up expired messages", zap.Int("count", expiredCount))
		c.stats.MessagesDropped.Add(int64(expiredCount))
	}
}

// cleanupExpiredSessions 清理过期的离线会话
func (c *Core) cleanupExpiredSessions() {
	now := time.Now()

	// 收集过期的会话 ID
	c.offlineSessionsMu.Lock()
	expiredIDs := make([]string, 0)
	for clientID, expiryTime := range c.offlineSessions {
		if now.After(expiryTime) {
			expiredIDs = append(expiredIDs, clientID)
			delete(c.offlineSessions, clientID)
		}
	}
	c.offlineSessionsMu.Unlock()

	// 清理过期会话
	for _, clientID := range expiredIDs {
		c.clientsMu.Lock()
		client, exists := c.clients[clientID]
		if exists && client.IsClosed() {
			// 只清理已关闭的客户端（离线状态）
			delete(c.clients, clientID)
			c.clientsMu.Unlock()

			// 清理订阅
			c.subscriptions.RemoveClient(clientID)

			// 清理持久化消息
			if c.messageStore != nil {
				if err := c.messageStore.deleteAllForClient(clientID); err != nil {
					c.logger.Warn("Failed to cleanup expired session messages",
						zap.String("clientID", clientID),
						zap.Error(err))
				}
			}

			c.logger.Info("Expired session removed", zap.String("clientID", clientID))
		} else {
			c.clientsMu.Unlock()
		}
	}
}

// CleanupExpired 清理过期消息，返回清理数量
func (c *Client) CleanupExpired(now int64) int {
	expiredCount := 0

	// 清理 pendingAck 中的过期消息
	c.qosMu.Lock()
	for packetID, pending := range c.pendingAck {
		if pending.msg.Packet != nil && pending.msg.Packet.Properties != nil &&
			pending.msg.Packet.Properties.ExpiryTime > 0 && now > pending.msg.Packet.Properties.ExpiryTime {
			// 删除持久化
			if c.core.messageStore != nil {
				c.core.messageStore.delete(c.ID, pending.msg.Packet.PacketID)
			}
			delete(c.pendingAck, packetID)
			expiredCount++
		}
	}
	c.qosMu.Unlock()

	return expiredCount
}

// clearPersistedMessages 清空该客户端的持久化消息 (CleanSession=true)
func (c *Client) clearPersistedMessages() {
	if c.core.messageStore == nil {
		return
	}

	if err := c.core.messageStore.deleteAllForClient(c.ID); err != nil {
		c.core.logger.Error("Failed to clear persisted messages",
			zap.String("clientID", c.ID),
			zap.Error(err))
	}
}

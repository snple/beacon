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
			c.cleanupExpiredSessions()
			c.cleanupExpiredMessages()
			c.cleanupExpiredRetainMessages()
		}
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
		if exists && client.Closed() {
			// 只清理已关闭的客户端（离线状态）
			delete(c.clients, clientID)
			c.clientsMu.Unlock()

			// 清理客户端相关资源
			c.cleanupClient(clientID)

			c.logger.Info("Expired session removed", zap.String("clientID", clientID))
		} else {
			c.clientsMu.Unlock()
		}
	}
}

// cleanupClient 清理客户端相关资源（需要先获取 clientsMu 锁）
func (c *Core) cleanupClient(clientID string) {
	// 清理订阅
	subCount := c.subTree.unsubscribeClient(clientID)
	if subCount > 0 {
		c.stats.SubscriptionsCount.Add(-int64(subCount))
	}

	// 清理注册的 actions
	actions := c.actionRegistry.unregisterClient(clientID)
	if len(actions) > 0 {
		c.logger.Debug("Unregistered actions on disconnect",
			zap.String("clientID", clientID),
			zap.Strings("actions", actions))
	}

	// 清理消息队列
	c.clientsMu.RLock()
	client, exists := c.clients[clientID]
	c.clientsMu.RUnlock()

	if exists {
		if err := client.queue.Clear(); err != nil {
			c.logger.Warn("Failed to clear client message queue",
				zap.String("clientID", clientID),
				zap.Error(err))
		}
	}
}

// cleanupClientWithoutLock 清理客户端相关资源（假定调用者已持有 clientsMu 锁）
func (c *Core) cleanupClientWithoutLock(clientID string, client *Client) {
	// 清理订阅
	subCount := c.subTree.unsubscribeClient(clientID)
	if subCount > 0 {
		c.stats.SubscriptionsCount.Add(-int64(subCount))
	}

	// 清理注册的 actions
	actions := c.actionRegistry.unregisterClient(clientID)
	if len(actions) > 0 {
		c.logger.Debug("Unregistered actions on disconnect",
			zap.String("clientID", clientID),
			zap.Strings("actions", actions))
	}

	// 清理消息队列
	if client != nil {
		if err := client.queue.Clear(); err != nil {
			c.logger.Warn("Failed to clear client message queue",
				zap.String("clientID", clientID),
				zap.Error(err))
		}
	}
}

// cleanupExpiredMessages 清理所有客户端的过期消息
func (c *Core) cleanupExpiredMessages() {
	expiredCount := 0

	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		clients = append(clients, client)
	}
	c.clientsMu.RUnlock()

	// 在锁外清理每个客户端的过期消息
	for _, client := range clients {
		count := client.cleanupExpired()
		expiredCount += count
	}

	if expiredCount > 0 {
		c.logger.Debug("Cleaned up expired messages", zap.Int("count", expiredCount))
		c.stats.MessagesDropped.Add(int64(expiredCount))
	}
}

// cleanupExpiredRetainMessages 清理过期的保留消息
func (c *Core) cleanupExpiredRetainMessages() {
	// 清理 retainStore 中的过期索引
	count, expiredTopics := c.retainStore.cleanupExpired()
	if count > 0 {
		c.logger.Debug("Cleaned up expired retain message index", zap.Int("count", count))

		// 同时从 store 中删除对应的消息
		for _, topic := range expiredTopics {
			if err := c.store.deleteRetain(topic); err != nil {
				c.logger.Warn("Failed to delete expired retain message from store",
					zap.String("topic", topic),
					zap.Error(err))
			}
		}
	}
}

// cleanupExpired 清理过期消息，返回清理数量
func (c *Client) cleanupExpired() int {
	expiredCount := 0

	// 清理 pendingAck 中的过期消息
	c.session.pendingAckMu.Lock()
	for packetID, pending := range c.session.pendingAck {
		if pending.msg.IsExpired() {
			// 删除持久化
			if err := c.queue.Delete(packetID); err != nil {
				c.core.logger.Debug("Failed to delete expired pendingAck message from queue",
					zap.String("clientID", c.ID),
					zap.String("packetID", packetID.Hex()),
					zap.Error(err))
			}

			// 从内存中删除
			delete(c.session.pendingAck, packetID)
			expiredCount++
		}
	}
	c.session.pendingAckMu.Unlock()

	return expiredCount
}

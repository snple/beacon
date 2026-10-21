package core

import (
	"sort"
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
			c.cleanupClient(clientID, client)

			c.logger.Info("Expired session removed", zap.String("clientID", clientID))
		} else {
			c.clientsMu.Unlock()
		}
	}
}

// cleanupClient 清理客户端相关资源。
// 当 client 非 nil 时，使用传入的已脱离 clients map 的客户端对象完成资源清理。
func (c *Core) cleanupClient(clientID string, client *Client) {
	// 会话元数据随会话一起删除
	c.deleteSession(clientID)

	// 清理订阅
	subCount := c.subTree.unsubscribeClient(clientID)
	if subCount > 0 {
		c.stats.SubscriptionsCount.Add(-int64(subCount))
	}

	// 清理消息队列
	if client == nil {
		c.clientsMu.RLock()
		client = c.clients[clientID]
		c.clientsMu.RUnlock()
	}

	if client != nil {
		if err := client.queue.Clear(); err != nil {
			c.logger.Warn("Failed to clear client message queue",
				zap.String("clientID", clientID),
				zap.Error(err))
		}
	}
}

// cleanupClientWithoutLock 清理客户端相关资源（假定调用者已持有 clientsMu 锁）
func (c *Core) cleanupClientWithoutLock(clientID string, client *Client) {
	// 会话元数据随会话一起删除
	c.deleteSession(clientID)

	// 清理订阅
	subCount := c.subTree.unsubscribeClient(clientID)
	if subCount > 0 {
		c.stats.SubscriptionsCount.Add(-int64(subCount))
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

// addOfflineSession 记录离线会话；超过 MaxOfflineSessions 时驱逐最早过期的会话。
// 调用方应先把会话记录持久化，这样被驱逐时 cleanupClient 能一并删除它。
func (c *Core) addOfflineSession(clientID string, expiry time.Time) {
	c.offlineSessionsMu.Lock()
	c.offlineSessions[clientID] = expiry
	c.offlineSessionsMu.Unlock()

	c.evictEarliestOfflineSession()
}

// evictEarliestOfflineSession 在离线会话数超限时驱逐最早过期的会话。
// 增量路径每次最多超限 1 个，因此单次扫描即可。
func (c *Core) evictEarliestOfflineSession() {
	limit := c.options.MaxOfflineSessions
	if limit <= 0 {
		return
	}

	c.offlineSessionsMu.Lock()
	if len(c.offlineSessions) <= limit {
		c.offlineSessionsMu.Unlock()
		return
	}

	var victim string
	var victimExpiry time.Time
	for id, expiry := range c.offlineSessions {
		if victim == "" || expiry.Before(victimExpiry) {
			victim = id
			victimExpiry = expiry
		}
	}
	delete(c.offlineSessions, victim)
	c.offlineSessionsMu.Unlock()

	c.evictOfflineSession(victim)
}

// evictOfflineSession 删除离线会话的客户端对象、订阅、待投递队列与会话记录。
func (c *Core) evictOfflineSession(clientID string) {
	c.clientsMu.Lock()
	client, exists := c.clients[clientID]
	if exists && client.Closed() {
		delete(c.clients, clientID)
	} else {
		exists = false
	}
	c.clientsMu.Unlock()

	if !exists {
		return
	}

	c.cleanupClient(clientID, client)
	c.logger.Warn("Offline session evicted (MaxOfflineSessions reached)",
		zap.String("clientID", clientID),
		zap.Int("limit", c.options.MaxOfflineSessions))
}

// trimOfflineSessions 在启动恢复后把离线会话收敛到上限内。
// 与增量驱逐不同，这里一次性排序，避免大量超限会话导致 O(n²)。
func (c *Core) trimOfflineSessions() int {
	limit := c.options.MaxOfflineSessions
	if limit <= 0 {
		return 0
	}

	type sessionExpiry struct {
		id     string
		expiry time.Time
	}

	c.offlineSessionsMu.Lock()
	if len(c.offlineSessions) <= limit {
		c.offlineSessionsMu.Unlock()
		return 0
	}

	entries := make([]sessionExpiry, 0, len(c.offlineSessions))
	for id, expiry := range c.offlineSessions {
		entries = append(entries, sessionExpiry{id: id, expiry: expiry})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].expiry.Before(entries[j].expiry) })

	victims := entries[:len(entries)-limit]
	for _, v := range victims {
		delete(c.offlineSessions, v.id)
	}
	c.offlineSessionsMu.Unlock()

	for _, v := range victims {
		c.evictOfflineSession(v.id)
	}

	return len(victims)
}

// isOfflineSession 报告 clientID 当前是否仍在离线会话表中。
func (c *Core) isOfflineSession(clientID string) bool {
	c.offlineSessionsMu.Lock()
	_, ok := c.offlineSessions[clientID]
	c.offlineSessionsMu.Unlock()
	return ok
}

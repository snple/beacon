package core

import (
	"time"

	"github.com/dgraph-io/badger/v4"
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

// cleanupClient 清理客户端相关资源
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

	// 清理持久化消息（如果 CleanSession 或会话过期）
	if c.messageStore != nil {
		if err := c.messageStore.deleteAllForClient(clientID); err != nil {
			c.logger.Warn("Failed to cleanup client messages",
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

	// 清理持久化存储中的过期消息
	if c.messageStore != nil {
		count, err := c.messageStore.cleanupExpired()
		if err != nil {
			c.logger.Warn("Failed to cleanup expired messages from store", zap.Error(err))
		} else if count > 0 {
			c.logger.Debug("Cleaned up expired messages from store", zap.Int("count", count))
			c.stats.MessagesDropped.Add(int64(count))
		}
	}
}

// cleanupExpiredRetainMessages 清理过期的保留消息
func (c *Core) cleanupExpiredRetainMessages() {
	// 清理 retainStore 中的过期索引
	count, expiredTopics := c.retainStore.cleanupExpired()
	if count > 0 {
		c.logger.Debug("Cleaned up expired retain message index", zap.Int("count", count))

		// 同时从 messageStore 中删除对应的消息
		if c.messageStore != nil {
			for _, topic := range expiredTopics {
				if err := c.messageStore.deleteRetainMessage(topic); err != nil {
					c.logger.Warn("Failed to delete expired retain message from store",
						zap.String("topic", topic),
						zap.Error(err))
				}
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
			if c.core.messageStore != nil {
				c.core.messageStore.delete(c.ID, packetID)
			}
			delete(c.session.pendingAck, packetID)
			expiredCount++
		}
	}
	c.session.pendingAckMu.Unlock()

	return expiredCount
}

// cleanupExpired 清理过期消息
func (ms *messageStore) cleanupExpired() (int, error) {
	count := 0
	keysToDelete := make([][]byte, 0)

	err := ms.db.View(func(txn *badger.Txn) error {
		prefix := []byte("msg:")
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var msg StoredMessage

				err := decodeStoredMessage(&msg, val)
				if err != nil {
					ms.logger.Warn("Failed to decode message",
						zap.String("key", string(item.Key())),
						zap.Error(err))
					return nil // 跳过损坏的消息
				}

				// 检查是否过期
				if msg.Message.IsExpired() {
					keyCopy := make([]byte, len(item.Key()))
					copy(keyCopy, item.Key())
					keysToDelete = append(keysToDelete, keyCopy)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// 批量删除过期消息
	if len(keysToDelete) > 0 {
		err = ms.db.Update(func(txn *badger.Txn) error {
			for _, key := range keysToDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
				count++
			}
			return nil
		})
	}

	return count, err
}

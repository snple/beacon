package core

import "go.uber.org/zap"

// recoverMessages 恢复持久化的消息
func (c *Core) recoverMessages() {
	if c.messageStore == nil {
		return
	}

	// 流式遍历保留消息，逐条重建索引，避免一次性加载所有消息到内存
	count := 0
	err := c.messageStore.iterateRetainMessageIndex(func(info RetainMessageInfo) bool {
		// 只存索引到 retainStore
		c.retainStore.set(info.Topic, info.ExpiryTime)
		count++
		return true // 继续遍历
	})

	if err != nil {
		c.logger.Error("Failed to recover retain messages", zap.Error(err))
	} else if count > 0 {
		c.logger.Info("Recovered retain message index", zap.Int("count", count))
	}

	// 统计待投递消息数量 (不加载到内存，客户端重连时按需加载)
	stats, err := c.messageStore.getStats()
	if err != nil {
		c.logger.Error("Failed to get storage stats", zap.Error(err))
	} else if stats.TotalMessages > 0 {
		c.logger.Info("Pending messages in storage (lazy load on client reconnect)",
			zap.Int64("count", stats.TotalMessages))
	}
}

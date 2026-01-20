package core

import "go.uber.org/zap"

// recoverMessages 恢复持久化的消息
func (c *Core) recoverMessages() {
	if c.store == nil {
		return
	}

	// 流式遍历保留消息，逐条重建索引，避免一次性加载所有消息到内存
	count := 0
	err := c.store.iterateRetain(func(msg *Message) bool {
		// 只存索引到 retainStore
		c.retainStore.set(msg.Packet.Topic, msg.Packet.Properties.ExpiryTime)
		count++
		return true // 继续遍历
	})

	if err != nil {
		c.logger.Error("Failed to recover retain messages", zap.Error(err))
	} else if count > 0 {
		c.logger.Info("Recovered retain message index", zap.Int("count", count))
	}
}

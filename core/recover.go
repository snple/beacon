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
		// 只存索引到 retainStore（防御存储中残留的异常数据：Packet/Properties 可能为 nil）
		if msg != nil && msg.Packet != nil && msg.Packet.Properties != nil {
			updatedAt := msg.Timestamp
			if updatedAt <= 0 && !msg.PacketID.IsZero() {
				updatedAt = msg.PacketID.Time().Unix()
			}
			meta := retainMeta{
				PayloadSize: len(msg.Packet.Payload),
				UpdatedAt:   updatedAt,
				QoS:         msg.QoS,
			}
			if err := c.retainStore.set(msg.Packet.Topic, msg.Packet.Properties.ExpiryTime, meta); err != nil {
				c.logger.Warn("Failed to recover retain index entry",
					zap.String("topic", msg.Packet.Topic),
					zap.Error(err))
			} else {
				count++
			}
		}
		return true // 继续遍历
	})

	if err != nil {
		c.logger.Error("Failed to recover retain messages", zap.Error(err))
	} else if count > 0 {
		c.logger.Info("Recovered retain message index", zap.Int("count", count))
	}
}

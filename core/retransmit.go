package core

import (
	"time"

	"go.uber.org/zap"
)

// retransmitLoop 定期触发消息重传
func (c *Core) retransmitLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.options.RetransmitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.triggerRetransmit()
		}
	}
}

// triggerRetransmit 让所有在线客户端重发超时未确认的投递。
//
// 这里只做「标记 + 唤醒」：真正写入由各连接的发送协程完成，
// 保证同一连接上的写入始终是串行的。
func (c *Core) triggerRetransmit() {
	c.clientsMu.RLock()
	clients := make([]*Client, 0, len(c.clients))
	for _, client := range c.clients {
		if !client.Closed() {
			clients = append(clients, client)
		}
	}
	c.clientsMu.RUnlock()

	for _, client := range clients {
		client.processRetransmit()
	}
}

// processRetransmit 把超过重传间隔仍未确认的投递标记为重发。
func (c *Client) processRetransmit() {
	conn := c.getConn()
	if conn == nil || conn.closed.Load() {
		return
	}

	interval := int64(c.core.options.RetransmitInterval.Seconds())
	if interval <= 0 {
		return
	}

	now := time.Now().Unix()

	scheduled := 0
	for _, d := range c.session.pendingAck.expired(now, interval) {
		if conn.addResend(d) {
			scheduled++
		}
	}

	if scheduled > 0 {
		c.core.logger.Debug("Retransmit scheduled",
			zap.String("clientID", c.ID),
			zap.Int("count", scheduled))
		conn.triggerSend()
	}
}

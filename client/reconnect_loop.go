package client

import (
	"time"

	"go.uber.org/zap"
)

// reconnectLoop listens for connection-loss signals from onConnectionLost() and performs
// automatic reconnection with exponential backoff.
//
// Design goals:
//   - Reconnect is transport-level (TCP/TLS) reconnection.
//   - Client-side state (subscriptions, sendQueue, persisted QoS1 store) is preserved.
//   - sendQueue remains intact across reconnections, allowing offline message buffering.
//   - In-flight waits are unblocked on disconnect by Connection.close(); reconnect does not try to
//     keep synchronous callers waiting across a disconnect.
func (c *Client) reconnectLoop() {
	defer c.wg.Done()

	initialDelay := 1 * time.Second
	maxDelay := 60 * time.Second

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.connLostCh:
			// Drain any extra signals to collapse bursts.
			drain := true
			for drain {
				select {
				case <-c.connLostCh:
				default:
					drain = false
				}
			}

			if !c.autoReconnect.Load() {
				continue
			}
			if !c.reconnecting.CompareAndSwap(false, true) {
				continue
			}

			// 获取当前队列数量
			var queuedMessages int
			conn := c.getConn()
			if conn != nil {
				queuedMessages = int(conn.sendQueue.used())
			}

			c.logger.Info("Starting automatic reconnection",
				zap.Duration("initialDelay", initialDelay),
				zap.Duration("maxDelay", maxDelay),
				zap.Int("queuedMessages", queuedMessages))

			currentDelay := initialDelay
			for {
				if !c.autoReconnect.Load() {
					c.logger.Info("Automatic reconnection disabled, stopping")
					break
				}

				// 使用 time.NewTimer + select 替代 time.After，确保 context 取消时立即响应
				timer := time.NewTimer(currentDelay)
				select {
				case <-c.ctx.Done():
					timer.Stop()
					c.reconnecting.Store(false)
					return
				case <-timer.C:
				}

				if !c.autoReconnect.Load() {
					c.logger.Info("Automatic reconnection disabled during backoff, stopping")
					break
				}

				c.logger.Info("Attempting to reconnect...",
					zap.Duration("delay", currentDelay))

				// 使用 dialer 重连（如果可用）
				var err error
				c.connMu.RLock()
				dialer := c.dialer
				c.connMu.RUnlock()

				if dialer != nil {
					err = c.ConnectWithDialer(dialer)
				} else {
					c.logger.Warn("No dialer available, cannot reconnect")
					break
				}

				if err != nil {
					// ConnectWithDialer() might return ErrAlreadyConnected if user connected in parallel.
					c.logger.Warn("Reconnection attempt failed",
						zap.Error(err))

					currentDelay *= 2
					if currentDelay > maxDelay {
						currentDelay = maxDelay
					}
					continue
				}

				// Connected.
				conn := c.getConn()
				sessionPresent := false
				queuedMessages := 0
				if conn != nil {
					sessionPresent = conn.sessionPresent
					queuedMessages = int(conn.sendQueue.used())
				}

				c.logger.Info("Reconnection successful",
					zap.Bool("sessionPresent", sessionPresent),
					zap.Int("queuedMessages", queuedMessages))

				// 会话未恢复时的订阅重放由 connectWithConn 统一完成
				// （手动 Connect 与自动重连走同一条路径，且保留原始订阅选项）。

				break
			}

			c.reconnecting.Store(false)
		}
	}
}

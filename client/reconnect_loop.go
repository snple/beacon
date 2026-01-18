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
//   - Client-side state (subscriptions/actions registry, sendQueue, persisted QoS1 store) is preserved.
//   - sendQueue remains intact across reconnections, allowing offline message buffering.
//   - In-flight waits are unblocked on disconnect by Connection.close(); reconnect does not try to
//     keep synchronous callers waiting across a disconnect.
func (c *Client) reconnectLoop() {
	initialDelay := 1 * time.Second
	maxDelay := 60 * time.Second

	for {
		select {
		case <-c.rootCtx.Done():
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

			// Snapshot session-related local state for restoration.
			c.subscribedTopicsMu.RLock()
			savedTopics := make([]string, 0, len(c.subscribedTopics))
			for topic := range c.subscribedTopics {
				savedTopics = append(savedTopics, topic)
			}
			c.subscribedTopicsMu.RUnlock()

			c.actionsMu.RLock()
			savedActions := make([]string, 0, len(c.registeredActions))
			for action := range c.registeredActions {
				savedActions = append(savedActions, action)
			}
			c.actionsMu.RUnlock()

			// 获取当前队列数量
			var queuedMessages int
			conn := c.getConn()
			if conn != nil {
				queuedMessages = int(conn.sendQueue.used.Load())
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
				select {
				case <-c.rootCtx.Done():
					c.reconnecting.Store(false)
					return
				case <-time.After(currentDelay):
				}

				if !c.autoReconnect.Load() {
					c.logger.Info("Automatic reconnection disabled during backoff, stopping")
					break
				}

				c.logger.Info("Attempting to reconnect...",
					zap.Duration("delay", currentDelay))

				err := c.Connect()
				if err != nil {
					// Connect() might return ErrAlreadyConnected if user connected in parallel.
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
					queuedMessages = int(conn.sendQueue.used.Load())
				}

				c.logger.Info("Reconnection successful",
					zap.Bool("sessionPresent", sessionPresent),
					zap.Int("queuedMessages", queuedMessages))

				// If the server did not restore the session, re-subscribe and re-register.
				if !sessionPresent {
					if len(savedTopics) > 0 {
						if err := c.Subscribe(savedTopics...); err != nil {
							c.logger.Warn("Failed to restore subscriptions", zap.Error(err))
						}
					}
					if len(savedActions) > 0 {
						if err := c.RegisterMultiple(savedActions); err != nil {
							c.logger.Warn("Failed to re-register actions", zap.Error(err))
						}
					}
				}

				break
			}

			c.reconnecting.Store(false)
		}
	}
}

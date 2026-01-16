package core

import (
	"context"
	"time"

	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// PollRequest 轮询获取客户端发来的请求（阻塞调用）
// 返回下一个请求，或在超时时返回 ErrPollTimeout
// 队列大小由 WithRequestQueueSize 配置参数决定（默认 100）
func (c *Core) PollRequest(ctx context.Context, timeout time.Duration) (*Request, error) {
	if !c.running.Load() {
		return nil, ErrCoreNotRunning
	}

	// 延迟初始化请求队列（仅第一次调用时）
	c.requestQueueMu.Lock()
	if !c.requestQueueInit {
		c.requestQueue = make(chan *Request, c.options.RequestQueueSize)
		c.requestQueueInit = true
	}
	c.requestQueueMu.Unlock()

	// 创建超时上下文
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case req := <-c.requestQueue:
		return req, nil
	case <-pollCtx.Done():
		if pollCtx.Err() == context.DeadlineExceeded {
			return nil, NewPollTimeoutError(timeout)
		}
		return nil, pollCtx.Err()
	case <-c.ctx.Done():
		return nil, ErrCoreShutdown
	}
}

// enqueueRequest 处理发送给 core 的请求（路径1：TargetClientID == "core"）
// enqueueRequest 将消息放入轮询队列（供 PollRequest 使用）
// 如果队列已初始化且未满，消息将被放入队列；否则丢弃
func (c *Client) enqueueRequest(p *packet.RequestPacket) error {
	c.core.requestQueueMu.Lock()
	if !c.core.requestQueueInit {
		c.core.requestQueueMu.Unlock()
		return nil // 队列未初始化（PollRequest 未被调用）
	}
	c.core.requestQueueMu.Unlock()

	coreReq := &Request{
		RequestID:      p.RequestID,
		Action:         p.Action,
		Payload:        p.Payload,
		SourceClientID: c.ID,
		core:           c.core,
	}

	if p.Properties != nil {
		coreReq.Timeout = p.Properties.Timeout
		coreReq.TraceID = p.Properties.TraceID
		coreReq.ContentType = p.Properties.ContentType
		coreReq.UserProperties = p.Properties.UserProperties
	}

	// 非阻塞尝试放入队列
	select {
	case c.core.requestQueue <- coreReq:
		// 成功放入队列，等待 core 端处理和响应
		return nil
	default:
		// 队列已满，返回错误响应
		resp := packet.NewResponsePacket(p.RequestID, c.ID, packet.ReasonServerBusy, nil)
		resp.Properties.ReasonString = "core request queue full"
		if err := c.WritePacket(resp); err != nil {
			c.core.logger.Warn("Failed to send RESPONSE",
				zap.String("clientID", c.ID),
				zap.Uint32("requestID", p.RequestID),
				zap.Error(err))

			return err
		}

		return nil
	}
}

// PollMessage 轮询获取客户端发布的消息（阻塞调用）
// 返回下一条消息，或在超时时返回 ErrPollTimeout
// 队列大小由 WithMessageQueueSize 配置参数决定（默认 100）
//
// 用法示例：
//
//	for {
//	    msg, err := core.PollMessage(ctx, 5*time.Second)
//	    if errors.Is(err, core.ErrPollTimeout) {
//	        continue // 超时，继续轮询
//	    }
//	    if err != nil {
//	        break // 处理错误
//	    }
//	    // 处理消息
//	    fmt.Printf("Received message on topic %s: %s\n", msg.Topic, msg.Payload)
//	}
func (c *Core) PollMessage(ctx context.Context, timeout time.Duration) (*Message, error) {
	if !c.running.Load() {
		return nil, ErrCoreNotRunning
	}

	// 延迟初始化消息队列（仅第一次调用时）
	c.messageQueueMu.Lock()
	if !c.messageQueueInit {
		c.messageQueue = make(chan Message, c.options.MessageQueueSize)
		c.messageQueueInit = true
	}
	c.messageQueueMu.Unlock()

	// 创建超时上下文
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case msg := <-c.messageQueue:
		return &msg, nil
	case <-pollCtx.Done():
		if pollCtx.Err() == context.DeadlineExceeded {
			return nil, NewPollTimeoutError(timeout)
		}
		return nil, pollCtx.Err()
	case <-c.ctx.Done():
		return nil, ErrCoreShutdown
	}
}

// enqueueMessage 处理发送给 core 的请求（路径1：TargetClientID == "core"）
// enqueueMessage 将消息放入轮询队列（供 PollMessage 使用）
// 如果队列已初始化且未满，消息将被放入队列；否则丢弃
func (c *Client) enqueueMessage(msg Message) error {
	c.core.messageQueueMu.Lock()
	if !c.core.messageQueueInit {
		c.core.messageQueueMu.Unlock()
		return nil // 队列未初始化（PollMessage 未被调用）
	}
	c.core.messageQueueMu.Unlock()

	// 非阻塞写入
	select {
	case c.core.messageQueue <- msg:
		// 消息已入队
		if msg.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(msg.PacketID, packet.ReasonSuccess)
			if err := c.WritePacket(puback); err != nil {
				c.core.logger.Warn("Failed to send PUBACK",
					zap.String("clientID", c.ID),
					zap.Uint16("packetID", msg.PacketID),
					zap.Error(err))

				return err
			}
		}
		return nil
	default:
		// 队列已满，丢弃消息（可以在这里添加日志或统计）
		c.core.logger.Debug("Message queue full, dropping message",
			zap.String("topic", msg.Packet.Topic))

		// 对于 QoS 1，仍需发送 PUBACK
		if msg.QoS == packet.QoS1 {
			puback := packet.NewPubackPacket(msg.PacketID, packet.ReasonServerBusy)
			puback.Properties.ReasonString = "core request queue full"
			if err := c.WritePacket(puback); err != nil {
				c.core.logger.Warn("Failed to send PUBACK",
					zap.String("clientID", c.ID),
					zap.Uint16("packetID", msg.PacketID),
					zap.Error(err))

				return err
			}
		}

		return nil
	}
}

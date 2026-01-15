package client

import (
	"context"
	"time"
)

// PollMessage 轮询获取接收到的消息（阻塞调用）
// 返回下一个消息，或在超时时返回 ErrPollTimeout
// 队列大小由 WithMessageQueueSize 配置参数决定（默认 100）
//
// 用法：
//
//	for {
//	    msg, err := c.PollMessage(ctx, 5*time.Second)
//	    if errors.Is(err, client.ErrPollTimeout) {
//	        continue // 超时，继续等待
//	    }
//	    if err != nil {
//	        break // 处理错误
//	    }
//	    // 处理消息
//	    fmt.Printf("Received: %s\n", msg.Topic)
//	}
func (c *Client) PollMessage(ctx context.Context, timeout time.Duration) (*Message, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	// 延迟初始化消息队列（仅第一次调用时）
	c.messageQueueMu.Lock()
	if !c.messageQueueInit {
		c.messageQueue = make(chan *Message, c.options.MessageQueueSize)
		c.messageQueueInit = true
	}
	c.messageQueueMu.Unlock()

	// 创建超时上下文
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case msg := <-c.messageQueue:
		return msg, nil
	case <-pollCtx.Done():
		if pollCtx.Err() == context.DeadlineExceeded {
			return nil, NewPollTimeoutError(timeout)
		}
		return nil, pollCtx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

// PollRequest 轮询获取接收到的请求（阻塞调用）
// 返回下一个请求，或在超时时返回 ErrPollTimeout
// 队列大小由 WithRequestQueueSize 配置参数决定（默认 100）
func (c *Client) PollRequest(ctx context.Context, timeout time.Duration) (*Request, error) {
	if !c.connected.Load() {
		return nil, ErrNotConnected
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
		req.client = c // 设置client引用，用于Response方法
		return req, nil
	case <-pollCtx.Done():
		if pollCtx.Err() == context.DeadlineExceeded {
			return nil, NewPollTimeoutError(timeout)
		}
		return nil, pollCtx.Err()
	case <-c.ctx.Done():
		return nil, ErrClientClosed
	}
}

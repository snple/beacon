package client

import (
	"context"
	"time"
)

// PollMessage 轮询获取接收到的消息（阻塞调用）
//
// 语义：
//   - 即使当前未连接，也会先返回已经缓冲的消息（例如连接刚断开）；
//   - 队列为空且未连接时返回 ErrNotConnected；
//   - timeout <= 0 表示不设置超时，一直阻塞到有消息、ctx 取消或客户端关闭；
//   - 超时返回 ErrPollTimeout。
//
// 队列大小由 WithMessageQueueSize 配置参数决定（默认 100）。
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
	// 先尝试取走已缓冲的消息：连接状态不应阻止应用处理已经收到的消息。
	select {
	case msg := <-c.messageQueue:
		return msg, nil
	default:
	}

	if !c.connected.Load() {
		return nil, ErrNotConnected
	}

	if timeout <= 0 {
		select {
		case msg := <-c.messageQueue:
			return msg, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.ctx.Done():
			return nil, ErrClientClosed
		}
	}

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

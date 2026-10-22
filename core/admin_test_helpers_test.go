package core

// 管理面已不再提供保留消息/订阅的枚举 API；以下辅助函数让内部测试
// 仍能直接检查 broker 内部状态。

// retainedMessagesForTest 返回匹配 topic（支持通配符）的保留消息。
func retainedMessagesForTest(c *Core, topic string) []*Message {
	var out []*Message
	for _, t := range c.retainStore.matchTopics(topic) {
		msg, err := c.store.getRetain(t)
		if err != nil || msg == nil {
			continue
		}
		out = append(out, msg)
	}
	return out
}

// getClientSubscriptionsForTest 返回客户端的订阅列表（等价于详情中的 Subscriptions）。
func getClientSubscriptionsForTest(c *Core, clientID string) ([]ClientSubscriptionInfo, error) {
	info, err := c.GetClientInfo(clientID)
	if err != nil {
		return nil, err
	}
	return info.Subscriptions, nil
}

package core

// 管理面已不再提供保留消息/订阅的枚举 API；以下辅助函数让内部测试
// 仍能直接检查 broker 内部状态。

// retainedMessagesForTest 返回匹配 topic（支持通配符）的保留消息。
// 返回的消息额外持有一个引用，调用方负责 release。
func retainedMessagesForTest(c *Core, topic string) []*Message {
	return c.retainStore.matchMessages(topic)
}

// getClientSubscriptionsForTest 返回客户端的订阅列表（等价于详情中的 Subscriptions）。
func getClientSubscriptionsForTest(c *Core, clientID string) ([]ClientSubscriptionInfo, error) {
	info, err := c.GetClientInfo(clientID)
	if err != nil {
		return nil, err
	}
	return info.Subscriptions, nil
}

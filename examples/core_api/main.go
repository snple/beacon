// Package main 演示 Core 管理 API 的使用
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
)

func main() {
	// 创建并启动 Core
	c, err := core.NewWithOptions(core.NewCoreOptions())
	if err != nil {
		log.Fatal("Failed to create core:", err)
	}
	if err := c.Start(); err != nil {
		log.Fatal(err)
	}
	defer c.Stop()

	// 在后台启动TCP服务
	go func() {
		if err := c.ServeTCP(":3883"); err != nil {
			log.Fatal("TCP server error:", err)
		}
	}()

	fmt.Println("Core started on :3883")
	fmt.Println("=== Core 管理 API 示例 ===")

	// 等待一些客户端连接
	time.Sleep(2 * time.Second)

	// 1. 获取在线客户端数量
	fmt.Println("1. 获取客户端统计:")
	fmt.Printf("   在线客户端: %d\n", c.GetOnlineClientsCount())
	fmt.Printf("   总客户端: %d\n", c.GetTotalClientsCount())

	// 2. 列出所有客户端
	fmt.Println("2. 列出所有客户端:")
	clients := c.ListClients()
	for _, client := range clients {
		fmt.Printf("   - ClientID: %s\n", client.ClientID)
		fmt.Printf("     地址: %s\n", client.RemoteAddr)
		fmt.Printf("     在线: %v\n", client.Connected)
		fmt.Printf("     心跳间隔: %d 秒\n", client.KeepAlive)
		fmt.Printf("     等待确认: %d\n", client.PendingAck)
		fmt.Printf("     订阅数: %d\n", len(client.Subscriptions))
		for _, sub := range client.Subscriptions {
			fmt.Printf("       * %s (QoS %d)\n", sub.Topic, sub.QoS)
		}
	}

	// 3. 检查特定客户端在线状态
	testClientID := "test-client"
	fmt.Printf("3. 检查客户端 '%s' 在线状态:\n", testClientID)
	if c.IsClientOnline(testClientID) {
		fmt.Println("   客户端在线")

		// 获取详细信息
		if info, err := c.GetClientInfo(testClientID); err == nil {
			data, _ := json.MarshalIndent(info, "   ", "  ")
			fmt.Println("   详细信息:")
			fmt.Println(string(data))
		}
	} else {
		fmt.Println("   客户端离线")
	}

	// 4. 向指定客户端发送消息
	fmt.Println("4. 向指定客户端发送消息:")
	if c.IsClientOnline(testClientID) {
		opts := core.PublishOptions{
			QoS:         packet.QoS1,
			TraceID:     "admin-msg-001",
			ContentType: "text/plain",
			Expiry:      300, // 5 分钟过期
		}

		err := c.PublishToClient(
			testClientID,
			"admin/command",
			[]byte("系统通知：请更新配置"),
			opts,
		)

		if err != nil {
			fmt.Printf("   发送失败: %v\n", err)
		} else {
			fmt.Println("   消息已发送")
		}
	}

	// 5. 广播消息到所有在线客户端
	fmt.Println("5. 向所有客户端广播消息:")
	opts := core.PublishOptions{
		QoS:         packet.QoS0,
		TraceID:     "broadcast-001",
		ContentType: "application/json",
		Expiry:      60,
	}

	successCount := c.Broadcast(
		"system/broadcast",
		[]byte(`{"type":"announcement","message":"系统维护通知"}`),
		opts,
	)
	fmt.Printf("   成功发送到 %d 个客户端\n", successCount)

	// 6. 获取主题订阅者
	fmt.Println("6. 获取主题订阅者:")
	topic := "sensors/temperature"
	subscribers := c.GetTopicSubscribers(topic)
	fmt.Printf("   主题 '%s' 的订阅者:\n", topic)
	for _, opts := range subscribers {
		fmt.Printf("   - %v\n", opts)
	}

	// 7. 获取客户端订阅列表
	fmt.Println("7. 获取客户端订阅列表:")
	if len(clients) > 0 {
		clientID := clients[0].ClientID
		subs, err := c.GetClientSubscriptions(clientID)
		if err != nil {
			fmt.Printf("   错误: %v\n", err)
		} else {
			fmt.Printf("   客户端 '%s' 的订阅:\n", clientID)
			for _, sub := range subs {
				fmt.Printf("   - %s (QoS %d)\n", sub.Topic, sub.QoS)
			}
		}
	}

	// 8. 获取统计信息
	fmt.Println("8. Core 统计信息:")
	stats := c.GetStats()
	fmt.Printf("   连接总数: %d\n", stats.ClientsTotal)
	fmt.Printf("   当前连接: %d\n", stats.ClientsConnected)
	fmt.Printf("   已接收消息: %d\n", stats.MessagesReceived)
	fmt.Printf("   已发送消息: %d\n", stats.MessagesSent)
	fmt.Printf("   已丢弃消息: %d\n", stats.MessagesDropped)
	fmt.Printf("   接收字节数: %d\n", stats.BytesReceived)
	fmt.Printf("   发送字节数: %d\n", stats.BytesSent)
	fmt.Printf("   订阅数: %d\n", stats.SubscriptionsCount)

	// 9. 断开指定客户端（可选）
	fmt.Println("9. 断开客户端连接 (演示):")
	if len(clients) > 0 {
		clientID := clients[0].ClientID
		fmt.Printf("   可以使用以下代码断开客户端 '%s':\n", clientID)
		fmt.Printf("   core.DisconnectClient(\"%s\", protocol.ReasonAdministrativeAction)\n", clientID)

		// 取消注释以实际断开连接
		// err := core.DisconnectClient(clientID, protocol.ReasonAdministrativeAction)
		// if err != nil {
		//     fmt.Printf("   断开失败: %v\n", err)
		// } else {
		//     fmt.Println("   客户端已断开")
		// }
	}

	fmt.Println("=== 示例结束 ===")
	fmt.Println("按 Ctrl+C 停止 Core")

	// 保持运行
	select {}
}

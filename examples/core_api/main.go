// Package main 演示 Core 管理 API 的使用
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
)

func main() {
	// 创建并启动 Core
	c, err := core.NewWithOptions(core.NewCoreOptions().
		WithBuildInfo("1.0.0", "example", "2026-01-01T00:00:00Z"))
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

	// 2. 列出所有客户端（轻量概要，不分页）
	fmt.Println("2. 列出所有客户端（概要）:")
	clients := c.ListClients()
	for _, summary := range clients {
		fmt.Printf("   - %s online=%v addr=%q lastSeen=%s expires=%s\n",
			summary.ClientID, summary.Connected, summary.RemoteAddr,
			summary.LastSeenAt.Format(time.RFC3339), formatTime(summary.ExpiresAt))
	}

	// 3. 检查特定客户端在线状态
	testClientID := "test-client"
	fmt.Printf("3. 检查客户端 '%s' 在线状态:\n", testClientID)
	if c.IsClientOnline(testClientID) {
		fmt.Println("   客户端在线")
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

	// 6. 客户端详情（配置 + 运行状态 + 统计）
	fmt.Println("6. 客户端详情:")
	if len(clients) > 0 {
		clientID := clients[0].ClientID
		info, err := c.GetClientInfo(clientID)
		if err != nil {
			fmt.Printf("   错误: %v\n", err)
		} else {
			fmt.Printf("   客户端 '%s': transport=%s protocol=%s keepAlive=%d sessionTimeout=%d\n",
				info.ClientID, info.Transport, info.ProtocolVersion, info.KeepAlive, info.SessionTimeout)
			fmt.Printf("   队列: size=%d bytes=%d oldest=%s 在途: ack=%d oldest=%s\n",
				info.QueueSize, info.QueueBytes, info.OldestQueuedAge,
				info.PendingAck, info.OldestPendingAckAge)
			fmt.Printf("   计数: sent=%d dropped=%d rejected=%d bytesOut=%d bytesIn=%d\n",
				info.MessagesSent, info.MessagesDropped, info.MessagesRejected,
				info.BytesSent, info.BytesReceived)
			for _, sub := range info.Subscriptions {
				fmt.Printf("   订阅: %s (QoS %d)\n", sub.Topic, sub.QoS)
			}
		}
	}

	// 7. 获取统计信息
	fmt.Println("7. Core 统计信息:")
	stats := c.GetStats()
	fmt.Printf("   累计会话数: %d\n", stats.ClientsTotal)
	fmt.Printf("   当前连接: %d\n", stats.ClientsConnected)
	fmt.Printf("   已接收消息: %d\n", stats.MessagesReceived)
	fmt.Printf("   已发送消息: %d\n", stats.MessagesSent)
	fmt.Printf("   已丢弃消息: %d\n", stats.MessagesDropped)
	fmt.Printf("   接收字节数: %d\n", stats.BytesReceived)
	fmt.Printf("   发送字节数: %d\n", stats.BytesSent)
	fmt.Printf("   订阅数: %d\n", stats.SubscriptionsCount)

	// 8. 服务器信息
	fmt.Println("8. 服务器信息:")
	info := c.Info()
	fmt.Printf("   版本: %s（commit %s, built %s, 协议 %s）\n",
		info.Version, info.Commit, info.BuildTime, info.Protocol)
	fmt.Printf("   运行时长: %s\n", info.Uptime)
	fmt.Printf("   监听器: %v\n", info.Listeners)
	fmt.Printf("   存储: path=%q ready=%v bytes=%d\n", info.Store.Path, info.Store.Ready, info.Store.Bytes)

	fmt.Println("9. 以 broker 身份发布（按订阅树投递）:")
	res, err := c.Publish("sensors/temperature", []byte(`{"value":25.6}`), core.PublishOptions{
		QoS:     packet.QoS0,
		TraceID: "admin-publish-001",
		Expiry:  60,
	})
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		fmt.Printf("   命中订阅者=%d 已入队=%v 已保留=%v\n",
			res.MatchedSubscribers, res.Enqueued, res.Retained)
	}

	fmt.Println("10. 订阅事件流（3 秒）:")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	stream, err := c.SubscribeEvents(ctx, core.EventOptions{Buffer: 256})
	if err != nil {
		fmt.Printf("   错误: %v\n", err)
	} else {
		for ev := range stream.Events() {
			fmt.Printf("   #%d %s client=%s topic=%s reason=%s\n",
				ev.Seq, ev.Type, ev.ClientID, ev.Topic, ev.Reason)
		}
		fmt.Printf("   丢弃: %d\n", stream.Dropped())
		stream.Close()
	}

	// 11. 断开指定客户端（可选）
	fmt.Println("11. 断开客户端连接 (演示):")
	if len(clients) > 0 {
		clientID := clients[0].ClientID
		fmt.Printf("   可以使用以下代码断开客户端 '%s':\n", clientID)
		fmt.Printf("   core.DisconnectClientAndWait(ctx, \"%s\", packet.ReasonBanned)\n", clientID)

		// 取消注释以实际断开连接
		// err := c.DisconnectClientAndWait(context.Background(), clientID, packet.ReasonBanned)
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

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}

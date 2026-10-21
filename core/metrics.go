package core

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ServeMetrics 在 address 上启动运维 HTTP 服务（阻塞，直到 Core 停止或服务关闭）。
//
// 提供的端点：
//   - GET /healthz  存活探针：Core 运行中返回 200 "ok"，否则 503
//   - GET /metrics  Prometheus 文本格式指标
//   - GET /stats    JSON 格式统计快照（便于人工排查）
func (c *Core) ServeMetrics(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	return c.ServeMetricsListener(listener)
}

// ServeMetricsListener 在已有 listener 上提供运维端点（阻塞）。
func (c *Core) ServeMetricsListener(listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", c.handleHealthz)
	mux.HandleFunc("/metrics", c.handleMetrics)
	mux.HandleFunc("/stats", c.handleStatsJSON)

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	stopClose := make(chan struct{})
	defer close(stopClose)

	go func() {
		select {
		case <-c.ctx.Done():
			_ = srv.Close()
		case <-stopClose:
		}
	}()

	id := c.registerListener(listener.Addr().String(), "metrics")
	defer c.unregisterListener(id)

	c.logger.Info("Started metrics listener",
		zap.String("address", listener.Addr().String()))

	err := srv.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (c *Core) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if !c.running.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("stopped\n"))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok\n"))
}

// statsCacheTTL 是 /metrics 与 /stats 的共享快照缓存时间。
// GetStats 需要遍历所有客户端，避免高频抓取造成重复全量扫描。
const statsCacheTTL = time.Second

// cachedStats 返回最多 statsCacheTTL 之前的统计快照。
func (c *Core) cachedStats() StatsSnapshot {
	c.statsCacheMu.Lock()
	defer c.statsCacheMu.Unlock()

	if !c.statsCacheAt.IsZero() && time.Since(c.statsCacheAt) < statsCacheTTL {
		return c.statsCache
	}

	c.statsCache = c.GetStats()
	c.statsCacheAt = time.Now()
	return c.statsCache
}

func (c *Core) handleStatsJSON(w http.ResponseWriter, _ *http.Request) {
	s := c.cachedStats()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = fmt.Fprintf(w, `{"clients_connected":%d,"clients_total":%d,`+
		`"messages_received":%d,"messages_sent":%d,"messages_dropped":%d,"messages_rejected":%d,`+
		`"bytes_received":%d,"bytes_sent":%d,"subscriptions":%d,`+
		`"pending_queue_depth":%d,"client_queue_depth":%d,"pending_ack":%d,`+
		`"offline_sessions":%d,"store_bytes":%d,"events_dropped":%d,"up":%d}`+"\n",
		s.ClientsConnected, s.ClientsTotal,
		s.MessagesReceived, s.MessagesSent, s.MessagesDropped, s.MessagesRejected,
		s.BytesReceived, s.BytesSent, s.SubscriptionsCount,
		s.PendingQueueDepth, s.ClientQueueDepth, s.PendingAckTotal,
		s.OfflineSessions, s.StoreBytes, s.EventsDropped, boolToInt(c.running.Load()))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c *Core) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	s := c.cachedStats()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	writeMetric(w, "beacon_up", "gauge", "Core 是否在运行（1=运行中）", float64(boolToInt(c.running.Load())))
	writeMetric(w, "beacon_clients_connected", "gauge", "当前在线客户端数", float64(s.ClientsConnected))
	writeMetric(w, "beacon_clients_total", "counter", "累计新建的会话数（单调递增）", float64(s.ClientsTotal))
	writeMetric(w, "beacon_messages_received_total", "counter", "累计接收消息数", float64(s.MessagesReceived))
	writeMetric(w, "beacon_messages_sent_total", "counter", "累计投递消息数", float64(s.MessagesSent))
	writeMetric(w, "beacon_messages_dropped_total", "counter", "累计丢弃消息数（过期/配额/订阅端拒收）", float64(s.MessagesDropped))
	writeMetric(w, "beacon_messages_rejected_total", "counter", "累计接收侧拒绝数（限速/配额/钩子拒绝）", float64(s.MessagesRejected))
	writeMetric(w, "beacon_bytes_received_total", "counter", "累计接收字节数", float64(s.BytesReceived))
	writeMetric(w, "beacon_bytes_sent_total", "counter", "累计发送字节数", float64(s.BytesSent))
	writeMetric(w, "beacon_subscriptions", "gauge", "当前订阅数", float64(s.SubscriptionsCount))
	writeMetric(w, "beacon_pending_queue_depth", "gauge", "全局待分发队列深度", float64(s.PendingQueueDepth))
	writeMetric(w, "beacon_client_queue_depth", "gauge", "所有客户端待发送队列深度之和", float64(s.ClientQueueDepth))
	writeMetric(w, "beacon_pending_ack", "gauge", "所有客户端未确认的 QoS1 消息数", float64(s.PendingAckTotal))
	writeMetric(w, "beacon_offline_sessions", "gauge", "保留中的离线会话数", float64(s.OfflineSessions))
	writeMetric(w, "beacon_store_bytes", "gauge", "存储占用（字节）", float64(s.StoreBytes))
	writeMetric(w, "beacon_events_dropped_total", "counter", "事件流因订阅者消费过慢而丢弃的事件数", float64(s.EventsDropped))
}

// writeMetric 输出单条 Prometheus 文本格式指标。
func writeMetric(w http.ResponseWriter, name, kind, help string, value float64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %g\n", name, help, name, kind, name, value)
}

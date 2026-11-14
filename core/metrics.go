package core

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const statsCacheTTL = time.Second

// ServeMetrics 启动运维端点（/healthz、/metrics、/stats）。
func (c *Core) ServeMetrics(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer func() { _ = listener.Close() }()
	return c.ServeMetricsListener(listener)
}

// ServeMetricsListener 在已有 listener 上提供运维端点。
func (c *Core) ServeMetricsListener(listener net.Listener) error {
	id := c.registerListener(listener.Addr().String(), "http")
	defer c.unregisterListener(id)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", c.handleHealthz)
	mux.HandleFunc("/metrics", c.handleMetrics)
	mux.HandleFunc("/stats", c.handleStatsJSON)

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-c.ctx.Done():
			_ = srv.Close()
		case <-stop:
		}
	}()

	if err := srv.Serve(listener); err != nil && c.running.Load() {
		if err != http.ErrServerClosed {
			return err
		}
	}
	return nil
}

func (c *Core) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	if !c.running.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("stopped"))
		return
	}
	_, _ = w.Write([]byte("ok"))
}

func (c *Core) handleStatsJSON(w http.ResponseWriter, _ *http.Request) {
	snap := c.cachedStats()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}

// cachedStats 在一秒内复用快照，避免高频抓取时重复遍历客户端。
func (c *Core) cachedStats() StatsSnapshot {
	c.statsCacheMu.Lock()
	defer c.statsCacheMu.Unlock()

	if time.Since(c.statsCacheAt) < statsCacheTTL {
		return c.statsCache
	}
	c.statsCache = c.GetStats()
	c.statsCacheAt = time.Now()
	return c.statsCache
}

func (c *Core) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	snap := c.cachedStats()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	writeMetric(w, "beacon_clients_connected", "gauge", "当前在线连接数", float64(snap.ClientsConnected))
	writeMetric(w, "beacon_clients_total", "counter", "累计创建的会话数", float64(snap.ClientsTotal))
	writeMetric(w, "beacon_sessions", "gauge", "当前会话总数（含离线保留）", float64(snap.Sessions))
	writeMetric(w, "beacon_messages_received_total", "counter", "接收的 PUSH 记录数", float64(snap.MessagesReceived))
	writeMetric(w, "beacon_messages_sent_total", "counter", "PULL 发出的记录数", float64(snap.MessagesSent))
	writeMetric(w, "beacon_messages_dropped_total", "counter", "队列溢出丢弃数", float64(snap.MessagesDropped))
	writeMetric(w, "beacon_messages_rejected_total", "counter", "接收侧拒绝数", float64(snap.MessagesRejected))
	writeMetric(w, "beacon_messages_no_match_total", "counter", "无匹配客户端的消息数", float64(snap.MessagesNoMatch))
	writeMetric(w, "beacon_bytes_received_total", "counter", "接收字节数", float64(snap.BytesReceived))
	writeMetric(w, "beacon_bytes_sent_total", "counter", "发送字节数", float64(snap.BytesSent))
	writeMetric(w, "beacon_duplicates_sent_total", "counter", "重复投递（delivered 重发）数", float64(snap.DuplicatesSent))
	writeMetric(w, "beacon_messages_nacked_total", "counter", "客户端 NACK 的消息数", float64(snap.MessagesNacked))
	writeMetric(w, "beacon_messages_expired_total", "counter", "过期丢弃的消息数", float64(snap.MessagesExpired))
	writeMetric(w, "beacon_commit_required_total", "counter", "触发无进展惩罚的次数", float64(snap.CommitRequired))
	writeMetric(w, "beacon_session_takeovers_total", "counter", "会话接管次数", float64(snap.SessionTakeovers))
	writeMetric(w, "beacon_target_misses_total", "counter", "定向投递目标不存在次数", float64(snap.TargetMisses))
	writeMetric(w, "beacon_inflight_messages", "gauge", "内存中未完成投递的消息条数", float64(snap.InFlightMessages))
	writeMetric(w, "beacon_inflight_bytes", "gauge", "内存中未完成投递消息的近似字节数", float64(snap.InFlightBytes))
	writeMetric(w, "beacon_wildcard_filters", "gauge", "参与广播匹配的通配符过滤器数量", float64(snap.WildcardFilters))
}

func writeMetric(w http.ResponseWriter, name, kind, help string, value float64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %v\n", name, help, name, kind, name, value)
}

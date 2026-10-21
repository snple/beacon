package core

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMetrics_Endpoints 验证 /healthz、/metrics、/stats 三个运维端点。
func TestMetrics_Endpoints(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	errCh := make(chan error, 1)
	go func() { errCh <- c.ServeMetricsListener(ln) }()

	base := "http://" + ln.Addr().String()

	// /healthz
	require.Eventually(t, func() bool {
		resp, err := http.Get(base + "/healthz")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok"
	}, 3*time.Second, 20*time.Millisecond, "healthz 应返回 200 ok")

	// /metrics
	resp, err := http.Get(base + "/metrics")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	text := string(body)
	for _, name := range []string{
		"beacon_up", "beacon_clients_connected", "beacon_messages_received_total",
		"beacon_pending_queue_depth", "beacon_client_queue_depth", "beacon_pending_ack",
		"beacon_offline_sessions", "beacon_store_bytes", "beacon_subscriptions",
	} {
		require.Contains(t, text, "# TYPE "+name, "缺少指标 %s", name)
	}
	require.Contains(t, text, "beacon_up 1")

	// /stats
	resp, err = http.Get(base + "/stats")
	require.NoError(t, err)
	var stats map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&stats))
	resp.Body.Close()
	require.Equal(t, float64(1), stats["up"])

	// Core 停止后 metrics 服务应返回
	c.Stop()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("Core 停止后 metrics 服务未退出")
	}

	// 停止后 healthz 应返回 503（直接调用 handler 校验）
	rec := httptest.NewRecorder()
	c.handleHealthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

// TestMetrics_StatsIncludeRuntimeDepth 验证 StatsSnapshot 暴露队列深度等关键指标。
func TestMetrics_StatsIncludeRuntimeDepth(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	addr := testServe(t, c)

	sub := testSetupClient(t, addr, "metrics-sub", nil)
	defer sub.Close()
	require.NoError(t, sub.Subscribe("metrics/topic"))

	pub := testSetupClient(t, addr, "metrics-pub", nil)
	defer pub.Close()
	require.NoError(t, pub.Publish("metrics/topic", []byte("hello"), nil))

	require.Eventually(t, func() bool {
		return c.GetStats().MessagesReceived >= 1
	}, 3*time.Second, 20*time.Millisecond, "应记录到接收消息")

	stats := c.GetStats()
	require.GreaterOrEqual(t, stats.PendingQueueDepth, int64(0))
	require.GreaterOrEqual(t, stats.ClientQueueDepth, int64(0))
	require.GreaterOrEqual(t, stats.StoreBytes, int64(0))
}

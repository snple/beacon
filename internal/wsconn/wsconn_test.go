package wsconn_test

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/snple/beacon/internal/wsconn"
)

// newWSServer 启动一个测试用 WebSocket 服务端，handler 在升级完成后执行。
func newWSServer(t *testing.T, handler func(*websocket.Conn)) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = ws.Close() }()
		handler(ws)
	}))
	t.Cleanup(srv.Close)

	return srv
}

// dialWS 连接测试服务端并包装为 net.Conn。
func dialWS(t *testing.T, srv *httptest.Server) *wsconn.Conn {
	t.Helper()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn := wsconn.New(ws)
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// TestReadAcrossMessages 验证跨 WebSocket 消息的字节流语义，且文本消息被跳过。
func TestReadAcrossMessages(t *testing.T) {
	srv := newWSServer(t, func(ws *websocket.Conn) {
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte("hello"))
		_ = ws.WriteMessage(websocket.TextMessage, []byte("ignored"))
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte("world"))
	})

	conn := dialWS(t, srv)

	buf := make([]byte, len("helloworld"))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if got := string(buf); got != "helloworld" {
		t.Fatalf("got %q, want %q", got, "helloworld")
	}
}

// TestReadZeroLength 验证零长度读取不会阻塞。
func TestReadZeroLength(t *testing.T) {
	srv := newWSServer(t, func(ws *websocket.Conn) {
		_ = ws.WriteMessage(websocket.BinaryMessage, []byte("x"))
	})

	conn := dialWS(t, srv)
	n, err := conn.Read(nil)
	if n != 0 || err != nil {
		t.Fatalf("Read(nil) = (%d, %v), want (0, nil)", n, err)
	}
}

// TestWrite 验证 Write 以二进制消息发出并返回完整长度。
func TestWrite(t *testing.T) {
	received := make(chan []byte, 1)
	srv := newWSServer(t, func(ws *websocket.Conn) {
		_, data, err := ws.ReadMessage()
		if err != nil {
			return
		}
		received <- data
	})

	conn := dialWS(t, srv)
	payload := []byte("ping")
	n, err := conn.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("Write returned %d, want %d", n, len(payload))
	}

	select {
	case got := <-received:
		if string(got) != string(payload) {
			t.Fatalf("server got %q, want %q", got, payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not receive payload")
	}
}

// TestSetReadLimit 验证超大消息在读取时被拒绝。
func TestSetReadLimit(t *testing.T) {
	srv := newWSServer(t, func(ws *websocket.Conn) {
		_ = ws.WriteMessage(websocket.BinaryMessage, make([]byte, 1024))
	})

	conn := dialWS(t, srv)
	conn.SetReadLimit(16)

	if _, err := conn.Read(make([]byte, 64)); !errors.Is(err, websocket.ErrReadLimit) {
		t.Fatalf("Read error = %v, want ErrReadLimit", err)
	}
}

// TestReadDeadline 验证读截止时间会被透传到底层 WebSocket。
func TestReadDeadline(t *testing.T) {
	srv := newWSServer(t, func(ws *websocket.Conn) {
		// 保持连接打开，让客户端的读取超时生效
		time.Sleep(2 * time.Second)
	})

	conn := dialWS(t, srv)
	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}

	_, err := conn.Read(make([]byte, 8))
	if err == nil {
		t.Fatal("expected read timeout error")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("error = %v, want net timeout", err)
	}
}

package core

import (
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestServeWS_RejectsOversizedFrame 验证 WebSocket 帧大小受 MaxPacketSize 限制：
// 超限帧应在被完整读入内存之前被拒绝并关闭连接。
func TestServeWS_RejectsOversizedFrame(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions().WithMaxPacketSize(1024))
	defer c.Stop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() {
		_ = c.ServeWSListener(listener, "/ws")
	}()

	ws, _, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer ws.Close()

	if err := ws.WriteMessage(websocket.BinaryMessage, make([]byte, 4096)); err != nil {
		t.Fatalf("write oversized frame: %v", err)
	}

	_ = ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := ws.ReadMessage(); err == nil {
		t.Fatal("expected connection to be closed after oversized frame")
	}
}

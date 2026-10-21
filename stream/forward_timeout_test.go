package stream

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// TestForwardDataHonorsWriteTimeout 确认数据面转发受 writeTimeout 约束：
// 对端不读数据时，转发协程必须因写超时退出，而不是永久阻塞。
func TestForwardDataHonorsWriteTimeout(t *testing.T) {
	b := &Core{writeTimeout: 50 * time.Millisecond}

	src, srcPeer := net.Pipe()
	defer src.Close()
	defer srcPeer.Close()

	// dst 的对端永不读取，Write 会一直阻塞到写截止时间。
	dst, dstPeer := net.Pipe()
	defer dst.Close()
	defer dstPeer.Close()

	var counter atomic.Uint64
	done := make(chan struct{})
	go func() {
		b.forwardData(src, dst, &counter)
		close(done)
	}()

	go func() {
		_, _ = srcPeer.Write([]byte("payload"))
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("forwardData did not honor the write timeout")
	}
}

package core_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"github.com/snple/beacon/stream"
	"go.uber.org/zap"
)

// startStreamCore 启动 core、主协议监听器和 stream 数据面监听器。
func startStreamCore(t *testing.T, opts *core.CoreOptions) (b *core.Core, mainAddr, streamAddr string) {
	t.Helper()
	if opts == nil {
		opts = core.NewCoreOptions()
	}
	opts.WithLogger(zap.NewNop())

	b, err := core.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("start core: %v", err)
	}

	main, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen main: %v", err)
	}
	streamListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen stream: %v", err)
	}

	go func() { _ = b.Serve(main) }()
	go func() { _ = b.ServeStream(streamListener) }()

	// ServeStream 在独立 goroutine 中注册监听器；等它完成再让客户端连接，
	// 否则 CONNACK 可能看到 StreamEnabled=false。
	deadline := time.Now().Add(2 * time.Second)
	for b.Info().StreamAddress == "" {
		if time.Now().After(deadline) {
			t.Fatal("stream listener did not start")
		}
		time.Sleep(time.Millisecond)
	}

	t.Cleanup(func() { _ = b.Stop() })
	return b, main.Addr().String(), streamListener.Addr().String()
}

func newStreamClient(t *testing.T, addr, id string) *client.Client {
	t.Helper()
	opts := client.NewClientOptions().
		WithCore(addr).
		WithClientID(id).
		WithAutoReconnect(false).
		WithStream(true).
		WithLogger(zap.NewNop())

	c, err := client.NewWithOptions(opts)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.ConnectContext(ctx); err != nil {
		t.Fatalf("connect %s: %v", id, err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func waitStreams(t *testing.T, b *core.Core, want int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if b.GetStats().StreamsActive == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("streams active = %d, want %d", b.GetStats().StreamsActive, want)
}

func TestStreamOpenEcho(t *testing.T) {
	opts := core.NewCoreOptions().WithStreamEnabled(true)
	b, addr, _ := startStreamCore(t, opts)

	accepted := make(chan *stream.Stream, 1)
	bClient := newStreamClient(t, addr, "b")
	bClient.OnStreamRequest(func(req *client.StreamRequest) {
		s, err := req.Accept(context.Background(), []byte("welcome"), map[string]string{"ok": "1"})
		if err != nil {
			t.Logf("accept stream: %v", err)
			return
		}
		accepted <- s
	})

	if !bClient.StreamSupported() || !bClient.StreamEnabled() {
		t.Fatal("b should support and have stream enabled")
	}

	aClient := newStreamClient(t, addr, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	aStream, err := aClient.OpenStream(ctx, "b", "chat/1", []byte("hello-meta"), map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer aStream.Close()

	if aStream.PeerClientID() != "b" || aStream.LocalClientID() != "a" {
		t.Fatalf("unexpected stream endpoints: %s -> %s", aStream.LocalClientID(), aStream.PeerClientID())
	}
	if aStream.Topic() != "chat/1" {
		t.Fatalf("topic = %q", aStream.Topic())
	}
	if string(aStream.Metadata()) != "welcome" {
		t.Fatalf("metadata = %q", aStream.Metadata())
	}

	var bStream *stream.Stream
	select {
	case bStream = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("target did not accept stream")
	}
	defer bStream.Close()

	if string(bStream.Metadata()) != "hello-meta" || bStream.Properties()["k"] != "v" {
		t.Fatalf("target metadata/properties = %q / %v", bStream.Metadata(), bStream.Properties())
	}

	if _, err := aStream.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(bStream, buf); err != nil {
		t.Fatalf("target read: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("target read %q", buf)
	}
	if _, err := bStream.Write([]byte("pong")); err != nil {
		t.Fatalf("target write: %v", err)
	}
	if _, err := io.ReadFull(aStream, buf); err != nil {
		t.Fatalf("initiator read: %v", err)
	}
	if string(buf) != "pong" {
		t.Fatalf("initiator read %q", buf)
	}

	waitStreams(t, b, 1)
	streams := b.Streams()
	if len(streams) != 1 {
		t.Fatalf("streams = %d", len(streams))
	}
	if streams[0].Source != "a" || streams[0].Target != "b" || streams[0].Topic != "chat/1" {
		t.Fatalf("unexpected stream info: %+v", streams[0])
	}

	if stats := b.GetStats(); stats.StreamsTotal != 1 || stats.StreamBytesBToA != 4 {
		t.Fatalf("unexpected stream stats: %+v", stats)
	}

	_ = aStream.Close()
	_ = bStream.Close()
	waitStreams(t, b, 0)
}

func TestStreamDisabledByDefault(t *testing.T) {
	_, addr, _ := startStreamCore(t, nil)

	c := newStreamClient(t, addr, "a")
	if c.StreamEnabled() {
		t.Fatal("stream should be disabled by default")
	}
	if !c.StreamSupported() {
		t.Fatal("server should implement stream")
	}
	if _, err := c.OpenStream(context.Background(), "b", "x", nil, nil); !errors.Is(err, client.ErrStreamNotSupported) {
		t.Fatalf("expected ErrStreamNotSupported, got %v", err)
	}
}

func TestStreamTargetUnavailable(t *testing.T) {
	opts := core.NewCoreOptions().WithStreamEnabled(true)
	_, addr, _ := startStreamCore(t, opts)

	aClient := newStreamClient(t, addr, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := aClient.OpenStream(ctx, "missing", "x", nil, nil)
	if !errors.Is(err, client.ErrStreamTargetUnavailable) {
		t.Fatalf("expected ErrStreamTargetUnavailable, got %v", err)
	}
}

func TestStreamRejected(t *testing.T) {
	opts := core.NewCoreOptions().WithStreamEnabled(true)
	_, addr, _ := startStreamCore(t, opts)

	bClient := newStreamClient(t, addr, "b")
	bClient.OnStreamRequest(func(req *client.StreamRequest) {
		_ = req.Reject(packet.ReasonStreamRejected, "no thanks")
	})

	aClient := newStreamClient(t, addr, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := aClient.OpenStream(ctx, "b", "x", nil, nil); !errors.Is(err, client.ErrStreamRejected) {
		t.Fatalf("expected ErrStreamRejected, got %v", err)
	}
}

func TestStreamAcceptStreamConvenience(t *testing.T) {
	opts := core.NewCoreOptions().WithStreamEnabled(true)
	_, addr, _ := startStreamCore(t, opts)

	bClient := newStreamClient(t, addr, "b")
	serverErr := make(chan error, 1)
	go func() {
		s, err := bClient.AcceptStream(context.Background())
		if err != nil {
			serverErr <- err
			return
		}
		defer s.Close()
		buf := make([]byte, 4)
		if _, err := io.ReadFull(s, buf); err != nil {
			serverErr <- err
			return
		}
		_, err = s.Write(buf)
		serverErr <- err
	}()

	aClient := newStreamClient(t, addr, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := aClient.OpenStream(ctx, "b", "x", nil, nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer s.Close()

	if _, err := s.Write([]byte("data")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(s, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "data" {
		t.Fatalf("read %q", buf)
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("accept stream: %v", err)
	}
}

func TestStreamSessionLossClosesStreams(t *testing.T) {
	opts := core.NewCoreOptions().WithStreamEnabled(true)
	b, addr, _ := startStreamCore(t, opts)

	accepted := make(chan *stream.Stream, 1)
	bClient := newStreamClient(t, addr, "b")
	bClient.OnStreamRequest(func(req *client.StreamRequest) {
		s, err := req.Accept(context.Background(), nil, nil)
		if err == nil {
			accepted <- s
		}
	})

	aClient := newStreamClient(t, addr, "a")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	aStream, err := aClient.OpenStream(ctx, "b", "x", nil, nil)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer aStream.Close()

	var bStream *stream.Stream
	select {
	case bStream = <-accepted:
	case <-time.After(5 * time.Second):
		t.Fatal("target did not accept stream")
	}
	defer bStream.Close()

	waitStreams(t, b, 1)
	_ = aClient.Close()

	_ = bStream.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := bStream.Read(make([]byte, 1)); err == nil {
		t.Fatal("expected stream read to fail after initiator session closed")
	}
	waitStreams(t, b, 0)
}

func TestStreamTLS(t *testing.T) {
	cert := generateTestCert(t)
	opts := core.NewCoreOptions().WithStreamEnabled(true)

	b, err := core.NewWithOptions(opts.WithLogger(zap.NewNop()))
	if err != nil {
		t.Fatalf("new core: %v", err)
	}
	if err := b.Start(); err != nil {
		t.Fatalf("start core: %v", err)
	}
	t.Cleanup(func() { _ = b.Stop() })

	main, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen main: %v", err)
	}
	streamTLS, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("listen stream: %v", err)
	}
	go func() { _ = b.Serve(main) }()
	go func() { _ = b.ServeStreamTLSListener(streamTLS) }()

	addr := main.Addr().String()
	tlsConfig := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}

	accepted := make(chan *stream.Stream, 1)
	bOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("b").
		WithAutoReconnect(false).
		WithStream(true).
		WithStreamTLSConfig(tlsConfig).
		WithLogger(zap.NewNop())
	bClient, err := client.NewWithOptions(bOpts)
	if err != nil {
		t.Fatalf("new client b: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := bClient.ConnectContext(ctx); err != nil {
		t.Fatalf("connect b: %v", err)
	}
	defer bClient.Close()
	bClient.OnStreamRequest(func(req *client.StreamRequest) {
		s, err := req.Accept(ctx, nil, nil)
		if err == nil {
			accepted <- s
		}
	})

	aOpts := client.NewClientOptions().
		WithCore(addr).
		WithClientID("a").
		WithAutoReconnect(false).
		WithStream(true).
		WithStreamTLSConfig(tlsConfig).
		WithLogger(zap.NewNop())
	aClient, err := client.NewWithOptions(aOpts)
	if err != nil {
		t.Fatalf("new client a: %v", err)
	}
	if err := aClient.ConnectContext(ctx); err != nil {
		t.Fatalf("connect a: %v", err)
	}
	defer aClient.Close()

	aStream, err := aClient.OpenStream(ctx, "b", "tls", nil, nil)
	if err != nil {
		t.Fatalf("open tls stream: %v", err)
	}
	defer aStream.Close()
	bStream := <-accepted
	defer bStream.Close()

	if _, err := aStream.Write([]byte("secure")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 6)
	if _, err := io.ReadFull(bStream, buf); err != nil || string(buf) != "secure" {
		t.Fatalf("read: %v %q", err, buf)
	}
}

func generateTestCert(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "beacon-stream-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("load key pair: %v", err)
	}
	return cert
}

package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestServerNameFromAddress(t *testing.T) {
	cases := []struct {
		address string
		want    string
	}{
		{"broker.example.com:8883", "broker.example.com"},
		{"localhost:8883", "localhost"},
		{"127.0.0.1:8883", ""},
		{"[::1]:8883", ""},
		{"broker.example.com", "broker.example.com"},
		{"", ""},
	}
	for _, c := range cases {
		if got := serverNameFromAddress(c.address); got != c.want {
			t.Errorf("serverNameFromAddress(%q) = %q, 期望 %q", c.address, got, c.want)
		}
	}
}

func TestNewTLSConfig(t *testing.T) {
	t.Run("nil-config-derives-servername", func(t *testing.T) {
		cfg := newTLSConfig(nil, "broker.example.com:8883")
		if cfg.ServerName != "broker.example.com" {
			t.Fatalf("ServerName = %q", cfg.ServerName)
		}
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Fatalf("MinVersion = %x, 期望 TLS1.2", cfg.MinVersion)
		}
		if cfg.InsecureSkipVerify {
			t.Fatal("不应关闭证书校验")
		}
	})

	t.Run("ip-address-keeps-servername-empty", func(t *testing.T) {
		cfg := newTLSConfig(nil, "127.0.0.1:8883")
		if cfg.ServerName != "" {
			t.Fatalf("IP 地址不应设置 ServerName，实际 %q", cfg.ServerName)
		}
	})

	t.Run("does-not-mutate-base-config", func(t *testing.T) {
		base := &tls.Config{}
		cfg := newTLSConfig(base, "broker.example.com:8883")
		if base.ServerName != "" {
			t.Fatalf("调用方配置被修改: %q", base.ServerName)
		}
		if cfg == base {
			t.Fatal("应返回克隆副本")
		}
		if cfg.ServerName != "broker.example.com" {
			t.Fatalf("ServerName = %q", cfg.ServerName)
		}
	})

	t.Run("keeps-explicit-servername", func(t *testing.T) {
		cfg := newTLSConfig(&tls.Config{ServerName: "explicit.example.com"}, "10.0.0.1:8883")
		if cfg.ServerName != "explicit.example.com" {
			t.Fatalf("ServerName = %q", cfg.ServerName)
		}
	})

	t.Run("insecure-skip-verify", func(t *testing.T) {
		cfg := newTLSConfig(&tls.Config{InsecureSkipVerify: true}, "broker.example.com:8883")
		if cfg.ServerName != "" {
			t.Fatalf("关闭校验时不应推导 ServerName，实际 %q", cfg.ServerName)
		}
	})
}

// TestTCPDialer_TLSHandshakeWithDerivedConfig 验证 tls:// 前缀 + 未传 TLSConfig 时
// 仍能完成证书校验（MinVersion 与 ServerName 均有安全默认值）。
func TestTCPDialer_TLSHandshakeWithDerivedConfig(t *testing.T) {
	cert, pool := newTestCertificate(t, "127.0.0.1")

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer ln.Close()

	accepted := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			accepted <- err
			return
		}
		defer conn.Close()
		accepted <- conn.(*tls.Conn).Handshake()
	}()

	dialer := &TCPDialer{
		Address:     "tls://" + ln.Addr().String(),
		TLSConfig:   &tls.Config{RootCAs: pool},
		DialTimeout: 5 * time.Second,
	}
	conn, err := dialer.Dial()
	if err != nil {
		t.Fatalf("TLS 拨号失败: %v", err)
	}
	conn.Close()

	if err := <-accepted; err != nil {
		t.Fatalf("服务端握手失败: %v", err)
	}
}

// newTestCertificate 生成自签证书（含 127.0.0.1 IP SAN）与对应的根证书池。
func newTestCertificate(t *testing.T, ip string) (tls.Certificate, *x509.CertPool) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("生成密钥失败: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: ip},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:           []net.IP{net.ParseIP(ip)},
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("签发证书失败: %v", err)
	}

	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("解析证书失败: %v", err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(leaf)

	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  key,
		Leaf:        leaf,
	}, pool
}

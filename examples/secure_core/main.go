// secure_core 演示认证 + TLS + 配额 + 运维端点。
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"

	"github.com/snple/beacon/core"
	"github.com/snple/beacon/packet"
	"go.uber.org/zap"
)

// staticAuth 是最小示例认证器：按 clientID 查密钥。
type staticAuth struct {
	secrets map[string]string
}

func (a *staticAuth) Authenticate(ctx *core.AuthContext) error {
	want, ok := a.secrets[ctx.ClientID]
	if !ok {
		return &core.AuthError{Reason: packet.ReasonNotAuthorized}
	}
	if string(ctx.AuthData) != want {
		return &core.AuthError{Reason: packet.ReasonBadAuthMethodOrAuthData}
	}
	ctx.Identity = &core.Identity{
		ID:            ctx.ClientID,
		ReadChannels:  []string{"sensors/**"},
		WriteChannels: []string{"sensors/**", "commands/**"},
	}
	return nil
}

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	certFile := os.Getenv("BEACON_CERT")
	keyFile := os.Getenv("BEACON_KEY")
	caFile := os.Getenv("BEACON_CA")

	b, err := core.NewWithOptions(core.NewCoreOptions().
		WithLogger(logger).
		WithQueueCap(2000).
		WithPushRateLimit(1000, 2000).
		WithHooks(&core.Hooks{
			Auth: &staticAuth{secrets: map[string]string{
				"device-1": "s3cret",
			}},
		}))
	if err != nil {
		log.Fatal(err)
	}
	if err := b.Start(); err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Stop() }()

	go func() {
		if err := b.ServeMetrics("127.0.0.1:19101"); err != nil {
			logger.Warn("metrics listener", zap.Error(err))
		}
	}()

	if certFile == "" || keyFile == "" {
		logger.Warn("BEACON_CERT/BEACON_KEY not set, starting plain TCP only")
		if err := b.ServeTCP("127.0.0.1:18883"); err != nil {
			logger.Warn("tcp listener", zap.Error(err))
		}
		return
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		log.Fatal(err)
	}
	tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}

	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			log.Fatal(err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			log.Fatal("failed to load CA")
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}

	fmt.Println("TLS listener on 127.0.0.1:18883")
	if err := b.ServeTLS("127.0.0.1:18883", tlsCfg); err != nil {
		logger.Warn("tls listener", zap.Error(err))
	}
}

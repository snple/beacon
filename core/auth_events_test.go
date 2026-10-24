package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/snple/beacon/client"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestSubscribeEvents_StopClosesStream 验证 Core.Stop 会关闭事件流通道。
// 订阅者可能使用 context.Background()，此时只有 Core 关闭通道才能让消费者退出。
func TestSubscribeEvents_StopClosesStream(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())

	stream, err := c.SubscribeEvents(context.Background(), EventOptions{Buffer: 4})
	require.NoError(t, err)

	require.NoError(t, c.Stop())

	select {
	case _, ok := <-stream.Events():
		require.False(t, ok, "event channel should be closed after Stop")
	case <-time.After(3 * time.Second):
		t.Fatal("event channel was not closed after Core.Stop")
	}

	// Stop 已经关闭过通道：重复 Close 必须安全（不得 panic 或重复 close）。
	stream.Close()
	stream.Close()
}

// TestHandleAuth_AppliesIdentityAndResponseProperties 验证 AUTH（重新认证）成功后，
// 认证器给出的 Identity 与 ResponseProperties 都会生效。
func TestHandleAuth_AppliesIdentityAndResponseProperties(t *testing.T) {
	authHandler := &AuthHandlerFunc{
		AuthFunc: func(ctx *AuthContext) (bool, []byte, error) {
			ctx.Identity = &Identity{Subject: "renewed", Roles: []string{"ops"}}
			ctx.ResponseProperties = map[string]string{"tenant": "acme"}
			return false, nil, nil
		},
	}

	c := testSetupCore(t, NewCoreOptions().WithAuthHandler(authHandler))
	defer c.Stop()

	addr := testServe(t, c)

	propsCh := make(chan map[string]string, 1)
	cliOpts := client.NewClientOptions().
		WithSessionTimeout(60).
		WithAuthHandler(client.AuthHandlerFunc(func(ctx *client.AuthContext) (bool, []byte, error) {
			propsCh <- ctx.Packet.Properties.UserProperties
			return false, nil, nil
		}))

	cli := testSetupClient(t, addr, "reauth-client", cliOpts)
	defer cli.Close()

	require.NoError(t, cli.SendAuth(packet.ReasonContinueAuth, []byte("token")))

	require.True(t, eventually(t, 3*time.Second, 20*time.Millisecond, "identity applied after re-auth", func() bool {
		info, err := c.GetClientInfo("reauth-client")
		return err == nil && info.Identity != nil && info.Identity.Subject == "renewed"
	}), "re-auth identity should be stored on the session")

	select {
	case props := <-propsCh:
		require.Equal(t, "acme", props["tenant"], "ResponseProperties should be returned to the client")
	case <-time.After(3 * time.Second):
		t.Fatal("client did not receive AUTH response properties")
	}
}

// TestHandleAuth_FailureClosesWithNotAuthorized 验证 AUTH 认证失败时，
// 连接关闭原因上报为 NotAuthorized 而不是 ProtocolError。
func TestHandleAuth_FailureClosesWithNotAuthorized(t *testing.T) {
	reasonCh := make(chan packet.ReasonCode, 1)

	authHandler := &AuthHandlerFunc{
		AuthFunc: func(*AuthContext) (bool, []byte, error) {
			return false, nil, errors.New("bad token")
		},
	}
	c := testSetupCore(t, NewCoreOptions().
		WithAuthHandler(authHandler).
		WithConnectionHandler(&ConnectionHandlerFunc{
			DisconnectFunc: func(ctx *DisconnectContext) {
				select {
				case reasonCh <- ctx.Reason:
				default:
				}
			},
		}))
	defer c.Stop()

	addr := testServe(t, c)
	cli := testSetupClient(t, addr, "reauth-fail", nil)
	defer cli.Close()

	require.NoError(t, cli.SendAuth(packet.ReasonContinueAuth, []byte("bad")))

	select {
	case reason := <-reasonCh:
		require.Equal(t, packet.ReasonNotAuthorized, reason)
	case <-time.After(3 * time.Second):
		t.Fatal("connection should be closed after failed re-auth")
	}
}

package core

import (
	"net"
	"testing"
	"time"

	"github.com/danclive/nson-go"
	"github.com/snple/beacon/packet"
	"github.com/stretchr/testify/require"
)

// TestSubscribe_InvalidFilterReturnsPerTopicReason 验证 SUBSCRIBE 中的非法过滤器
// 只会让该条订阅失败（SUBACK 返回 ReasonTopicFilterInvalid），不会断开连接。
func TestSubscribe_InvalidFilterReturnsPerTopicReason(t *testing.T) {
	c := testSetupCore(t, NewCoreOptions())
	defer c.Stop()

	conn, err := net.Dial("tcp", testServe(t, c))
	require.NoError(t, err)
	defer conn.Close()

	connect := packet.NewConnectPacket()
	connect.ClientID = "sub-invalid-filter"
	require.NoError(t, packet.WritePacket(conn, connect, 0))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	pkt, err := packet.ReadPacket(conn, 0)
	require.NoError(t, err)
	_, ok := pkt.(*packet.ConnackPacket)
	require.True(t, ok, "expected CONNACK, got %T", pkt)

	sub := packet.NewSubscribePacket(nson.NewId())
	sub.Subscriptions = []packet.Subscription{
		{Topic: "ok/topic", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
		{Topic: "bad/#", Options: packet.SubscribeOptions{QoS: packet.QoS0}},
	}
	require.NoError(t, packet.WritePacket(conn, sub, 0))

	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	pkt, err = packet.ReadPacket(conn, 0)
	require.NoError(t, err, "server must not close the connection on an invalid filter")

	suback, ok := pkt.(*packet.SubackPacket)
	require.True(t, ok, "expected SUBACK, got %T", pkt)
	require.Equal(t, sub.PacketID, suback.PacketID)
	require.Equal(t, []packet.ReasonCode{
		packet.ReasonGrantedQoS0,
		packet.ReasonTopicFilterInvalid,
	}, suback.ReasonCodes)

	// 连接仍然可用
	require.NoError(t, packet.WritePacket(conn, packet.NewPingPacket(7, uint64(time.Now().UnixNano())), 0))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	pkt, err = packet.ReadPacket(conn, 0)
	require.NoError(t, err)
	pong, ok := pkt.(*packet.PongPacket)
	require.True(t, ok, "expected PONG, got %T", pkt)
	require.Equal(t, uint32(7), pong.Seq)
}

package packet

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectStreamBits(t *testing.T) {
	// 能力/开关复用 CleanSession 与 SessionPresent 的 bit1，bit0 语义不变。
	conn := NewConnectPacket("client-a")
	conn.SetCleanSession(true)
	conn.SetStreamCapable(true)
	frame, err := EncodePacket(conn, 0)
	require.NoError(t, err)
	require.Equal(t, byte(3), frame[6], "clean byte should carry both bits")

	got, _, err := ReadPacketWithSize(bytes.NewReader(frame), 0)
	require.NoError(t, err)
	gotConn := got.(*ConnectPacket)
	require.True(t, gotConn.CleanSession())
	require.True(t, gotConn.StreamCapable())

	ack := NewConnackPacket(ReasonSuccess)
	ack.SetSessionPresent(true)
	ack.SetStreamEnabled(true)
	ack.SetStreamSupported(true)
	frame, err = EncodePacket(ack, 0)
	require.NoError(t, err)
	require.Equal(t, byte(7), frame[5], "session-present byte should carry all three bits")

	got, _, err = ReadPacketWithSize(bytes.NewReader(frame), 0)
	require.NoError(t, err)
	gotAck := got.(*ConnackPacket)
	require.True(t, gotAck.SessionPresent())
	require.True(t, gotAck.StreamEnabled())
	require.True(t, gotAck.StreamSupported())

	// 旧端（两个 bit 都为 0）线格式保持不变。
	plain, err := EncodePacket(NewConnackPacket(ReasonSuccess), 0)
	require.NoError(t, err)
	require.Equal(t, byte(0), plain[5])
}

func TestConnectFlagsUnknown(t *testing.T) {
	known := ConnectFlagCleanSession | ConnectFlagStream
	require.Zero(t, known.Unknown())

	f := known | ConnectFlags(0x80)
	require.Equal(t, ConnectFlags(0x80), f.Unknown())
}

func TestStreamControlPacketRoundTrip(t *testing.T) {
	cases := []Packet{
		&StreamOpenPacket{
			RequestID:  7,
			Target:     "b",
			Topic:      "chat",
			Metadata:   []byte("meta"),
			Properties: map[string]string{"k": "v"},
		},
		&StreamOpenOkPacket{
			RequestID:     7,
			ReasonCode:    ReasonSuccess,
			Message:       "accepted",
			StreamID:      9,
			StreamAddress: "127.0.0.1:9000",
			TLS:           true,
			Ticket:        []byte("ticket"),
			Challenge:     []byte("challenge"),
			Metadata:      []byte("reply"),
			Properties:    map[string]string{"a": "b"},
			BindTimeoutMs: 1000,
		},
		&StreamRequestPacket{
			StreamID:      9,
			Source:        "a",
			Topic:         "chat",
			Metadata:      []byte("meta"),
			Properties:    map[string]string{"k": "v"},
			Ticket:        []byte("ticket"),
			StreamAddress: "127.0.0.1:9000",
			TLS:           true,
			BindTimeoutMs: 1000,
		},
		&StreamAcceptPacket{
			StreamID:   9,
			Accepted:   true,
			ReasonCode: ReasonSuccess,
			Message:    "ok",
			Challenge:  []byte("challenge"),
			Metadata:   []byte("reply"),
			Properties: map[string]string{"a": "b"},
		},
		&StreamAbortPacket{
			StreamID:   9,
			ReasonCode: ReasonStreamTimeout,
			Message:    "timeout",
		},
	}

	for _, want := range cases {
		frame, err := EncodePacket(want, 0)
		require.NoError(t, err, want.Type().String())

		got, size, err := ReadPacketWithSize(bytes.NewReader(frame), 0)
		require.NoError(t, err, want.Type().String())
		require.Equal(t, len(frame), int(size))
		require.Equal(t, want, got, want.Type().String())
	}
}

func TestStreamPacketTooShort(t *testing.T) {
	// STREAM_OPEN 至少需要 18 字节包体；这里只声明 1 字节。
	frame := []byte{byte(STREAM_OPEN), 0, 0, 0, 1, 0}
	_, _, err := ReadPacketWithSize(bytes.NewReader(frame), 0)
	require.Error(t, err)
}

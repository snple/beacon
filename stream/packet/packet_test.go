package packet

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandshakePacketRoundTrip(t *testing.T) {
	cases := []Packet{
		&BindPacket{
			Version:   Version,
			StreamID:  42,
			Role:      RoleInitiator,
			Ticket:    []byte("ticket"),
			Challenge: []byte("challenge"),
		},
		&BindOkPacket{StreamID: 42, Role: RoleInitiator, ReasonCode: Success},
	}

	for _, want := range cases {
		var buf bytes.Buffer
		require.NoError(t, WritePacket(&buf, want), want.Type().String())

		got, err := ReadPacket(&buf)
		require.NoError(t, err, want.Type().String())
		require.Equal(t, want, got, want.Type().String())
	}
}

func TestHandshakeRejectsOversizedAndUnknown(t *testing.T) {
	// 声明的长度超过上限。
	frame := []byte{byte(BIND), 0, 1, 0, 1}
	_, err := ReadPacket(bytes.NewReader(frame))
	require.Error(t, err)

	// 未知类型。
	_, err = ReadPacket(bytes.NewReader([]byte{0xFF, 0, 0, 0, 0}))
	require.Error(t, err)
}

package packet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestReadPacket_RejectsTrailingBytes(t *testing.T) {
	pkt := &ConnectPacket{ClientID: "stream-trailing"}

	var buf bytes.Buffer
	if err := WritePacket(&buf, pkt); err != nil {
		t.Fatalf("failed to encode packet: %v", err)
	}

	data := append([]byte(nil), buf.Bytes()...)
	bodyLen := binary.BigEndian.Uint32(data[1:5])
	binary.BigEndian.PutUint32(data[1:5], bodyLen+1)
	data = append(data, 0x42)

	_, err := ReadPacket(bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected trailing bytes to be rejected")
	}

	var protoErr *StreamProtocolError
	if !errors.As(err, &protoErr) || protoErr.Code != MalformedPacket {
		t.Fatalf("expected malformed StreamProtocolError, got %v", err)
	}
}

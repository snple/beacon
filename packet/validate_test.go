package packet

import (
	"strings"
	"testing"
)

func TestValidateClientID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"simple", "device-1", true},
		{"auto generated", "65f1c0a1b2c3d4e5f6071829", true},
		{"utf8", "设备-1", true},
		{"empty", "", false},
		{"colon", "a:b", false},
		{"leading colon", ":a", false},
		{"trailing colon", "a:", false},
		{"nul", "a\x00b", false},
		{"newline", "a\nb", false},
		{"tab", "a\tb", false},
		{"del", "a\x7fb", false},
		{"invalid utf8", "a\xffb", false},
		{"too long", strings.Repeat("a", MaxClientIDLength+1), false},
		{"max length", strings.Repeat("a", MaxClientIDLength), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateClientID(tt.id); got != tt.want {
				t.Fatalf("ValidateClientID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestMaxClientIDLengthFitsProtocol(t *testing.T) {
	if MaxClientIDLength > MaxStringLength {
		t.Fatalf("MaxClientIDLength (%d) must not exceed protocol string limit (%d)",
			MaxClientIDLength, MaxStringLength)
	}
}

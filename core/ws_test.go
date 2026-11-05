package core

import (
	"net/http/httptest"
	"testing"
)

func TestDefaultWSCheckOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		want   bool
	}{
		{"no origin", "", true},
		{"same origin", "http://example.com:8080", true},
		{"cross origin", "https://evil.example", false},
		{"malformed", "://bad", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://example.com:8080/ws", nil)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			if got := defaultWSCheckOrigin(r); got != tc.want {
				t.Fatalf("defaultWSCheckOrigin(%q) = %v, want %v", tc.origin, got, tc.want)
			}
		})
	}
}

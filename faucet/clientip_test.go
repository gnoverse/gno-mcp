package faucet

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientIP(t *testing.T) {
	tests := []struct {
		name           string
		remoteAddr     string
		xff            string
		trustedProxies int
		want           string
	}{
		{"no trusted proxies uses remote addr", "203.0.113.5:443", "198.51.100.7", 0, "203.0.113.5"},
		{"one hop takes rightmost xff", "10.0.0.1:443", "198.51.100.7", 1, "198.51.100.7"},
		{"one hop ignores spoofed left entries", "10.0.0.1:443", "1.1.1.1, 2.2.2.2, 198.51.100.7", 1, "198.51.100.7"},
		{"two hops takes second from right", "10.0.0.1:443", "198.51.100.7, 10.0.0.9", 2, "198.51.100.7"},
		{"missing xff falls back to remote addr", "203.0.113.5:443", "", 1, "203.0.113.5"},
		{"fewer xff entries than hops falls back to remote addr", "203.0.113.5:443", "198.51.100.7", 2, "203.0.113.5"},
		{"remote addr without port", "203.0.113.5", "", 0, "203.0.113.5"},
		{"trims whitespace around entry", "10.0.0.1:443", "  198.51.100.7  ", 1, "198.51.100.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{RemoteAddr: tt.remoteAddr, Header: http.Header{}}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			assert.Equal(t, tt.want, clientIP(r, tt.trustedProxies))
		})
	}
}

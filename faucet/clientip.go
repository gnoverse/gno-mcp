package faucet

import (
	"net"
	"net/http"
	"strings"
)

// clientIP returns the originating client IP for r. trustedProxies is the number
// of reverse proxies between the client and this server that we trust to append
// honest X-Forwarded-For entries (e.g. 1 for a single ALB). The direct peer
// (r.RemoteAddr) counts as the first trusted proxy; each additional trusted hop
// consumes one X-Forwarded-For entry from the right (the side proxies append to),
// so a client cannot forge a left-hand entry to evade per-IP limiting. With no
// trusted proxies, or when there are too few X-Forwarded-For entries, it falls
// back to r.RemoteAddr.
func clientIP(r *http.Request, trustedProxies int) string {
	remote := hostOnly(r.RemoteAddr)
	if trustedProxies <= 0 {
		return remote
	}
	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return remote
	}
	parts := strings.Split(xff, ",")
	idx := len(parts) - trustedProxies
	if idx < 0 || idx >= len(parts) {
		return remote
	}
	return strings.TrimSpace(parts[idx])
}

// hostOnly strips the port from a host:port address, returning the input
// unchanged if it has no port.
func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

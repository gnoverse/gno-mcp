package faucet

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

// reqInfo accumulates request-scoped fields for the single access-log line. The
// middleware seeds method/ip; fund handlers enrich outcome/address/chain_id.
type reqInfo struct {
	method  string
	ip      string
	outcome string
	address string
	chainID string
}

type reqInfoKey struct{}

func reqInfoFromContext(ctx context.Context) *reqInfo {
	info, _ := ctx.Value(reqInfoKey{}).(*reqInfo)
	return info
}

// statusRecorder captures the response status for the access log, defaulting to
// 200 when the handler never calls WriteHeader. wroteHeader tracks whether the
// status line has been committed so the panic handler knows if it can still send
// a 500.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.status = code
	s.wroteHeader = true
	s.ResponseWriter.WriteHeader(code)
}

// Write commits an implicit 200 on first write, mirroring net/http, so a panic
// after the body has started is not misreported as a 500.
func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

// logRequests wraps next, emitting one structured JSON line per request after it
// completes. It extracts the client IP once and stores it (with the request
// method) in a context-carried reqInfo that fund handlers enrich with outcome,
// address, and chain-id. Only public, non-sensitive fields are logged.
func (f *Faucet) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		info := &reqInfo{method: r.Method, ip: clientIP(r, f.trustedProxies)}
		r = r.WithContext(context.WithValue(r.Context(), reqInfoKey{}, info))
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		// The access log is emitted from a defer so it runs on every exit path,
		// including a handler panic. A panic is recovered here (not allowed to
		// propagate and drop the connection): the client gets a 500 if nothing was
		// written yet, and the request is logged at ERROR with the panic value and
		// stack — so a panic is visible in the JSON stream, not just as a
		// plain-text stderr trace from net/http's per-connection recovery.
		defer func() {
			panicVal := recover()
			if panicVal != nil && !rec.wroteHeader {
				rec.WriteHeader(http.StatusInternalServerError)
			}

			route := r.Pattern
			if route == "" {
				route = r.URL.Path
			}
			attrs := []any{
				"method", info.method,
				"route", route,
				"status", rec.status,
				"latency_ms", time.Since(start).Milliseconds(),
				"client_ip", info.ip,
			}
			if info.outcome != "" {
				attrs = append(attrs, "outcome", info.outcome)
			}
			if info.address != "" {
				attrs = append(attrs, "address", info.address)
			}
			if info.chainID != "" {
				attrs = append(attrs, "chain_id", info.chainID)
			}
			if panicVal != nil {
				attrs = append(attrs, "panic", fmt.Sprint(panicVal), "stack", string(debug.Stack()))
				f.logger.Error("http_request", attrs...)
				return
			}
			f.logger.Info("http_request", attrs...)
		}()

		next.ServeHTTP(rec, r)
	})
}

package faucet

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogRequestsEmitsOneJSONLine(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	f := &Faucet{logger: logger, metrics: nopMetrics{}, trustedProxies: 1}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", func(w http.ResponseWriter, r *http.Request) {
		info := reqInfoFromContext(r.Context())
		require.NotNil(t, info)
		info.outcome = OutcomeSuccess
		info.address = "g1recipient"
		info.chainID = "test-13"
		// confirm the middleware extracted the client IP for handler reuse
		assert.Equal(t, "198.51.100.7", info.ip)
		w.WriteHeader(http.StatusOK)
	})
	h := f.logRequests(mux)

	req := httptest.NewRequest("POST", "/fund", nil)
	req.RemoteAddr = "10.0.0.1:443"
	req.Header.Set("X-Forwarded-For", "198.51.100.7")
	h.ServeHTTP(httptest.NewRecorder(), req)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	require.Len(t, lines, 1, "exactly one log line per request")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(lines[0], &entry))
	assert.Equal(t, "POST", entry["method"])
	assert.Equal(t, "POST /fund", entry["route"])
	assert.Equal(t, float64(200), entry["status"])
	assert.Equal(t, "198.51.100.7", entry["client_ip"])
	assert.Equal(t, "g1recipient", entry["address"])
	assert.Equal(t, "test-13", entry["chain_id"])
	assert.Equal(t, "success", entry["outcome"])
	assert.Contains(t, entry, "latency_ms")
}

func TestLogRequestsHealthHasNoFundFields(t *testing.T) {
	var buf bytes.Buffer
	f := &Faucet{logger: slog.New(slog.NewJSONHandler(&buf, nil)), metrics: nopMetrics{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })
	h := f.logRequests(mux)

	req := httptest.NewRequest("GET", "/health", nil)
	req.RemoteAddr = "10.0.0.1:443"
	h.ServeHTTP(httptest.NewRecorder(), req)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	assert.Equal(t, "GET /health", entry["route"])
	assert.Equal(t, float64(200), entry["status"])
	assert.NotContains(t, entry, "outcome")
	assert.NotContains(t, entry, "address")
}

func TestLogRequestsLogsUnmatchedRoutes(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		want               int
	}{
		{"unknown path 404", "GET", "/nope", http.StatusNotFound},
		{"wrong method 405", "GET", "/fund", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			f := &Faucet{logger: slog.New(slog.NewJSONHandler(&buf, nil)), metrics: nopMetrics{}}
			mux := http.NewServeMux()
			mux.HandleFunc("POST /fund", func(http.ResponseWriter, *http.Request) {})
			h := f.logRequests(mux)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.RemoteAddr = "1.2.3.4:5"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			require.Equal(t, tc.want, rec.Code)
			var entry map[string]any
			require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry),
				"unmatched route must still emit exactly one log line")
			assert.Equal(t, float64(tc.want), entry["status"])
		})
	}
}

func TestLogRequestsRecoversAndLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	f := &Faucet{logger: slog.New(slog.NewJSONHandler(&buf, nil)), metrics: nopMetrics{}}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /fund", func(http.ResponseWriter, *http.Request) {
		panic("boom in handler")
	})
	h := f.logRequests(mux)

	req := httptest.NewRequest("POST", "/fund", nil)
	req.RemoteAddr = "1.2.3.4:5"
	rec := httptest.NewRecorder()

	// A handler panic must NOT propagate out of the middleware (one bad request
	// must not crash the connection / take down the server).
	require.NotPanics(t, func() { h.ServeHTTP(rec, req) })

	// The client gets a 500, not a dropped connection.
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	// Exactly one structured ERROR line, carrying the panic value — so a panic is
	// visible in the JSON log stream, not only as a plain-text stderr stack.
	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	assert.Equal(t, "ERROR", entry["level"])
	assert.Equal(t, float64(500), entry["status"])
	assert.Contains(t, entry, "panic")
	assert.Contains(t, entry["panic"], "boom in handler")
	assert.Contains(t, entry, "stack")
}

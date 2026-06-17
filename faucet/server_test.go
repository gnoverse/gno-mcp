package faucet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_fund(t *testing.T) {
	fd := &fakeDispenser{}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))
	srv := httptest.NewServer(f.Handler())
	defer srv.Close()

	body200 := `{"address":"` + validAddr + `","chain_id":"test5"}`

	// happy path -> 200 + tx hash
	resp, err := http.Post(srv.URL+"/fund", "application/json", strings.NewReader(body200))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	var body fundResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	resp.Body.Close()
	assert.Equal(t, "0xhash", body.TxHash)

	// second request, same address -> 429 (cooldown)
	resp2, err := http.Post(srv.URL+"/fund", "application/json", strings.NewReader(body200))
	require.NoError(t, err)
	assert.Equal(t, 429, resp2.StatusCode)
	resp2.Body.Close()

	// non-test chain -> 403
	resp3, err := http.Post(srv.URL+"/fund", "application/json", strings.NewReader(`{"address":"g1z","chain_id":"mainnet"}`))
	require.NoError(t, err)
	assert.Equal(t, 403, resp3.StatusCode)
	resp3.Body.Close()

	// bad JSON -> 400
	resp4, err := http.Post(srv.URL+"/fund", "application/json", strings.NewReader(`{`))
	require.NoError(t, err)
	assert.Equal(t, 400, resp4.StatusCode)
	resp4.Body.Close()
}

func TestServer_health(t *testing.T) {
	f := New("test5", 1_000_000, &fakeDispenser{}, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))
	srv := httptest.NewServer(f.Handler())
	defer srv.Close()

	// Liveness probe for the load balancer: 200 while the process serves,
	// independent of chain reachability.
	resp, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_dispenseFailureIsGeneric(t *testing.T) {
	var logBuf bytes.Buffer
	fd := &fakeDispenser{err: errors.New("SECRET-INTERNAL-CHECKTX-LOG")}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}), WithLogger(slog.New(slog.NewJSONHandler(&logBuf, nil))))
	srv := httptest.NewServer(f.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/fund", "application/json",
		strings.NewReader(`{"address":"`+validAddr+`","chain_id":"test5"}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "SECRET-INTERNAL-CHECKTX-LOG",
		"internal dispense error must not leak to anonymous clients")
	// The internal detail is logged server-side for operators (it carries no
	// user secret — no mnemonic, no key), so ops can debug a failing dispenser.
	assert.Contains(t, logBuf.String(), "SECRET-INTERNAL-CHECKTX-LOG",
		"internal dispense error must be logged server-side")
}

type captureMetrics struct {
	mu       sync.Mutex
	outcomes []string
}

func (c *captureMetrics) RecordOutcome(_ context.Context, outcome string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.outcomes = append(c.outcomes, outcome)
}

// postFund POSTs body to srv's /fund and returns the response status code.
func postFund(t *testing.T, baseURL, body string) int {
	t.Helper()
	resp, err := http.Post(baseURL+"/fund", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestHandleFundRecordsOutcomeAndStatus(t *testing.T) {
	var buf bytes.Buffer
	cm := &captureMetrics{}
	lim := NewLimiter(LimiterCfg{PerAddrMax: 1, PerIPMax: 10, DailyCapUgnot: 1_000_000_000_000, GrantUgnot: 1_000_000})
	f := New("test5", 1_000_000, &fakeDispenser{}, lim,
		WithLogger(slog.New(slog.NewJSONHandler(&buf, nil))),
		WithMetrics(cm),
		WithTrustedProxies(0),
	)
	srv := httptest.NewServer(f.Handler())
	defer srv.Close()

	// success
	assert.Equal(t, http.StatusOK, postFund(t, srv.URL, `{"address":"`+validAddr+`","chain_id":"test5"}`))
	// chain mismatch -> 403 + chain_mismatch outcome (test99 is a valid testnet id, just not ours)
	assert.Equal(t, http.StatusForbidden, postFund(t, srv.URL, `{"address":"`+validAddr+`","chain_id":"test99"}`))
	// malformed body -> 400 + bad_request outcome, never reaches Fund
	assert.Equal(t, http.StatusBadRequest, postFund(t, srv.URL, `not json`))

	assert.Equal(t, []string{OutcomeSuccess, OutcomeChainMismatch, OutcomeBadRequest}, cm.outcomes)
}

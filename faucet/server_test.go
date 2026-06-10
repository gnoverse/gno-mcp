package faucet

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestServer_dispenseFailureIsGeneric(t *testing.T) {
	fd := &fakeDispenser{err: errors.New("SECRET-INTERNAL-CHECKTX-LOG")}
	f := New("test5", 1_000_000, fd, NewLimiter(LimiterCfg{
		PerAddrMax: 1, PerIPMax: 5, DailyCapUgnot: 1_000_000_000, GrantUgnot: 1_000_000,
	}))
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
}

package read

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func TestStatus_reportsConfigAndLiveTip(t *testing.T) {
	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{
		ChainID:   "test5",
		Height:    4242,
		BlockTime: time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
	})

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f), http.DefaultClient)
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "4242")
	assert.Contains(t, res.Text, `<untrusted_content kind="status"`, "node-sourced text must be wrapped")
	assert.NotContains(t, res.Text, "mismatch")
	require.NotNil(t, res.StructuredContent)
	assert.Equal(t, "testnet5", res.StructuredContent["profile"])
	assert.Equal(t, "test5", res.StructuredContent["chain_id"])
	assert.Equal(t, "http://127.0.0.1:26657", res.StructuredContent["rpc_url"])
	assert.Equal(t, int64(4242), res.StructuredContent["height"])
	assert.Equal(t, "2026-06-10T12:00:00Z", res.StructuredContent["block_time"])
}

func TestStatus_flagsChainIDMismatch(t *testing.T) {
	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{ChainID: "dev", Height: 7})

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f), http.DefaultClient)
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "mismatch", "node reporting a different chain-id than the profile declares must be flagged")
	assert.Equal(t, "dev", res.StructuredContent["node_chain_id"])
}

func TestStatus_nodeUnreachableStillReportsConfig(t *testing.T) {
	f := chain.NewFake()
	f.SetStatusError(assert.AnError)

	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(f), http.DefaultClient)
	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "a dead node is a finding, not a tool failure — config info must still flow")
	assert.Contains(t, res.Text, "http://127.0.0.1:26657")
	assert.Equal(t, "test5", res.StructuredContent["chain_id"])
	assert.NotEmpty(t, res.StructuredContent["height_error"])
	assert.NotContains(t, res.StructuredContent, "height")
}

func TestStatus_unknownProfile(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterStatus(s, constResolver(chain.NewFake()), http.DefaultClient)
	_, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{
		"profile": "nope",
	})
	require.Error(t, err)
}

// newFaucetStatusServer builds a server whose "testnet5" profile points its
// faucet-service-url at faucetURL.
func newFaucetStatusServer(t *testing.T, faucetURL string) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "http://127.0.0.1:26657", ChainID: "test5", FaucetServiceURL: faucetURL},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	return server.NewServer(cfg, "")
}

func TestStatus_surfacesFaucetLimits(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /limits", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"grant_ugnot": 10_000_000,
			"per_address": map[string]any{"max": 1, "window_seconds": 86400},
		})
	})
	fsrv := httptest.NewServer(mux)
	defer fsrv.Close()

	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{ChainID: "test5", Height: 9})
	s := newFaucetStatusServer(t, fsrv.URL)
	RegisterStatus(s, constResolver(f), fsrv.Client())

	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{"profile": "testnet5"})
	require.NoError(t, err)
	require.NotNil(t, res.StructuredContent["faucet"])
	assert.Contains(t, res.Text, "per address")
	assert.Contains(t, res.Text, "GNOT")
}

func TestStatus_faucetUnreachableIsNonFatal(t *testing.T) {
	// A profile pointing at a dead faucet URL: status still succeeds.
	f := chain.NewFake()
	f.SetStatus(chain.NodeStatus{ChainID: "test5", Height: 9})
	s := newFaucetStatusServer(t, "http://127.0.0.1:1") // nothing listening
	RegisterStatus(s, constResolver(f), &http.Client{})

	res, err := s.Registry().Call(context.Background(), "gno_status", map[string]any{"profile": "testnet5"})
	require.NoError(t, err, "a dead faucet is a finding, not a tool failure")
	assert.NotEmpty(t, res.StructuredContent["faucet_error"])
	assert.NotContains(t, res.StructuredContent, "faucet")
}

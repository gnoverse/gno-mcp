package read

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// The catalog is how an agent maps a user's chain name ("deploy on test13",
// "use topaz") to a profile: every entry must show name AND chain-id together,
// plus endpoints and lifecycle status.
func TestProfileList_catalog(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet": {
			RPCURL:           "https://rpc.current.example:443",
			ChainID:          "topaz-1",
			GnowebURL:        "https://gnoweb.current.example",
			TxIndexerURL:     "https://idx.current.example/graphql/query",
			FaucetServiceURL: "https://faucet.current.example",
		},
		"test13": {RPCURL: "https://rpc.old.example:443", ChainID: "test-13", Sunset: true},
		"local":  {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"beta":   {RPCURL: "https://rpc.main.example:443", ChainID: "gnoland1"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	s := server.NewServer(cfg, "")
	RegisterProfileList(s)

	res, err := s.Registry().Call(context.Background(), "gno_profile_list", map[string]any{})
	require.NoError(t, err)

	// Name and chain-id visible together, per profile.
	assert.Contains(t, res.Text, "testnet")
	assert.Contains(t, res.Text, "topaz-1")
	assert.Contains(t, res.Text, "test-13")
	assert.Contains(t, res.Text, "dev")
	assert.Contains(t, res.Text, "gnoland1")

	// Lifecycle status: the sunset chain is labeled, the current one is not.
	assert.Contains(t, res.Text, "sunset")
	assert.Contains(t, res.Text, "current")

	// Endpoints of the current testnet.
	assert.Contains(t, res.Text, "https://rpc.current.example:443")
	assert.Contains(t, res.Text, "https://gnoweb.current.example")
	assert.Contains(t, res.Text, "https://idx.current.example/graphql/query")
	assert.Contains(t, res.Text, "https://faucet.current.example")

	// Structured mirror for programmatic use.
	require.NotNil(t, res.StructuredContent)
	list, ok := res.StructuredContent["profiles"].([]map[string]any)
	require.True(t, ok, "profiles list missing from structured content")
	require.Len(t, list, 4)
	byName := map[string]map[string]any{}
	for _, p := range list {
		byName[p["name"].(string)] = p
	}
	assert.Equal(t, "topaz-1", byName["testnet"]["chain_id"])
	assert.Equal(t, "testnet", byName["testnet"]["kind"])
	assert.Equal(t, false, byName["testnet"]["sunset"])
	assert.Equal(t, true, byName["test13"]["sunset"])
	assert.Equal(t, "testnet", byName["test13"]["kind"], "sunset is advisory — the profile stays a writable testnet")
	assert.Equal(t, "local", byName["local"]["kind"])
}

// No profile argument: the tool lists everything unconditionally.
func TestProfileList_noArgs(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterProfileList(s)
	res, err := s.Registry().Call(context.Background(), "gno_profile_list", map[string]any{})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "testnet5")
	assert.Contains(t, res.Text, "test5")
}

package read

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// The read profile arg is free-form, so its description must carry the live
// name->chain-id map (with lifecycle labels) for the agent to resolve a chain
// named by the user — including sunset and read-only chains, which reads may
// still target.
func Test_addProfileArg_descriptionMapsNamesToChainIDs(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet": {RPCURL: "https://rpc.current.example:443", ChainID: "topaz-1"},
		"test13":  {RPCURL: "https://rpc.old.example:443", ChainID: "test-13", Sunset: true},
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"beta":    {RPCURL: "https://rpc.main.example:443", ChainID: "gnoland1"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	s := server.NewServer(cfg, "")

	props := map[string]any{}
	var required []string
	addProfileArg(s, props, &required)

	arg := props["profile"].(map[string]any)
	desc := arg["description"].(string)
	assert.Contains(t, desc, "testnet (chain topaz-1)")
	assert.Contains(t, desc, "test13 (chain test-13, sunset)")
	assert.Contains(t, desc, "local (chain dev)")
	assert.Contains(t, desc, "beta (chain gnoland1, read-only)")
}

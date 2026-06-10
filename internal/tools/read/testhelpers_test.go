package read

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// newBaseTestServer builds a Server with one "testnet5" profile loaded
// and no tools registered. Callers register only the tool under test.
func newBaseTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "http://127.0.0.1:26657", ChainID: "test5"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	return server.NewServer(cfg, "")
}

// constResolver returns a chain.Resolver that yields c regardless of
// the profile argument.
func constResolver(c chain.Client) chain.Resolver {
	return func(_ string) chain.Client { return c }
}

// onlyProfileResolver returns a chain.Resolver that yields c for the
// given profile name and nil for anything else. Useful for exercising
// the no-client-for-profile error path.
func onlyProfileResolver(name string, c chain.Client) chain.Resolver {
	return func(profile string) chain.Client {
		if profile == name {
			return c
		}
		return nil
	}
}

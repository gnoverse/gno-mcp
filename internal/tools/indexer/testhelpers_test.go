package indexer

import (
	"testing"

	"github.com/stretchr/testify/require"

	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

// newBaseTestServer builds a Server with one "testnet5" profile that
// has a tx-indexer-url set, and no tools registered. Callers register
// only the tool under test.
func newBaseTestServer(t *testing.T) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {
			ChainType:    "testnet",
			RPCURL:       "x",
			ChainID:      "test5",
			TxIndexerURL: "https://indexer.test5/graphql",
		},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err)
	return server.NewServer(cfg, "")
}

func constResolver(c indexerpkg.Client) indexerpkg.Resolver {
	return func(_ string) indexerpkg.Client { return c }
}

func onlyProfileResolver(name string, c indexerpkg.Client) indexerpkg.Resolver {
	return func(profile string) indexerpkg.Client {
		if profile == name {
			return c
		}
		return nil
	}
}

//go:build integration

package integration_test

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	admintools "github.com/gnoverse/gno-mcp/internal/tools/admin"
)

// newDevNodeAddr boots a bare in-process node whose genesis declares chain-id
// "dev" (the default in-memory id "tendermint_test" fails the profile
// allowlist) and returns its RPC address rewritten to http:// (the node
// reports a tcp:// listen address; the dynamic-add path requires http(s)).
func newDevNodeAddr(t *testing.T) string {
	t.Helper()
	cfg := integration.TestingMinimalNodeConfig(gnoenv.RootDir())
	cfg.Genesis.ChainID = "dev"

	node, remoteAddr := integration.TestingInMemoryNode(t, slog.Default(), cfg)
	t.Cleanup(func() { _ = node.Stop() })

	return strings.Replace(remoteAddr, "tcp://", "http://", 1)
}

func TestIntegration_QueryChainID(t *testing.T) {
	addr := newDevNodeAddr(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := chain.QueryChainID(ctx, addr)
	require.NoError(t, err)
	assert.Equal(t, "dev", got, "node must report the genesis chain-id")
}

func TestIntegration_ProfileAdd_live(t *testing.T) {
	addr := newDevNodeAddr(t)
	ctx := context.Background()

	s := server.NewServer(&profiles.Config{Profiles: profiles.BuiltinProfiles()}, "")
	added := 0
	admintools.RegisterProfileAdd(s, http.DefaultClient, chain.QueryChainID, func() error { added++; return nil })

	// Happy path: declared chain-id matches what the live node reports.
	_, err := s.Registry().Call(ctx, "gno_profile_add", map[string]any{
		"name": "devnode", "rpc_url": addr, "chain_id": "dev",
	})
	require.NoError(t, err)
	p, ok := s.Config().Profiles["devnode"]
	require.True(t, ok, "profile must be added after live verification")
	assert.Equal(t, addr, p.RPCURL)
	assert.Equal(t, 1, added)

	// Mismatch: the node reports "dev", not the declared "test5".
	_, err = s.Registry().Call(ctx, "gno_profile_add", map[string]any{
		"name": "liar", "rpc_url": addr, "chain_id": "test5",
	})
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "chain_id_mismatch", te.Code)
	assert.Equal(t, "dev", te.Extra["reported"])
	_, ok = s.Config().Profiles["liar"]
	assert.False(t, ok, "mismatched profile must not be added")

	// Unreachable: nothing listens on port 1.
	_, err = s.Registry().Call(ctx, "gno_profile_add", map[string]any{
		"name": "deadport", "rpc_url": "http://127.0.0.1:1", "chain_id": "test5",
	})
	require.Error(t, err)
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "chain_unreachable", te.Code)
}

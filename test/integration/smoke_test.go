//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	writetools "github.com/gnoverse/gno-mcp/internal/tools/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmoke_renderGnolandHome(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	require.NoError(t, err, "NewReal")
	out, err := c.Render(context.Background(), "gno.land/r/gnoland/home", "")
	require.NoError(t, err, "Render")
	require.NotEmpty(t, out, "expected non-empty render output")
	assert.True(t, strings.Contains(strings.ToLower(out), "gno"),
		"expected 'gno' somewhere in homepage output, got first 200 chars: %s", firstN(out, 200))
}

func TestSmoke_inspectGrc20(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	require.NoError(t, err, "NewReal")
	doc, err := c.Doc(context.Background(), "gno.land/p/demo/tokens/grc20")
	require.NoError(t, err, "Doc")
	assert.Contains(t, doc, "Transfer",
		"expected grc20 to mention Transfer, got: %s", firstN(doc, 500))
}

func TestSmoke_sessionPropose_returnsValidCommand(t *testing.T) {
	cfg := &profiles.Config{
		Profiles: map[string]profiles.Profile{
			"test11": {
				ChainType:     "testnet",
				RPCURL:        "https://rpc.test11.testnets.gno.land:443",
				ChainID:       "test11",
				MasterAddress: "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3",
			},
		},
	}
	_, err := cfg.Validate()
	require.NoError(t, err, "validate")

	s := server.NewServer(cfg, "")
	sessionMgr := session.NewManager(t.TempDir(), "")
	writetools.RegisterSessionPropose(s, sessionMgr)

	tool, ok := s.Registry().Get("gno_session_propose")
	require.True(t, ok, "gno_session_propose not registered")

	res, err := tool.Handler(context.Background(), map[string]any{
		"profile":     "test11",
		"allow_paths": []any{"gno.land/r/test/example"},
	})
	require.NoError(t, err, "Handler")
	assert.Contains(t, res.Text, "gnokey maketx session create")
	assert.Contains(t, res.Text, "gpub1")
	assert.Contains(t, res.Text, "<your-master-key-name>")
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

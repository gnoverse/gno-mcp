//go:build integration
// +build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	writetools "github.com/gnoverse/gno-mcp/internal/tools/write"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const counterRealm = "gno.land/r/test/counter"

// TestSmoke_renderCounter exercises Render against the in-process node seeded
// with the counter testdata realm. No network access required.
func TestSmoke_renderCounter(t *testing.T) {
	c := newNodeBackedReal(t)
	out, err := c.Render(context.Background(), counterRealm, "")
	require.NoError(t, err, "Render")
	require.NotEmpty(t, out, "expected non-empty render output")
	assert.Contains(t, out, "Counter",
		"expected 'Counter' in render output, got first 200 chars: %s", firstN(out, 200))
}

// TestSmoke_docCounter exercises Doc against the in-process node seeded with
// the counter testdata realm. No network access required.
func TestSmoke_docCounter(t *testing.T) {
	c := newNodeBackedReal(t)
	doc, err := c.Doc(context.Background(), counterRealm)
	require.NoError(t, err, "Doc")
	assert.Contains(t, doc, "Increment",
		"expected counter doc to mention Increment, got: %s", firstN(doc, 500))
}

// TestSmoke_sessionPropose_returnsValidCommand is already offline: it exercises
// the session-propose tool's command-generation logic without hitting a network.
func TestSmoke_sessionPropose_returnsValidCommand(t *testing.T) {
	cfg := &profiles.Config{
		Profiles: map[string]profiles.Profile{
			"test-13": {
				RPCURL:        "https://rpc.test13.testnets.gno.land:443",
				ChainID:       "test-13",
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
		"profile":     "test-13",
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

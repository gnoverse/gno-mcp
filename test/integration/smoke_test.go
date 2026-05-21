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
)

func TestSmoke_renderGnolandHome(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	out, err := c.Render(context.Background(), "gno.land/r/gnoland/home", "")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty render output")
	}
	if !strings.Contains(strings.ToLower(out), "gno") {
		t.Errorf("expected 'gno' somewhere in homepage output, got first 200 chars: %s", firstN(out, 200))
	}
}

func TestSmoke_inspectGrc20(t *testing.T) {
	c, err := chain.NewReal("https://rpc.test11.testnets.gno.land:443", "test11")
	if err != nil {
		t.Fatalf("NewReal: %v", err)
	}
	doc, err := c.Doc(context.Background(), "gno.land/p/demo/tokens/grc20")
	if err != nil {
		t.Fatalf("Doc: %v", err)
	}
	if !strings.Contains(doc, "Transfer") {
		t.Errorf("expected grc20 to mention Transfer, got: %s", firstN(doc, 500))
	}
}

func TestSmoke_sessionPropose_returnsValidCommand(t *testing.T) {
	cfg := &profiles.Config{
		Profiles: map[string]profiles.Profile{
			"test11": {
				ChainType:           "testnet",
				RPCURL:              "https://rpc.test11.testnets.gno.land:443",
				ChainID:             "test11",
				AllowDangerousTools: true,
			},
		},
	}
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}

	s := server.NewServer(cfg, "")
	sessionMgr := session.NewManager(t.TempDir(), "")
	writetools.RegisterSessionPropose(s, sessionMgr)

	tool, ok := s.Registry().Get("gno_session_propose")
	if !ok {
		t.Fatal("gno_session_propose not registered")
	}

	res, err := tool.Handler(context.Background(), map[string]any{
		"profile":     "test11",
		"allow_paths": []any{"gno.land/r/test/example"},
	})
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if !strings.Contains(res.Text, "gnokey maketx session create") {
		t.Errorf("expected gnokey command in text, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "gpub1") {
		t.Errorf("expected --pubkey gpub1... in text, got: %s", res.Text)
	}
	if !strings.Contains(res.Text, "<your-master-key-name>") {
		t.Errorf("expected master-key placeholder in text, got: %s", res.Text)
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

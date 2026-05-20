package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
)

func newTestServer(t *testing.T, c chain.Client) *server.Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s := server.NewServer(cfg, "")
	RegisterRender(s, func(profile string) chain.Client { return c })
	return s
}

func TestRender_returnsResource(t *testing.T) {
	f := chain.NewFake()
	f.SetRender("gno.land/r/foo", "", "# Hello\nBody.")

	s := newTestServer(t, f)
	res, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.ResourceBody != "# Hello\nBody." {
		t.Errorf("ResourceBody = %q", res.ResourceBody)
	}
	if res.ResourceURI == "" {
		t.Error("expected ResourceURI to be set")
	}
}

func TestRender_passesPath(t *testing.T) {
	f := chain.NewFake()
	f.SetRender("gno.land/r/foo", "subpath/x", "subbody")

	s := newTestServer(t, f)
	res, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   "gno.land/r/foo",
		"path":    "subpath/x",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.ResourceBody != "subbody" {
		t.Errorf("ResourceBody = %q, want 'subbody'", res.ResourceBody)
	}
}

func TestRender_requiresRealm(t *testing.T) {
	s := newTestServer(t, chain.NewFake())
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

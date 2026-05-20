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
	if res.ResourceURI != "gno://gno.land/r/foo" {
		t.Errorf("ResourceURI = %q, want gno://gno.land/r/foo", res.ResourceURI)
	}
	if res.ResourceMIME != "text/markdown" {
		t.Errorf("ResourceMIME = %q, want text/markdown", res.ResourceMIME)
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
	if res.ResourceURI != "gno://gno.land/r/foo/subpath/x" {
		t.Errorf("ResourceURI = %q, want gno://gno.land/r/foo/subpath/x", res.ResourceURI)
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

func TestRender_rejectsNonStringRealm(t *testing.T) {
	s := newTestServer(t, chain.NewFake())
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   42,
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected type error when realm is not a string")
	}
}

func TestRender_unknownProfileReturnsError(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {ChainType: "testnet", RPCURL: "x", ChainID: "test5"},
	}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	s := server.NewServer(cfg, "")
	// Resolver returns nil for unknown profile names.
	RegisterRender(s, func(profile string) chain.Client {
		if profile == "testnet5" {
			return chain.NewFake()
		}
		return nil
	})
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for unknown profile")
	}
}

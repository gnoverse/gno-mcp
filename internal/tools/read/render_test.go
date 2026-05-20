package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestRender_returnsResource(t *testing.T) {
	f := chain.NewFake()
	f.SetRender("gno.land/r/foo", "", "# Hello\nBody.")

	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(f))
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

	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(f))
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
	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestRender_rejectsNonStringRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   42,
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected type error when realm is not a string")
	}
}

func TestRender_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRender(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for unknown profile")
	}
}

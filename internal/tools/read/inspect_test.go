package read

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestInspect_returnsText(t *testing.T) {
	f := chain.NewFake()
	f.SetDoc("gno.land/r/foo", "package foo\n\nfunc Bar() string")

	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.Text != "package foo\n\nfunc Bar() string" {
		t.Errorf("Text = %q", res.Text)
	}
}

func TestInspect_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestInspect_rejectsNonStringRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   42,
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected type error when realm is not a string")
	}
}

func TestInspect_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for unknown profile")
	}
}

func TestInspect_propagatesDocError(t *testing.T) {
	// No SetDoc call — Fake.Doc will return an error for any realm.
	f := chain.NewFake()

	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(f))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when Doc returns error")
	}
	if !strings.Contains(err.Error(), "gno_inspect:") {
		t.Errorf("error should be wrapped with gno_inspect: prefix, got %q", err.Error())
	}
}

package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func TestRead_returnsFile(t *testing.T) {
	f := chain.NewFake()
	f.SetFile("gno.land/r/x", "x.gno", "package x\n\nfunc Foo() {}")

	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"file":    "x.gno",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.ResourceBody != "package x\n\nfunc Foo() {}" {
		t.Errorf("ResourceBody = %q", res.ResourceBody)
	}
	if res.ResourceURI != "gno://gno.land/r/x/x.gno" {
		t.Errorf("ResourceURI = %q, want gno://gno.land/r/x/x.gno", res.ResourceURI)
	}
	if res.ResourceMIME != "text/x-gno" {
		t.Errorf("ResourceMIME = %q, want text/x-gno", res.ResourceMIME)
	}
}

func TestRead_returnsListing(t *testing.T) {
	f := chain.NewFake()
	f.SetListing("gno.land/r/x", []string{"x.gno", "helper.gno"})

	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.ResourceBody != "x.gno\nhelper.gno\n" {
		t.Errorf("ResourceBody = %q, want %q", res.ResourceBody, "x.gno\nhelper.gno\n")
	}
	if res.ResourceURI != "gno://gno.land/r/x" {
		t.Errorf("ResourceURI = %q, want gno://gno.land/r/x", res.ResourceURI)
	}
	if res.ResourceMIME != "text/plain" {
		t.Errorf("ResourceMIME = %q, want text/plain", res.ResourceMIME)
	}
}

func TestRead_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestRead_rejectsNonStringFile(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"file":    42,
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected type error when file is not a string")
	}
}

func TestRead_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for unknown profile")
	}
}

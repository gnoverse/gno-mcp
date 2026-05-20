package indexer

import (
	"context"
	"strings"
	"testing"

	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
)

func TestList_returnsRealms(t *testing.T) {
	f := indexerpkg.NewFake()
	f.SetList(indexerpkg.ListFilter{}, []indexerpkg.Realm{
		{
			Path:        "gno.land/r/demo/boards",
			Description: "A simple message board.",
			Tags:        []string{"social", "messaging"},
			Category:    "social",
		},
	})

	s := newBaseTestServer(t)
	RegisterList(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_list", map[string]any{
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "gno.land/r/demo/boards") {
		t.Errorf("Text does not contain realm path: %q", res.Text)
	}
	if !strings.Contains(res.Text, "social") {
		t.Errorf("Text does not contain tag info: %q", res.Text)
	}
}

func TestList_noMatches(t *testing.T) {
	f := indexerpkg.NewFake()
	f.SetList(indexerpkg.ListFilter{}, []indexerpkg.Realm{})

	s := newBaseTestServer(t)
	RegisterList(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_list", map[string]any{
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "No realms matched the filter.") {
		t.Errorf("Text = %q, want 'No realms matched the filter.'", res.Text)
	}
}

func TestList_profileWithoutIndexer(t *testing.T) {
	f := indexerpkg.NewFake()
	s := newBaseTestServer(t)
	RegisterList(s, onlyProfileResolver("testnet5", f))
	_, err := s.Registry().Call(context.Background(), "gno_list", map[string]any{
		"profile": "ghost",
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for profile without indexer")
	}
}

func TestList_rejectsNonStringTag(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterList(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_list", map[string]any{
		"profile": "testnet5",
		"tag":     42,
	})
	if err == nil {
		t.Fatal("expected type error when tag is not a string")
	}
}

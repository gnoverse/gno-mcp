package indexer

import (
	"context"
	"strings"
	"testing"
	"time"

	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
)

func TestActivity_filtersByTimeRange(t *testing.T) {
	f := indexerpkg.NewFake()

	old := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	new_ := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	f.SetActivity(testRealm, []indexerpkg.TxEvent{
		{Hash: "0xold", Height: 10, Time: old, Kind: "MsgCall", Caller: "g1old"},
		{Hash: "0xmid", Height: 20, Time: mid, Kind: "MsgCall", Caller: "g1mid"},
		{Hash: "0xnew", Height: 30, Time: new_, Kind: "MsgRun", Caller: "g1new"},
	})

	since := "2024-03-01T00:00:00Z"
	until := "2024-09-01T00:00:00Z"

	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
		"since":   since,
		"until":   until,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !strings.Contains(res.Text, "0xmid") {
		t.Errorf("expected mid event in result, got: %q", res.Text)
	}
	if strings.Contains(res.Text, "0xold") {
		t.Errorf("old event should be filtered out, got: %q", res.Text)
	}
	if strings.Contains(res.Text, "0xnew") {
		t.Errorf("new event should be filtered out, got: %q", res.Text)
	}
}

func TestActivity_invalidSince(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
		"since":   "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid since")
	}
	if !strings.Contains(err.Error(), "invalid 'since'") {
		t.Errorf("error = %q, want 'invalid 'since''", err.Error())
	}
}

func TestActivity_invalidUntil(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
		"until":   "not-a-date",
	})
	if err == nil {
		t.Fatal("expected error for invalid until")
	}
	if !strings.Contains(err.Error(), "invalid 'until'") {
		t.Errorf("error = %q, want 'invalid 'until''", err.Error())
	}
}

func TestActivity_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestActivity_profileWithoutIndexer(t *testing.T) {
	f := indexerpkg.NewFake()
	s := newBaseTestServer(t)
	RegisterActivity(s, onlyProfileResolver("testnet5", f))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "ghost",
		"realm":   testRealm,
	})
	if err == nil {
		t.Fatal("expected error when resolver returns nil for profile without indexer")
	}
	if !strings.Contains(err.Error(), "no tx-indexer-url") {
		t.Errorf("error = %q, want 'no tx-indexer-url'", err.Error())
	}
}

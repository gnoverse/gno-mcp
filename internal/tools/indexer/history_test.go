package indexer

import (
	"context"
	"strings"
	"testing"
	"time"

	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
)

const testRealm = "gno.land/r/demo/boards"

func TestHistory_returnsEvents(t *testing.T) {
	f := indexerpkg.NewFake()
	f.SetHistory(testRealm, []indexerpkg.TxEvent{
		{
			Hash:   "0xabc",
			Height: 100,
			Time:   time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			Kind:   "MsgAddPackage",
			Caller: "g1deployer",
		},
		{
			Hash:   "0xdef",
			Height: 200,
			Time:   time.Date(2024, 2, 20, 12, 30, 0, 0, time.UTC),
			Kind:   "MsgCall",
			Caller: "g1caller",
			Func:   "CreatePost",
		},
	})

	s := newBaseTestServer(t)
	RegisterHistory(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "0xabc") {
		t.Errorf("Text does not contain hash 0xabc: %q", res.Text)
	}
	if !strings.Contains(res.Text, "0xdef") {
		t.Errorf("Text does not contain hash 0xdef: %q", res.Text)
	}
	if !strings.Contains(res.Text, "kind=MsgAddPackage") {
		t.Errorf("Text does not contain kind=MsgAddPackage: %q", res.Text)
	}
	if !strings.Contains(res.Text, "kind=MsgCall") {
		t.Errorf("Text does not contain kind=MsgCall: %q", res.Text)
	}
}

func TestHistory_emptyHistoryReturnsMessage(t *testing.T) {
	f := indexerpkg.NewFake()
	f.SetHistory(testRealm, []indexerpkg.TxEvent{})

	s := newBaseTestServer(t)
	RegisterHistory(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.Text != "No transactions found for this realm." {
		t.Errorf("Text = %q, want exact no-tx message", res.Text)
	}
}

func TestHistory_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterHistory(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when realm is missing")
	}
}

func TestHistory_profileWithoutIndexer(t *testing.T) {
	f := indexerpkg.NewFake()
	s := newBaseTestServer(t)
	RegisterHistory(s, onlyProfileResolver("testnet5", f))
	_, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
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

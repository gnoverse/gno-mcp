package indexer

import (
	"context"
	"strings"
	"testing"
	"time"

	indexerpkg "github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "Call")
	assert.True(t, strings.Contains(res.Text, "0xmid"), "expected mid event in result, got: %q", res.Text)
	assert.False(t, strings.Contains(res.Text, "0xold"), "old event should be filtered out, got: %q", res.Text)
	assert.False(t, strings.Contains(res.Text, "0xnew"), "new event should be filtered out, got: %q", res.Text)
}

func TestActivity_invalidSince(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
		"since":   "not-a-date",
	})
	require.Error(t, err, "expected error for invalid since")
	assert.True(t, strings.Contains(err.Error(), "invalid 'since'"), "error = %q, want 'invalid 'since''", err.Error())
}

func TestActivity_invalidUntil(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
		"realm":   testRealm,
		"until":   "not-a-date",
	})
	require.Error(t, err, "expected error for invalid until")
	assert.True(t, strings.Contains(err.Error(), "invalid 'until'"), "error = %q, want 'invalid 'until''", err.Error())
}

func TestActivity_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterActivity(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestActivity_profileWithoutIndexer(t *testing.T) {
	f := indexerpkg.NewFake()
	s := newBaseTestServer(t)
	RegisterActivity(s, onlyProfileResolver("testnet5", f))
	_, err := s.Registry().Call(context.Background(), "gno_activity", map[string]any{
		"profile": "ghost",
		"realm":   testRealm,
	})
	require.Error(t, err, "expected error when resolver returns nil for profile without indexer")
	assert.True(t, strings.Contains(err.Error(), "no tx-indexer-url"), "error = %q, want 'no tx-indexer-url'", err.Error())
}

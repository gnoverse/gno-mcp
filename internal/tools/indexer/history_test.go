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
	require.NoError(t, err, "Call")
	assert.True(t, strings.Contains(res.Text, "0xabc"), "Text does not contain hash 0xabc: %q", res.Text)
	assert.True(t, strings.Contains(res.Text, "0xdef"), "Text does not contain hash 0xdef: %q", res.Text)
	assert.True(t, strings.Contains(res.Text, "kind=MsgAddPackage"), "Text does not contain kind=MsgAddPackage: %q", res.Text)
	assert.True(t, strings.Contains(res.Text, "kind=MsgCall"), "Text does not contain kind=MsgCall: %q", res.Text)
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
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "No transactions found for this realm.")
	assert.Contains(t, res.Text, `<untrusted_content kind="history"`, "history output must be wrapped as untrusted")
}

func TestFormatEvents_omitsZeroTimestamp(t *testing.T) {
	// The indexer schema doesn't expose block time, so TxEvent.Time is zero;
	// it must not render as a bogus 0001-01-01 date.
	out := formatEvents([]indexerpkg.TxEvent{{Hash: "0xabc", Height: 7, Kind: "MsgCall"}})
	assert.NotContains(t, out, "0001", "a zero TxEvent.Time must not render as a date")
	assert.Contains(t, out, "0xabc")
	assert.Contains(t, out, "height 7")
}

func TestHistory_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterHistory(s, constResolver(indexerpkg.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestHistory_profileWithoutIndexer(t *testing.T) {
	f := indexerpkg.NewFake()
	s := newBaseTestServer(t)
	RegisterHistory(s, onlyProfileResolver("testnet5", f))
	_, err := s.Registry().Call(context.Background(), "gno_history", map[string]any{
		"profile": "ghost",
		"realm":   testRealm,
	})
	require.Error(t, err, "expected error when resolver returns nil for profile without indexer")
	assert.True(t, strings.Contains(err.Error(), "no tx-indexer-url"), "error = %q, want 'no tx-indexer-url'", err.Error())
}

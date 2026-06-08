package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, "package foo\n\nfunc Bar() string", res.Text)
}

func TestInspect_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestInspect_rejectsNonStringRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when realm is not a string")
}

func TestInspect_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
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
	require.Error(t, err, "expected error when Doc returns error")
	assert.Contains(t, err.Error(), "gno_inspect:", "error should be wrapped with gno_inspect: prefix")
}

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
		"path":    "gno.land/r/foo",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Contains(t, res.Text, "package foo\n\nfunc Bar() string")
	assert.Contains(t, res.Text, `<untrusted_content kind="doc"`, "inspect output must be wrapped as untrusted")
}

func TestInspect_requiresPath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when path is missing")
}

func TestInspect_rejectsNonStringPath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"path":    42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when path is not a string")
}

func TestInspect_rejectsNonPackagePath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"path":    "std",
		"profile": "testnet5",
	})
	require.Error(t, err, "expected rejection for a non realm/pure path")
}

func TestInspect_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterInspect(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"path":    "gno.land/r/foo",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
}

// TestInspect_typedNilClientRecovers is a regression test for the profile-
// resolution crash: a resolver yielding a typed-nil *chain.Real bypasses the
// handler's `if c == nil` guard, so c.Doc derefs a nil receiver. registry.Call's
// recover must turn that into a tool error rather than segfaulting the server.
func TestInspect_typedNilClientRecovers(t *testing.T) {
	var typedNil *chain.Real
	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(chain.Client(typedNil)))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"path":    "gno.land/r/foo",
		"profile": "testnet5",
	})
	require.Error(t, err, "typed-nil client must surface as an error, not crash")
}

func TestInspect_propagatesDocError(t *testing.T) {
	// No SetDoc call — Fake.Doc will return an error for any realm.
	f := chain.NewFake()

	s := newBaseTestServer(t)
	RegisterInspect(s, constResolver(f))
	_, err := s.Registry().Call(context.Background(), "gno_inspect", map[string]any{
		"path":    "gno.land/r/foo",
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when Doc returns error")
	assert.Contains(t, err.Error(), "gno_inspect:", "error should be wrapped with gno_inspect: prefix")
}

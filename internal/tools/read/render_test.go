package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, "# Hello\nBody.", res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/foo", res.ResourceURI)
	assert.Equal(t, "text/markdown", res.ResourceMIME)
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
	require.NoError(t, err)
	assert.Equal(t, "subbody", res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/foo/subpath/x", res.ResourceURI)
}

func TestRender_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestRender_rejectsNonStringRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRender(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when realm is not a string")
}

func TestRender_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRender(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_render", map[string]any{
		"realm":   "gno.land/r/foo",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
}

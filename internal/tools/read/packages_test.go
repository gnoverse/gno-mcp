package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackages_listsUnderPrefix(t *testing.T) {
	f := chain.NewFake()
	f.SetPaths("gno.land/r/demo/", []string{"gno.land/r/demo/foo", "gno.land/r/demo/bar"})

	s := newBaseTestServer(t)
	RegisterPackages(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_packages", map[string]any{
		"path":    "gno.land/r/demo/",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "gno.land/r/demo/foo\ngno.land/r/demo/bar", res.Text)
}

func TestPackages_namespaceTarget(t *testing.T) {
	f := chain.NewFake()
	f.SetPaths("@demo", []string{"gno.land/p/demo/lib", "gno.land/r/demo/foo"})

	s := newBaseTestServer(t)
	RegisterPackages(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_packages", map[string]any{
		"path":    "@demo",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "gno.land/p/demo/lib\ngno.land/r/demo/foo", res.Text)
}

func TestPackages_requiresPath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterPackages(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_packages", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err)
}

func TestPackages_rejectsBadLimit(t *testing.T) {
	f := chain.NewFake()
	f.SetPaths("gno.land/r/demo/", []string{"gno.land/r/demo/foo"})

	s := newBaseTestServer(t)
	RegisterPackages(s, constResolver(f))
	for _, bad := range []any{-5.0, 2.5} {
		_, err := s.Registry().Call(context.Background(), "gno_packages", map[string]any{
			"path":    "gno.land/r/demo/",
			"limit":   bad,
			"profile": "testnet5",
		})
		require.Error(t, err, "expected rejection for limit=%v", bad)
	}
}

func TestPackages_acceptsValidLimit(t *testing.T) {
	f := chain.NewFake()
	f.SetPaths("gno.land/r/demo/", []string{"gno.land/r/demo/foo"})

	s := newBaseTestServer(t)
	RegisterPackages(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_packages", map[string]any{
		"path":    "gno.land/r/demo/",
		"limit":   float64(10),
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "gno.land/r/demo/foo", res.Text)
}

package read

import (
	"context"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err)
	assert.Equal(t, "package x\n\nfunc Foo() {}", res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/x/x.gno", res.ResourceURI)
	assert.Equal(t, "text/x-gno", res.ResourceMIME)
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
	require.NoError(t, err)
	assert.Equal(t, "x.gno\nhelper.gno\n", res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/x", res.ResourceURI)
	assert.Equal(t, "text/plain", res.ResourceMIME)
}

func TestRead_requiresRealm(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when realm is missing")
}

func TestRead_rejectsNonStringFile(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"file":    42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when file is not a string")
}

func TestRead_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"realm":   "gno.land/r/x",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
}

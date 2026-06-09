package read

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

func TestRead_returnsFile(t *testing.T) {
	f := chain.NewFake()
	f.SetFile("gno.land/r/demo/x", "x.gno", "package x\n\nfunc Foo() {}")

	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"path":    "gno.land/r/demo/x",
		"file":    "x.gno",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "package x\n\nfunc Foo() {}", res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/demo/x/x.gno", res.ResourceURI)
	assert.Equal(t, "text/x-gno", res.ResourceMIME)
}

func TestRead_returnsWholePackageTxtar(t *testing.T) {
	f := chain.NewFake()
	f.SetListing("gno.land/r/demo/x", []string{"x.gno", "helper.gno"})
	f.SetFile("gno.land/r/demo/x", "x.gno", "package x // main")
	f.SetFile("gno.land/r/demo/x", "helper.gno", "package x // helper")

	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	res, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"path":    "gno.land/r/demo/x",
		"profile": "testnet5",
	})
	require.NoError(t, err)
	assert.Equal(t, "gno://gno.land/r/demo/x", res.ResourceURI)
	assert.Equal(t, "text/plain", res.ResourceMIME)

	ar := txtar.Parse([]byte(res.ResourceBody))
	require.Len(t, ar.Files, 2)
	// ReadPackageFiles sorts by name: helper.gno before x.gno.
	assert.Equal(t, "helper.gno", ar.Files[0].Name)
	assert.Equal(t, "package x // helper", strings.TrimSpace(string(ar.Files[0].Data)))
	assert.Equal(t, "x.gno", ar.Files[1].Name)
	assert.Equal(t, "package x // main", strings.TrimSpace(string(ar.Files[1].Data)))
}

func TestRead_requiresPath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when path is missing")
}

func TestRead_rejectsNonPackagePath(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	for _, bad := range []string{"std", "not a path", "gno.land/e/g1abc/run"} {
		_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
			"path":    bad,
			"profile": "testnet5",
		})
		require.Error(t, err, "expected rejection for %q", bad)
	}
}

func TestRead_rejectsNonStringFile(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"path":    "gno.land/r/demo/x",
		"file":    42,
		"profile": "testnet5",
	})
	require.Error(t, err, "expected type error when file is not a string")
}

func TestRead_unknownProfileReturnsError(t *testing.T) {
	s := newBaseTestServer(t)
	RegisterRead(s, onlyProfileResolver("testnet5", chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"path":    "gno.land/r/demo/x",
		"profile": "ghost",
	})
	require.Error(t, err, "expected error when resolver returns nil for unknown profile")
}

func TestRead_emptyPackageErrors(t *testing.T) {
	f := chain.NewFake()
	f.SetListing("gno.land/r/demo/empty", []string{})

	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	_, err := s.Registry().Call(context.Background(), "gno_read", map[string]any{
		"path":    "gno.land/r/demo/empty",
		"profile": "testnet5",
	})
	require.Error(t, err, "empty/undeployed package must error, not return an empty txtar")
}

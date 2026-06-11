package read

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

const counterSrc = `package counter

var count int64

// Inc increments the counter.
func Inc(cur realm, step int64) int64 {
	count += step
	return count
}
`

func counterFake() *chain.Fake {
	f := chain.NewFake()
	f.SetListing("gno.land/r/demo/x", []string{"counter.gno", "helper.gno"})
	f.SetFile("gno.land/r/demo/x", "counter.gno", counterSrc)
	f.SetFile("gno.land/r/demo/x", "helper.gno", "package counter\n\nfunc helper() {}\n")
	return f
}

func callRead(t *testing.T, f chain.Client, args map[string]any) (server.Result, error) {
	t.Helper()
	s := newBaseTestServer(t)
	RegisterRead(s, constResolver(f))
	args["profile"] = "testnet5"
	return s.Registry().Call(context.Background(), "gno_read", args)
}

func TestRead_defaultIsPackageOutline(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{"path": "gno.land/r/demo/x"})
	require.NoError(t, err)
	assert.Equal(t, "gno://gno.land/r/demo/x#outline", res.ResourceURI)
	assert.Equal(t, "text/plain", res.ResourceMIME)
	assert.Contains(t, res.ResourceBody, "func Inc(cur realm, step int64) int64")
	assert.Contains(t, res.ResourceBody, "Inc increments the counter")
	assert.NotContains(t, res.ResourceBody, "count += step", "outline must elide bodies")

	ar := txtar.Parse([]byte(res.ResourceBody))
	require.Len(t, ar.Files, 2)
	assert.Equal(t, "counter.gno", ar.Files[0].Name)
	assert.Equal(t, "helper.gno", ar.Files[1].Name)
}

func TestRead_fileSelectsSingleFileOutline(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{
		"path": "gno.land/r/demo/x",
		"file": "counter.gno",
	})
	require.NoError(t, err)
	assert.Equal(t, "gno://gno.land/r/demo/x/counter.gno#outline", res.ResourceURI)
	assert.Contains(t, res.ResourceBody, "func Inc(cur realm, step int64) int64")
	assert.NotContains(t, res.ResourceBody, "count += step")
	ar := txtar.Parse([]byte(res.ResourceBody))
	require.Len(t, ar.Files, 1)
}

func TestRead_fullFileIsVerbatim(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{
		"path": "gno.land/r/demo/x",
		"file": "counter.gno",
		"full": true,
	})
	require.NoError(t, err)
	assert.Equal(t, counterSrc, res.ResourceBody)
	assert.Equal(t, "gno://gno.land/r/demo/x/counter.gno", res.ResourceURI)
	assert.Equal(t, "text/x-gno", res.ResourceMIME)
}

func TestRead_fullPackageIsRawTxtar(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{
		"path": "gno.land/r/demo/x",
		"full": true,
	})
	require.NoError(t, err)
	assert.Equal(t, "gno://gno.land/r/demo/x", res.ResourceURI)
	ar := txtar.Parse([]byte(res.ResourceBody))
	require.Len(t, ar.Files, 2)
	assert.Contains(t, string(ar.Files[0].Data), "count += step", "full package must carry bodies")
}

func TestRead_fullFileGetsExplicitBudget(t *testing.T) {
	// 9–20KB single files are the realistic audit payload: over the default
	// tier, well under the explicit one. full=true must deliver them whole.
	big := "package big\n\n// Blob is large.\nvar Blob = `" + strings.Repeat("x", 9000) + "`\n"
	f := chain.NewFake()
	f.SetListing("gno.land/r/demo/big", []string{"big.gno"})
	f.SetFile("gno.land/r/demo/big", "big.gno", big)

	res, err := callRead(t, f, map[string]any{
		"path": "gno.land/r/demo/big",
		"file": "big.gno",
		"full": true,
	})
	require.NoError(t, err)
	assert.Equal(t, big, res.ResourceBody, "explicit single-file request must not be summarized away")
}

func TestRead_outlineGetsExplicitBudget(t *testing.T) {
	// A package whose outline exceeds the default tier (many documented
	// exported funcs) must still render: the outline is the navigation entry
	// point and is bounded by construction (bodies elided), so it gets the
	// explicit tier. Live-found: a modest realm's outline hit 4.3KB.
	f := chain.NewFake()
	var names []string
	for i := 0; i < 40; i++ {
		name := string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".gno"
		names = append(names, name)
		f.SetFile("gno.land/r/demo/many", name,
			"package many\n\n// Exported"+name+" has a doc comment long enough to add real weight to the outline output.\nfunc Exported"+name[:2]+"(cur realm, arg string) string { return arg }\n")
	}
	f.SetListing("gno.land/r/demo/many", names)

	res, err := callRead(t, f, map[string]any{"path": "gno.land/r/demo/many"})
	require.NoError(t, err)
	assert.NotContains(t, res.ResourceBody, "preview omitted",
		"a multi-file outline must not be truncated at the default tier")
	ar := txtar.Parse([]byte(res.ResourceBody))
	assert.Len(t, ar.Files, 40)
}

func TestRead_symbolsReturnsVerbatimWithDeps(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{
		"path":    "gno.land/r/demo/x",
		"symbols": []any{"Inc"},
	})
	require.NoError(t, err)
	assert.Equal(t, "gno://gno.land/r/demo/x#symbols", res.ResourceURI)
	assert.Contains(t, res.ResourceBody, "count += step", "symbol bodies are verbatim")
	assert.Contains(t, res.ResourceBody, "// Inc increments the counter.")
	assert.Contains(t, res.ResourceBody, "// deps: count")
}

func TestRead_symbolsAllMissingErrorsWithAvailable(t *testing.T) {
	_, err := callRead(t, counterFake(), map[string]any{
		"path":    "gno.land/r/demo/x",
		"symbols": []any{"Nope"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Nope")
	assert.Contains(t, err.Error(), "Inc", "the miss error must list available symbols")
}

func TestRead_symbolsPartialMissNotedInResult(t *testing.T) {
	res, err := callRead(t, counterFake(), map[string]any{
		"path":    "gno.land/r/demo/x",
		"symbols": []any{"Inc", "Nope"},
	})
	require.NoError(t, err)
	assert.Contains(t, res.ResourceBody, "not found: Nope")
	assert.Contains(t, res.ResourceBody, "count += step")
}

func TestRead_symbolsAndFullAreMutuallyExclusive(t *testing.T) {
	_, err := callRead(t, counterFake(), map[string]any{
		"path":    "gno.land/r/demo/x",
		"symbols": []any{"Inc"},
		"full":    true,
	})
	require.Error(t, err)
}

func TestRead_symbolsAndFileAreMutuallyExclusive(t *testing.T) {
	// Symbol names are package-scoped; a file filter would silently hide
	// cross-file declarations, so the combination is rejected outright.
	_, err := callRead(t, counterFake(), map[string]any{
		"path":    "gno.land/r/demo/x",
		"file":    "counter.gno",
		"symbols": []any{"Inc"},
	})
	require.Error(t, err)
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
	_, err := callRead(t, counterFake(), map[string]any{
		"path": "gno.land/r/demo/x",
		"file": 42,
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
	_, err := callRead(t, f, map[string]any{"path": "gno.land/r/demo/empty"})
	require.Error(t, err, "empty/undeployed package must error, not return an empty txtar")
}

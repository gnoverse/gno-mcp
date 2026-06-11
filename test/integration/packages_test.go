//go:build integration
// +build integration

package integration_test

import (
	"context"
	"slices"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/gnosrc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegration_ListPaths exercises Real.ListPaths against the in-process
// node via real vm/qpaths. The node is seeded with gno.land/r/test/counter.
func TestIntegration_ListPaths(t *testing.T) {
	c := newNodeBackedReal(t)
	paths, err := c.ListPaths(context.Background(), "gno.land/r/test/", 0)
	require.NoError(t, err, "ListPaths")
	assert.Contains(t, paths, counterRealm,
		"expected %q under gno.land/r/test/, got: %v", counterRealm, paths)
}

// TestIntegration_ReadPackageFiles exercises the whole-package fetch (the
// engine behind gno_read's txtar output) against the in-process node via real
// vm/qfile, confirming names, bodies, and sort order.
func TestIntegration_ReadPackageFiles(t *testing.T) {
	c := newNodeBackedReal(t)
	files, err := chain.ReadPackageFiles(context.Background(), c, counterRealm)
	require.NoError(t, err, "ReadPackageFiles")
	require.NotEmpty(t, files, "expected at least one file")

	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	assert.True(t, slices.IsSorted(names), "files must be sorted by name, got: %v", names)
	assert.Contains(t, names, "counter.gno", "expected counter.gno in %v", names)

	var counterBody string
	for _, f := range files {
		if f.Name == "counter.gno" {
			counterBody = f.Body
		}
	}
	assert.Contains(t, counterBody, "package counter", "counter.gno body should be real source")
}

// TestIntegration_OutlineAndSymbols runs the gnosrc views (the engines behind
// gno_read's default and symbols modes) over real chain-fetched source,
// confirming on-chain bytes parse and both views hold their contracts.
func TestIntegration_OutlineAndSymbols(t *testing.T) {
	c := newNodeBackedReal(t)
	memFiles, err := chain.ReadPackageFiles(context.Background(), c, counterRealm)
	require.NoError(t, err, "ReadPackageFiles")
	files := make([]gnosrc.File, len(memFiles))
	for i, mf := range memFiles {
		files[i] = gnosrc.File{Name: mf.Name, Body: mf.Body}
	}

	outline := gnosrc.Outline(files)
	assert.Contains(t, outline, "func Increment(cur realm) int", "outline must carry signatures")
	assert.Contains(t, outline, "Increment adds 1 to the running total", "outline must carry docs")
	assert.NotContains(t, outline, "total++", "outline must elide bodies")
	assert.NotContains(t, outline, "parse error", "real on-chain source must parse cleanly")

	x := gnosrc.ExtractSymbols(files, []string{"Increment"})
	require.Equal(t, []string{"Increment"}, x.Found)
	assert.Empty(t, x.Errors)
	assert.Contains(t, x.Body, "total++", "symbol body must be verbatim")
	assert.Contains(t, x.Body, "// deps: total", "dep header must resolve the package-level var")
}

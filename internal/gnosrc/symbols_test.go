package gnosrc

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"
)

func TestExtractSymbols_funcVerbatimWithDoc(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Inc"})
	require.Equal(t, []string{"Inc"}, x.Found)
	assert.Empty(t, x.Missing)
	assert.Contains(t, x.Body, "// Inc increments the counter and returns the new value.")
	assert.Contains(t, x.Body, "count += step", "body must be verbatim, not elided")
	assert.Contains(t, x.Body, `panic("step too large")`)

	ar := txtar.Parse([]byte(x.Body))
	require.Len(t, ar.Files, 1)
	assert.Equal(t, "counter.gno", ar.Files[0].Name)
}

func TestExtractSymbols_depsHeader(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Inc"})
	assert.Contains(t, x.Body, "// deps:")
	assert.Contains(t, x.Body, "MaxStep")
	assert.Contains(t, x.Body, "Tree")
	assert.Contains(t, x.Body, "count")
	assert.Contains(t, x.Body, "// imports: chain/runtime")
}

func TestExtractSymbols_unresolvedMethodCallsAreLoud(t *testing.T) {
	// Inc has two unresolved method calls: .Address() on the PreviousRealm()
	// result and Tree.Set(...). The header must say the dep list is incomplete.
	x := ExtractSymbols(fixtureFiles(), []string{"Inc"})
	assert.Contains(t, x.Body, "+2 unresolved")
	assert.Contains(t, x.Body, "read the full file")
}

func TestExtractSymbols_noUnresolvedMarkerWhenComplete(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Reset"})
	assert.NotContains(t, x.Body, "unresolved")
}

func TestExtractSymbols_method(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Counter.Add"})
	require.Equal(t, []string{"Counter.Add"}, x.Found)
	assert.Contains(t, x.Body, "c.N += n")
	assert.Contains(t, x.Body, "// Add adds n to the wrapped count.")
}

func TestExtractSymbols_crossFileDeps(t *testing.T) {
	// Reset (helper.gno) references count (counter.gno) and reset (same file).
	x := ExtractSymbols(fixtureFiles(), []string{"Reset"})
	assert.Contains(t, x.Body, "// deps:")
	assert.Contains(t, x.Body, "count")
	assert.Contains(t, x.Body, "reset")
}

func TestExtractSymbols_localShadowNotADep(t *testing.T) {
	files := []File{{Name: "shadow.gno", Body: `package shadow

var count int64

// Snapshot returns a constant.
func Snapshot() int64 {
	count := int64(1)
	return count
}
`}}
	x := ExtractSymbols(files, []string{"Snapshot"})
	require.Equal(t, []string{"Snapshot"}, x.Found)
	assert.NotContains(t, x.Body, "// deps:", "local shadow of a package-level name is not a dep")
}

func TestExtractSymbols_varWithDoc(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Tree"})
	require.Equal(t, []string{"Tree"}, x.Found)
	assert.Contains(t, x.Body, "// Tree indexes per-user counts.")
	assert.Contains(t, x.Body, "avl.NewTree()", "var initializer must be verbatim")
}

func TestExtractSymbols_groupedDeclReturnsWholeBlock(t *testing.T) {
	files := []File{{Name: "group.gno", Body: `package group

var (
	// A is first.
	A = 1
	b = 2
)
`}}
	x := ExtractSymbols(files, []string{"A"})
	require.Equal(t, []string{"A"}, x.Found)
	// Verbatim slicing never splits a grouped decl: siblings come along.
	assert.Contains(t, x.Body, "A = 1")
	assert.Contains(t, x.Body, "b = 2")
}

func TestExtractSymbols_siblingsOfOneGroupEmitOnce(t *testing.T) {
	files := []File{{Name: "group.gno", Body: `package group

var (
	A = 1
	B = 2
)
`}}
	x := ExtractSymbols(files, []string{"A", "B"})
	require.Equal(t, []string{"A", "B"}, x.Found, "both names are found")
	assert.Equal(t, 1, strings.Count(x.Body, "A = 1"),
		"two names resolving to one grouped decl must emit the block once")
}

func TestExtractSymbols_methodReceiverTypeIsNotASelfDep(t *testing.T) {
	// Requesting Counter.Add already names Counter; listing it as a dep is
	// noise. Genuine deps in the body must survive.
	x := ExtractSymbols(fixtureFiles(), []string{"Counter.Add"})
	require.Equal(t, []string{"Counter.Add"}, x.Found)
	assert.NotContains(t, x.Body, "// deps: Counter")
}

func TestExtractSymbols_missingReportedWithAvailable(t *testing.T) {
	x := ExtractSymbols(fixtureFiles(), []string{"Nope", "Inc"})
	assert.Equal(t, []string{"Inc"}, x.Found)
	assert.Equal(t, []string{"Nope"}, x.Missing)
	assert.Contains(t, x.Available, "Inc")
	assert.Contains(t, x.Available, "Reset")
	assert.Contains(t, x.Available, "Counter")
	assert.Contains(t, x.Available, "Counter.Add")
	assert.Contains(t, x.Available, "count")
}

func TestExtractSymbols_parseErrorSurfacedNotSilent(t *testing.T) {
	files := append(fixtureFiles(), File{Name: "bad.gno", Body: "package \x01 nope"})
	x := ExtractSymbols(files, []string{"Inc"})
	require.Equal(t, []string{"Inc"}, x.Found, "a broken sibling file must not block extraction")
	require.Len(t, x.Errors, 1)
	assert.Contains(t, x.Errors[0], "bad.gno")
}

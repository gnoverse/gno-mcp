package auditharness

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadRecordResolvesFixturePaths(t *testing.T) {
	rec, err := LoadRecord(filepath.Join("..", "..", "audit-harness", "expected", "current-guard.yaml"))
	require.NoError(t, err)
	require.Equal(t, "current-guard", rec.ID)
	require.Len(t, rec.Fixtures, 2)
	require.True(t, filepath.IsAbs(rec.Fixtures[0].Path))
}

func TestCurrentGuardRule(t *testing.T) {
	base := filepath.Join("..", "..", "audit-harness", "fixtures", "current-guard")

	hits, err := RunRule("current_guard", filepath.Join(base, "vulnerable"))
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "admin.gno", hits[0].File)
	require.Equal(t, 6, hits[0].Line)

	hits, err = RunRule("current_guard", filepath.Join(base, "fixed"))
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestRenderMarkdownEscapeRule(t *testing.T) {
	base := filepath.Join("..", "..", "audit-harness", "fixtures", "render-markdown")

	hits, err := RunRule("render_markdown_escape", filepath.Join(base, "vulnerable"))
	require.NoError(t, err)
	require.Len(t, hits, 1)
	require.Equal(t, "echo.gno", hits[0].File)

	hits, err = RunRule("render_markdown_escape", filepath.Join(base, "fixed"))
	require.NoError(t, err)
	require.Empty(t, hits)
}

func TestRunWithFakeGNO(t *testing.T) {
	tmp := t.TempDir()
	gno := filepath.Join(tmp, "gno")
	require.NoError(t, os.WriteFile(gno, []byte("#!/bin/sh\necho ok\n"), 0o755))

	rec, err := LoadRecord(filepath.Join("..", "..", "audit-harness", "expected", "current-guard.yaml"))
	require.NoError(t, err)

	report := Run(context.Background(), rec, Options{GNOBin: gno})
	require.True(t, report.OK)
	require.Len(t, report.Fixtures, 2)
	for _, fixture := range report.Fixtures {
		require.True(t, fixture.PathOK)
		require.True(t, fixture.GNOTestOK)
		require.True(t, fixture.PatternExpectationOK)
	}
}

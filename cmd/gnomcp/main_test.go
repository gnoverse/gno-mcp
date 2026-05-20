package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// srcDir returns the directory containing this test file.
func srcDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

func TestVersionSubcommand(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	cmd.Dir = srcDir()
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != version {
		t.Errorf("output = %q, want %q", got, version)
	}
}

func TestMissingProfileTOML_exitsCleanly(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--config", filepath.Join(t.TempDir(), "nope.toml"))
	cmd.Dir = srcDir()
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when config missing")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() == 0 {
		t.Errorf("expected exit code != 0, got: %v", err)
	}
}

func TestAuditGrep(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	// seed a few JSON-lines audit entries
	entries := []map[string]any{
		{"time": "2024-01-01T00:00:00Z", "tool": "gno_render", "result": "ok"},
		{"time": "2024-01-01T00:01:00Z", "tool": "gno_eval", "result": "tool_err"},
		{"time": "2024-01-01T00:02:00Z", "tool": "gno_render", "result": "ok"},
	}
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			t.Fatalf("encode entry: %v", err)
		}
	}
	f.Close()

	// grep for "gno_eval" — should return exactly 1 line
	cmd := exec.Command("go", "run", ".", "audit", "grep", "gno_eval")
	cmd.Dir = srcDir()
	cmd.Env = append(os.Environ(), "GNOMCP_AUDIT_PATH="+logPath)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("audit grep: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 matching line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "gno_eval") {
		t.Errorf("line does not contain 'gno_eval': %s", lines[0])
	}

	// grep for "gno_render" — should return 2 lines
	cmd2 := exec.Command("go", "run", ".", "audit", "grep", "gno_render")
	cmd2.Dir = srcDir()
	cmd2.Env = append(os.Environ(), "GNOMCP_AUDIT_PATH="+logPath)
	out2, err := cmd2.Output()
	if err != nil {
		t.Fatalf("audit grep 2: %v", err)
	}
	lines2 := strings.Split(strings.TrimSpace(string(out2)), "\n")
	if len(lines2) != 2 {
		t.Errorf("expected 2 matching lines, got %d: %v", len(lines2), lines2)
	}
}

package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "run")
	got := strings.TrimSpace(string(out))
	assert.Equal(t, version, got)
}

func TestMissingProfileTOML_exitsCleanly(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "--config", filepath.Join(t.TempDir(), "nope.toml"))
	cmd.Dir = srcDir()
	err := cmd.Run()
	require.Error(t, err, "expected non-zero exit when config missing")
	exitErr, ok := err.(*exec.ExitError)
	require.True(t, ok && exitErr.ExitCode() != 0, "expected exit code != 0, got: %v", err)
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
	require.NoError(t, err, "create audit log")
	enc := json.NewEncoder(f)
	for _, e := range entries {
		require.NoError(t, enc.Encode(e), "encode entry")
	}
	f.Close()

	// grep for "gno_eval" — should return exactly 1 line
	cmd := exec.Command("go", "run", ".", "audit", "grep", "gno_eval")
	cmd.Dir = srcDir()
	cmd.Env = append(os.Environ(), "GNOMCP_AUDIT_PATH="+logPath)
	out, err := cmd.Output()
	require.NoError(t, err, "audit grep")
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.Len(t, lines, 1, "expected 1 matching line, got %d: %v", len(lines), lines)
	assert.Contains(t, lines[0], "gno_eval")

	// grep for "gno_render" — should return 2 lines
	cmd2 := exec.Command("go", "run", ".", "audit", "grep", "gno_render")
	cmd2.Dir = srcDir()
	cmd2.Env = append(os.Environ(), "GNOMCP_AUDIT_PATH="+logPath)
	out2, err := cmd2.Output()
	require.NoError(t, err, "audit grep 2")
	lines2 := strings.Split(strings.TrimSpace(string(out2)), "\n")
	assert.Len(t, lines2, 2, "expected 2 matching lines, got %d: %v", len(lines2), lines2)
}

func TestMain_writeToolsAbsent_whenAllReadOnly(t *testing.T) {
	toml := `
[testnet5]
chain-type = "testnet"
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
`
	cfg := t.TempDir()
	cfgFile := filepath.Join(cfg, "profiles.toml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(toml), 0o600))
	sessDir := t.TempDir()

	cmd := exec.Command("go", "run", ".", "-config", cfgFile, "-sessions-path", sessDir)
	cmd.Dir = srcDir()
	cmd.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		require.ErrorAs(t, err, &ee)
	}

	for _, tool := range []string{"gno_call", "gno_run", "gno_auth_status", "gno_session_propose", "gno_session_revoke"} {
		assert.NotContains(t, string(out), tool,
			"write tool %q should not be in initialize response for read-only profile (no master-address)", tool)
	}
}

func TestMain_writeToolsPresent_whenWritableProfile(t *testing.T) {
	toml := `
[testnet5]
chain-type = "testnet"
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`
	cfg := t.TempDir()
	cfgFile := filepath.Join(cfg, "profiles.toml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(toml), 0o600))
	sessDir := t.TempDir()

	cmd := exec.Command("go", "run", ".", "-config", cfgFile, "-sessions-path", sessDir)
	cmd.Dir = srcDir()
	// Use a StdinPipe so we control when the server sees EOF.
	// Closing stdin too soon races with MCP's async response dispatch:
	// the server closes stdout when stdin reaches EOF, which may happen
	// before the response is written. We keep stdin open until we receive
	// the tools/list response (signalled by reading from stdout), then
	// close it to let the server exit cleanly.
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err, "stdin pipe")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "stdout pipe")
	require.NoError(t, cmd.Start(), "start")

	const (
		initMsg = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n"
		listMsg = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	)
	_, err = stdin.Write([]byte(initMsg + listMsg))
	require.NoError(t, err, "write init+list")

	// Read all stdout in a goroutine; the server will not write after stdin
	// closes, so draining stdout first is safe.
	outCh := make(chan []byte, 1)
	go func() {
		var buf strings.Builder
		tmp := make([]byte, 4096)
		for {
			n, readErr := stdout.Read(tmp)
			if n > 0 {
				buf.Write(tmp[:n])
			}
			if readErr != nil {
				break
			}
		}
		outCh <- []byte(buf.String())
	}()

	// Give the server enough time to respond to both requests, then close
	// stdin so it exits cleanly.  500 ms is generous; a cached go run
	// typically responds in <100 ms.
	timer := time.NewTimer(500 * time.Millisecond)
	<-timer.C
	stdin.Close()

	out := string(<-outCh)

	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		require.ErrorAs(t, err, &ee)
	}

	for _, tool := range []string{"gno_call", "gno_run", "gno_session_propose", "gno_session_revoke", "gno_auth_status"} {
		assert.Contains(t, out, tool,
			"write tool %q missing from initialize response for writable profile (master-address set)", tool)
	}
}

func TestResolveSources_Defaults(t *testing.T) {
	t.Setenv("GNOMCP_CONFIG", "")
	s := resolveSources("")
	assert.NotEmpty(t, s.GlobalPath, "expected a global path default")
	assert.Equal(t, "profiles.toml", s.ProjectPath)
}

func TestInitialize_instructions_mentionsSessions_whenWritable(t *testing.T) {
	toml := `
[testnet5]
chain-type = "testnet"
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
master-address = "g17ernafy6ctpcz6uepfsq2js8x2vz0wladh5yc3"
`
	cfgDir := t.TempDir()
	cfgFile := filepath.Join(cfgDir, "profiles.toml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(toml), 0o600))
	sessDir := t.TempDir()

	cmd := exec.Command("go", "run", ".", "-config", cfgFile, "-sessions-path", sessDir)
	cmd.Dir = srcDir()
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())

	var out []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, _ = io.ReadAll(stdout)
	}()

	_, err = io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`+"\n")
	require.NoError(t, err, "write init")
	time.Sleep(500 * time.Millisecond)
	stdin.Close()
	cmd.Wait()
	<-done

	assert.Contains(t, string(out), "gno_session_propose",
		"expected instructions to mention gno_session_propose; response:\n%s", string(out))
}

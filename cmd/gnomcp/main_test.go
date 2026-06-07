package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestMain_writeToolsAbsent_whenAllReadOnly(t *testing.T) {
	toml := `
[testnet5]
chain-type = "testnet"
rpc-url = "https://rpc.test5.gno.land:443"
chain-id = "test5"
`
	cfg := t.TempDir()
	cfgFile := filepath.Join(cfg, "profiles.toml")
	if err := os.WriteFile(cfgFile, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	sessDir := t.TempDir()

	cmd := exec.Command("go", "run", ".", "-config", cfgFile, "-sessions-path", sessDir)
	cmd.Dir = srcDir()
	cmd.Stdin = strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n")
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run: %v", err)
		}
	}

	for _, tool := range []string{"gno_call", "gno_run", "gno_auth_status", "gno_session_propose", "gno_session_revoke"} {
		if strings.Contains(string(out), tool) {
			t.Errorf("write tool %q should not be in initialize response for read-only profile (no master-address)", tool)
		}
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
	if err := os.WriteFile(cfgFile, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
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
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	const (
		initMsg = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}` + "\n"
		listMsg = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	)
	if _, err := stdin.Write([]byte(initMsg + listMsg)); err != nil {
		t.Fatalf("write init+list: %v", err)
	}

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
		if !errors.As(err, &ee) {
			t.Fatalf("run: %v", err)
		}
	}

	for _, tool := range []string{"gno_call", "gno_run", "gno_session_propose", "gno_session_revoke", "gno_auth_status"} {
		if !strings.Contains(out, tool) {
			t.Errorf("write tool %q missing from initialize response for writable profile (master-address set); response:\n%s", tool, out)
		}
	}
}

func TestResolveSources_Defaults(t *testing.T) {
	t.Setenv("GNOMCP_CONFIG", "")
	s := resolveSources("")
	if s.GlobalPath == "" {
		t.Error("expected a global path default")
	}
	if s.ProjectPath != "profiles.toml" {
		t.Errorf("project path = %q, want profiles.toml", s.ProjectPath)
	}
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
	if err := os.WriteFile(cfgFile, []byte(toml), 0o600); err != nil {
		t.Fatal(err)
	}
	sessDir := t.TempDir()

	cmd := exec.Command("go", "run", ".", "-config", cfgFile, "-sessions-path", sessDir)
	cmd.Dir = srcDir()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var out []byte
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, _ = io.ReadAll(stdout)
	}()

	if _, err := io.WriteString(stdin, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`+"\n"); err != nil {
		t.Fatalf("write init: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	stdin.Close()
	cmd.Wait()
	<-done

	if !strings.Contains(string(out), "gno_session_propose") {
		t.Errorf("expected instructions to mention gno_session_propose; response:\n%s", string(out))
	}
}

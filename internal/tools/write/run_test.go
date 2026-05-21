package write

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

const testCode = `package main; func main() { println("hello") }`

func TestRun_happyPath(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xrun1",
		Height:  10,
		Output:  "hello\n",
		GasUsed: 4000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot")
	})

	RegisterRun(s, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res.Text, "0xrun1") {
		t.Errorf("expected tx hash in text:\n%s", res.Text)
	}
	sc := res.StructuredContent
	if sc["tx_hash"] != "0xrun1" {
		t.Errorf("expected tx_hash=0xrun1, got %v", sc["tx_hash"])
	}
	if sc["simulated"] != false {
		t.Errorf("expected simulated=false, got %v", sc["simulated"])
	}

	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Tool != "gno_run" {
		t.Errorf("audit tool=%q", entries[0].Tool)
	}
	if entries[0].Result != "ok" {
		t.Errorf("audit result=%q", entries[0].Result)
	}
}

func TestRun_missingProfile(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"code": testCode,
	})
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
	if !strings.Contains(err.Error(), "profile") {
		t.Errorf("expected 'profile' in error: %v", err)
	}
}

func TestRun_missingCode(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error for missing code")
	}
	if !strings.Contains(err.Error(), "code") {
		t.Errorf("expected 'code' in error: %v", err)
	}
}

func TestRun_dangerousDisabled(t *testing.T) {
	s := newReadOnlyTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err == nil {
		t.Fatal("expected dangerous_disabled error")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "dangerous_disabled" {
		t.Errorf("expected code=dangerous_disabled, got %q", te.Code)
	}
}

func TestRun_authenticationRequired(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t) // no sessions
	fake := chain.NewFake()
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err == nil {
		t.Fatal("expected authentication_required error")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "authentication_required" {
		t.Errorf("expected code=authentication_required, got %q", te.Code)
	}
}

func TestRun_picksAnySessionWhenNoRealm(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xany",
		Height:  5,
		Output:  "ok\n",
		GasUsed: 1000,
	})

	// Session covers a specific realm — gno_run should still pick it because
	// realm="" is a wildcard (MsgRun scoping is chain-side per AllowPaths).
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot")
	})

	RegisterRun(s, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err != nil {
		t.Fatalf("expected success with any active session: %v", err)
	}
	if res.StructuredContent["tx_hash"] != "0xany" {
		t.Errorf("unexpected tx_hash: %v", res.StructuredContent["tx_hash"])
	}
}

func TestRun_simulateBypasses_session(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		Output:  "simulated\n",
		GasUsed: 500,
	})

	mgr := noSessionMgr(t) // no session — simulate should bypass
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"simulate": true,
	})
	if err != nil {
		t.Fatalf("simulate Run: %v", err)
	}
	if res.StructuredContent["simulated"] != true {
		t.Errorf("expected simulated=true, got %v", res.StructuredContent["simulated"])
	}

	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Result != "sim" {
		t.Errorf("expected audit result=sim, got %q", entries[0].Result)
	}
}

func TestRun_simulateUnsupported(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunError(testCode, chain.ErrSimulateUnsupported)

	mgr := noSessionMgr(t)
	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"simulate": true,
	})
	if err == nil {
		t.Fatal("expected simulate_unsupported error")
	}
	var te *server.ToolError
	if !errors.As(err, &te) {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "simulate_unsupported" {
		t.Errorf("expected code=simulate_unsupported, got %q", te.Code)
	}
}

func TestRun_updatesSessionSpend(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xspend",
		GasUsed: 7000,
		Output:  "ok\n",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot")
	})

	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := mgr.Get("testnet5", sessionAddr)
	if meta == nil {
		t.Fatal("session not found after run")
	}
	if meta.SpendRemaining == "1000000ugnot" {
		t.Errorf("SpendRemaining was not updated: %s", meta.SpendRemaining)
	}
}

func TestRun_writesAuditEntry(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xaudit",
		GasUsed: 2500,
		Output:  "ok\n",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot")
	})

	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Tool != "gno_run" {
		t.Errorf("audit Tool=%q", e.Tool)
	}
	if e.Profile != "testnet5" {
		t.Errorf("audit Profile=%q", e.Profile)
	}
	if e.SessionAddress != sessionAddr {
		t.Errorf("audit SessionAddress=%q, want %q", e.SessionAddress, sessionAddr)
	}
	if e.Result != "ok" {
		t.Errorf("audit Result=%q", e.Result)
	}
	if e.Duration < 0 {
		t.Errorf("audit Duration=%d (negative)", e.Duration)
	}
}

func TestRun_broadcastError_auditsResult(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunError(testCode, errors.New("broadcast failed"))

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot")
	})

	RegisterRun(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
	})
	if err == nil {
		t.Fatal("expected broadcast error to propagate")
	}

	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry even on broadcast error, got %d", len(entries))
	}
	if entries[0].Result != "broadcast_err" {
		t.Errorf("expected audit result=broadcast_err, got %q", entries[0].Result)
	}
}

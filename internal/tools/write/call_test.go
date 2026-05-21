package write

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// ---- helpers

// seedActiveSession adds a pending session to mgr and promotes it to active.
// Returns the session address.
func seedActiveSession(t *testing.T, mgr *session.Manager, profile string, allowPaths []string, spendLimit string) string {
	t.Helper()
	return seedActiveSessionWithRun(t, mgr, profile, allowPaths, spendLimit, false)
}

// seedActiveSessionWithRun is like seedActiveSession but lets the caller set
// AllowRun on both the proposed scope and the activated chain status.
func seedActiveSessionWithRun(t *testing.T, mgr *session.Manager, profile string, allowPaths []string, spendLimit string, allowRun bool) string {
	t.Helper()
	kp, err := session.NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	scope := session.Scope{
		AllowPaths: allowPaths,
		AllowRun:   allowRun,
		SpendLimit: spendLimit,
	}
	meta, err := mgr.AddPending(profile, kp, scope, "g1master")
	if err != nil {
		t.Fatalf("AddPending: %v", err)
	}
	status := chain.SessionStatus{
		Active:         true,
		AllowPaths:     scope.AllowPaths,
		AllowRun:       scope.AllowRun,
		SpendLimit:     scope.SpendLimit,
		SpendRemaining: scope.SpendLimit,
	}
	if err := mgr.MarkActive(profile, meta.SessionAddress, status); err != nil {
		t.Fatalf("MarkActive: %v", err)
	}
	return meta.SessionAddress
}

// parseAuditEntries decodes all JSON-line entries from buf.
func parseAuditEntries(t *testing.T, buf *bytes.Buffer) []audit.Entry {
	t.Helper()
	var entries []audit.Entry
	dec := json.NewDecoder(buf)
	for dec.More() {
		var e audit.Entry
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("decode audit entry: %v", err)
		}
		entries = append(entries, e)
	}
	return entries
}

// ---- tests

func TestCall_happyPath(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{"1"}, chain.CallResult{
		TxHash:  "0xabc",
		Height:  42,
		Result:  "ok",
		GasUsed: 5000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		"args":    []any{"1"},
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "0xabc") {
		t.Errorf("expected tx hash in text:\n%s", res.Text)
	}
	sc := res.StructuredContent
	if sc["tx_hash"] != "0xabc" {
		t.Errorf("expected tx_hash=0xabc, got %v", sc["tx_hash"])
	}
	if sc["simulated"] != false {
		t.Errorf("expected simulated=false, got %v", sc["simulated"])
	}

	// Audit entry written.
	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Tool != "gno_call" {
		t.Errorf("audit tool=%q", entries[0].Tool)
	}
	if entries[0].Result != "ok" {
		t.Errorf("audit result=%q", entries[0].Result)
	}
}

func TestCall_missingRealm(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"func":    "Increment",
	})
	if err == nil {
		t.Fatal("expected error for missing realm")
	}
	if !strings.Contains(err.Error(), "realm") {
		t.Errorf("expected 'realm' in error: %v", err)
	}
}

func TestCall_missingFunc(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
	})
	if err == nil {
		t.Fatal("expected error for missing func")
	}
	if !strings.Contains(err.Error(), "func") {
		t.Errorf("expected 'func' in error: %v", err)
	}
}

func TestCall_wrongTypeArgs(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		"args":    []any{42}, // non-string element
	})
	if err == nil {
		t.Fatal("expected type error for non-string args element")
	}
}

func TestCall_dangerousDisabled(t *testing.T) {
	s := newReadOnlyTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if err == nil {
		t.Fatal("expected dangerous_disabled error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "dangerous_disabled" {
		t.Errorf("expected code=dangerous_disabled, got %q", te.Code)
	}
}

func TestCall_authenticationRequired(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t) // no sessions
	fake := chain.NewFake()
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if err == nil {
		t.Fatal("expected authentication_required error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "authentication_required" {
		t.Errorf("expected code=authentication_required, got %q", te.Code)
	}
}

func TestCall_scopeMismatch(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	// Session covers a different realm.
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/other/realm"}, "1000000ugnot")
	})

	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if err == nil {
		t.Fatal("expected scope_mismatch error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "scope_mismatch" {
		t.Errorf("expected code=scope_mismatch, got %q", te.Code)
	}
	// available_paths must be present in Extra.
	if _, ok := te.Extra["available_paths"]; !ok {
		t.Errorf("expected available_paths in ToolError.Extra: %v", te.Extra)
	}
}

func TestCall_simulateRequiresSession(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	mgr := noSessionMgr(t) // no session — simulate must still error
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
	})
	if err == nil {
		t.Fatal("expected authentication_required for simulate without session")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "authentication_required" {
		t.Errorf("expected code=authentication_required, got %q", te.Code)
	}
}

func TestCall_simulateWithSession(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "",
		Result:  "simulated",
		GasUsed: 1000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
	})
	if err != nil {
		t.Fatalf("simulate Call: %v", err)
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

func TestCall_simulateUnsupported(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCallError("gno.land/r/test/counter", "Increment", chain.ErrSimulateUnsupported)

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
	})
	if err == nil {
		t.Fatal("expected simulate_unsupported error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "simulate_unsupported" {
		t.Errorf("expected code=simulate_unsupported, got %q", te.Code)
	}
}

func TestCall_updatesSessionSpend(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "0xdef",
		GasUsed: 3000,
		Result:  "ok",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	meta := mgr.Get("testnet5", sessionAddr)
	if meta == nil {
		t.Fatal("session not found after call")
	}
	// SpendRemaining must have decreased from 1000000 by 3000.
	if meta.SpendRemaining == "1000000ugnot" {
		t.Errorf("SpendRemaining was not updated: %s", meta.SpendRemaining)
	}
}

func TestCall_writesAuditEntry(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{"5"}, chain.CallResult{
		TxHash:  "0xfeed",
		GasUsed: 2500,
		Result:  "ok",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		"args":    []any{"5"},
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Tool != "gno_call" {
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
	// ArgsSummary should contain the realm.
	if !strings.Contains(e.ArgsSummary, "gno.land/r/test/counter") {
		t.Errorf("audit ArgsSummary missing realm: %q", e.ArgsSummary)
	}
}

func TestCall_simulateError_auditsSimErr(t *testing.T) {
	s := newBaseTestServer(t)
	f := chain.NewFake()
	f.SetCallError("gno.land/r/test/counter", "Increment", errors.New("node unavailable"))
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	var auditBuf bytes.Buffer
	RegisterCall(s, mgr, constChainResolver(f), audit.NewLog(&auditBuf))

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
	})
	if err == nil {
		t.Fatal("expected simulate error to propagate")
	}
	entries := parseAuditEntries(t, &auditBuf)
	if len(entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(entries))
	}
	if entries[0].Result != "sim_err" {
		t.Errorf("expected result=sim_err, got %q", entries[0].Result)
	}
}

func TestCall_broadcastError_auditsResult(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCallError("gno.land/r/test/counter", "Increment", errors.New("broadcast failed"))

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
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

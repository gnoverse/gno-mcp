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
	"github.com/gnoverse/gno-mcp/internal/keystore"
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
	fake.SetCallAsUser("gno.land/r/test/counter", "Increment", []string{"1"}, chain.CallResult{
		TxHash:  "0xabc",
		Height:  42,
		Result:  "ok",
		GasUsed: 5000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"args":     []any{"1"},
		"identity": "session",
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
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"func":     "Increment",
		"identity": "session",
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
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"identity": "session",
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
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"args":     []any{42}, // non-string element
		"identity": "session",
	})
	if err == nil {
		t.Fatal("expected type error for non-string args element")
	}
}

func TestCall_authenticationRequired(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t) // no sessions
	fake := chain.NewFake()
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session",
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

	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session",
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
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
		"identity": "session",
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
	fake.SetCallAsUser("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "",
		Result:  "simulated",
		GasUsed: 1000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
		"identity": "session",
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
	fake.SetCallAsUserError("gno.land/r/test/counter", "Increment", chain.ErrSimulateUnsupported)

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
		"identity": "session",
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
	fake.SetCallAsUser("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "0xdef",
		GasUsed: 3000, // actual gas — must be IGNORED for spend (chain bills the GasFee)
		Result:  "ok",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "100000000ugnot")
	})

	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	meta := mgr.Get("testnet5", sessionAddr)
	if meta == nil {
		t.Fatal("session not found after call")
	}
	// The chain bills the full GasFee (10M), not GasUsed (3000): 100M - 10M = 90M.
	// Guards #5: local spend tracking must match the chain's GasFee accounting.
	if meta.SpendRemaining != "90000000ugnot" {
		t.Errorf("SpendRemaining: got %s, want 90000000ugnot (deduct GasFee, not GasUsed)", meta.SpendRemaining)
	}
}

func TestCall_writesAuditEntry(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCallAsUser("gno.land/r/test/counter", "Increment", []string{"5"}, chain.CallResult{
		TxHash:  "0xfeed",
		GasUsed: 2500,
		Result:  "ok",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"args":     []any{"5"},
		"identity": "session",
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
	f.SetCallAsUserError("gno.land/r/test/counter", "Increment", errors.New("node unavailable"))
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	var auditBuf bytes.Buffer
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(f), audit.NewLog(&auditBuf))

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
		"identity": "session",
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

// TestCall_NoSession_ReadOnlyProfile verifies that a read-only profile (no
// master-address) returns authentication_required when there is no active
// session — the registration gate must not exist.
func TestCall_NoSession_ReadOnlyProfile(t *testing.T) {
	s := newReadOnlyTestServer(t) // no master-address (read-only)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t) // empty session manager
	fake := chain.NewFake()
	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/echo",
		"func":     "Echo",
		"args":     []any{"hi"},
		"identity": "session",
	})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var te *server.ToolError
	if !errors.As(err, &te) || te.Code != "authentication_required" {
		t.Fatalf("want authentication_required, got %v", err)
	}
}

func TestCall_broadcastError_auditsResult(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetCallAsUserError("gno.land/r/test/counter", "Increment", errors.New("broadcast failed"))

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})

	RegisterCall(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session",
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

// ---- identity selector tests

// TestCall_agentIdentity_local verifies that on a local profile with no identity
// arg, gno_call defaults to "agent" and uses c.Call (not c.CallAsUser).
func TestCall_agentIdentity_local(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{"1"}, chain.CallResult{
		TxHash:  "0xagent1",
		Height:  7,
		Result:  "ok",
		GasUsed: 3000,
	})

	mgr := noSessionMgr(t) // no sessions — agent path must not need one
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "local",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		"args":    []any{"1"},
	})
	if err != nil {
		t.Fatalf("Call (agent): %v", err)
	}
	if !strings.Contains(res.Text, "0xagent1") {
		t.Errorf("expected tx hash in text:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "Signed by: agent test1 ("+keystore.Test1Address+")") {
		t.Errorf("expected signed-by agent line in text:\n%s", res.Text)
	}
	sc := res.StructuredContent
	if sc["identity"] != "agent" {
		t.Errorf("expected identity=agent, got %v", sc["identity"])
	}
	if sc["signer_address"] != keystore.Test1Address {
		t.Errorf("expected signer_address=%s, got %v", keystore.Test1Address, sc["signer_address"])
	}
	if _, ok := sc["master_address"]; ok {
		t.Errorf("expected no master_address for agent identity, got %v", sc["master_address"])
	}
}

// TestCall_sessionIdentity_explicit verifies that explicitly passing identity="session"
// on a testnet profile uses c.CallAsUser and includes the signed-by session line.
func TestCall_sessionIdentity_explicit(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	fake.SetCallAsUser("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "0xsess1",
		Height:  9,
		Result:  "ok",
		GasUsed: 4000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSession(t, m, "testnet5", []string{"gno.land/r/test/counter"}, "1000000ugnot")
	})
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session",
	})
	if err != nil {
		t.Fatalf("Call (session): %v", err)
	}
	if !strings.Contains(res.Text, "Signed by: session") {
		t.Errorf("expected signed-by session line in text:\n%s", res.Text)
	}
	sc := res.StructuredContent
	if sc["identity"] != "session" {
		t.Errorf("expected identity=session, got %v", sc["identity"])
	}
	if sc["master_address"] == nil {
		t.Errorf("expected master_address for session identity, got nil")
	}
}

// TestCall_defaultAgent_testnet verifies that on a testnet profile with no
// identity arg and no agent key, gno_call defaults to "agent" and returns
// agent_identity_unavailable (not authentication_required).
// The new default is tested here; use identity=session to opt into the session path.
func TestCall_defaultAgent_testnet(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "") // no agent key

	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		// no identity — defaults to agent on testnet
	})
	if err == nil {
		t.Fatal("expected agent_identity_unavailable error (default=agent on testnet, no key generated)")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "agent_identity_unavailable" {
		t.Errorf("expected code=agent_identity_unavailable, got %q", te.Code)
	}
}

// TestCall_defaultAgent_testnet_noKey verifies that on a testnet profile with no
// identity arg and no agent key, gno_call defaults to "agent" and returns
// agent_identity_unavailable whose message mentions gno_key_generate.
func TestCall_defaultAgent_testnet_noKey(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "") // no key generated

	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet5",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
		// no identity — must default to agent on testnet
	})
	if err == nil {
		t.Fatal("expected agent_identity_unavailable error")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "agent_identity_unavailable" {
		t.Errorf("expected code=agent_identity_unavailable, got %q", te.Code)
	}
	if !strings.Contains(te.Message, "gno_key_generate") {
		t.Errorf("expected message to mention gno_key_generate, got: %q", te.Message)
	}
}

// TestCall_sessionIdentity_testnet_explicit verifies that explicitly passing
// identity="session" on a testnet profile still routes to the session path.
func TestCall_sessionIdentity_testnet_explicit(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "") // no agent key

	fake := chain.NewFake()
	mgr := noSessionMgr(t) // no session either — confirms we go session path (not agent)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet5",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "session", // explicit override
	})
	if err == nil {
		t.Fatal("expected authentication_required (session path, no active session)")
	}
	te, ok := errors.AsType[*server.ToolError](err)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", err, err)
	}
	if te.Code != "authentication_required" {
		t.Errorf("expected code=authentication_required, got %q", te.Code)
	}
}

// TestCall_agentTestnet_insufficientFunds verifies that gno_call returns
// insufficient_funds when the agent's testnet account has zero balance.
func TestCall_agentTestnet_insufficientFunds(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	// Generate an agent key for the testnet profile.
	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	// Fake balance is 0 (never-funded) by default.
	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, callErr := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet9999",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if callErr == nil {
		t.Fatal("expected insufficient_funds error")
	}
	te, ok := errors.AsType[*server.ToolError](callErr)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", callErr, callErr)
	}
	if te.Code != "insufficient_funds" {
		t.Errorf("expected code=insufficient_funds, got %q", te.Code)
	}
	if te.Extra["address"] != agentAddr {
		t.Errorf("Extra[address]=%v, want %q", te.Extra["address"], agentAddr)
	}
}

// TestCall_agentTestnet_funded verifies that a funded testnet agent account
// proceeds past the balance check and reaches the chain call.
func TestCall_agentTestnet_funded(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	// Generate an agent key and capture its address.
	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000) // funded
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		TxHash:  "0xfunded",
		Height:  1,
		GasUsed: 3000,
	})

	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	res, callErr := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet9999",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	if callErr != nil {
		t.Fatalf("expected success for funded account, got: %v", callErr)
	}
	if !strings.Contains(res.Text, "0xfunded") {
		t.Errorf("expected tx hash in result text:\n%s", res.Text)
	}
}

// TestCall_agentTestnet_simulate_skipsBalanceCheck verifies that simulate=true
// bypasses the balance pre-check (dry-runs should not require funds).
func TestCall_agentTestnet_simulate_skipsBalanceCheck(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	if _, err := ks.GenerateForProfile("testnet9999", testnet9999Profile()); err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake()
	// balance stays 0 — simulate must not be blocked
	fake.SetCall("gno.land/r/test/counter", "Increment", []string{}, chain.CallResult{
		GasUsed:   1000,
		Simulated: false,
	})

	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	res, callErr := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "testnet9999",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"simulate": true,
	})
	if callErr != nil {
		t.Fatalf("simulate with zero balance should succeed, got: %v", callErr)
	}
	if res.StructuredContent["simulated"] != true {
		t.Errorf("expected simulated=true")
	}
}

// TestCall_bogusIdentity verifies that an unknown identity value returns an error.
func TestCall_bogusIdentity(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile":  "local",
		"realm":    "gno.land/r/test/counter",
		"func":     "Increment",
		"identity": "bogus",
	})
	if err == nil {
		t.Fatal("expected error for bogus identity")
	}
	if !strings.Contains(err.Error(), "agent") || !strings.Contains(err.Error(), "session") {
		t.Errorf("expected error mentioning agent and session, got: %v", err)
	}
}

package write

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

const testCode = `package main; func main() { println("hello") }`

func TestRun_happyPath(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		TxHash:  "0xrun1",
		Height:  10,
		Output:  "hello\n",
		GasUsed: 4000,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})

	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"identity": "session",
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
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

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
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"identity": "session",
	})
	if err == nil {
		t.Fatal("expected error for missing code")
	}
	if !strings.Contains(err.Error(), "code") {
		t.Errorf("expected 'code' in error: %v", err)
	}
}

func TestRun_authenticationRequired(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	mgr := noSessionMgr(t) // no sessions
	fake := chain.NewFake()
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
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

func TestRun_picksAnySessionWhenNoRealm(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		TxHash:  "0xany",
		Height:  5,
		Output:  "ok\n",
		GasUsed: 1000,
	})

	// Session covers a specific realm — gno_run should still pick it because
	// realm="" is a wildcard (MsgRun scoping is chain-side per AllowPaths).
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})

	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"identity": "session",
	})
	if err != nil {
		t.Fatalf("expected success with any active session: %v", err)
	}
	if res.StructuredContent["tx_hash"] != "0xany" {
		t.Errorf("unexpected tx_hash: %v", res.StructuredContent["tx_hash"])
	}
}

func TestRun_simulateRequiresSession(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	mgr := noSessionMgr(t) // no session — simulate must still error
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
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

func TestRun_simulateWithSession(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		Output:  "simulated\n",
		GasUsed: 500,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"simulate": true,
		"identity": "session",
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
	fake.SetRunAsUserError(testCode, chain.ErrSimulateUnsupported)

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
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

func TestRun_updatesSessionSpend(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		TxHash:  "0xspend",
		GasUsed: 7000, // actual gas — must be IGNORED for spend (chain bills the GasFee)
		Output:  "ok\n",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "100000000ugnot", true)
	})

	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"identity": "session",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	meta := mgr.Get("testnet5", sessionAddr)
	if meta == nil {
		t.Fatal("session not found after run")
	}
	// The chain bills the full GasFee (10M), not GasUsed (7000): 100M - 10M = 90M (guards #5).
	if meta.SpendRemaining != "90000000ugnot" {
		t.Errorf("SpendRemaining: got %s, want 90000000ugnot (deduct GasFee, not GasUsed)", meta.SpendRemaining)
	}
}

func TestRun_writesAuditEntry(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		TxHash:  "0xaudit",
		GasUsed: 2500,
		Output:  "ok\n",
	})

	var sessionAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		sessionAddr = seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})

	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"identity": "session",
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

func TestRun_simulateError_auditsSimErr(t *testing.T) {
	s := newBaseTestServer(t)
	f := chain.NewFake()
	f.SetRunAsUserError(testCode, errors.New("node unavailable"))
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})
	var auditBuf bytes.Buffer
	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(f), audit.NewLog(&auditBuf))

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
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

func TestRun_broadcastError_auditsResult(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)

	fake := chain.NewFake()
	fake.SetRunAsUserError(testCode, errors.New("broadcast failed"))

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})

	RegisterRun(s, keystore.New(t.TempDir(), ""), mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
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

// TestRun_agentIdentity_local verifies that on a local profile with no identity
// arg, gno_run defaults to "agent" and uses c.Run (not c.RunAsUser).
func TestRun_agentIdentity_local(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xagentrun1",
		Height:  5,
		Output:  "hello\n",
		GasUsed: 2000,
	})

	mgr := noSessionMgr(t) // no sessions — agent path must not need one
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "local",
		"code":    testCode,
	})
	if err != nil {
		t.Fatalf("Run (agent): %v", err)
	}
	if !strings.Contains(res.Text, "0xagentrun1") {
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

// TestRun_sessionIdentity_explicit verifies that explicitly passing identity="session"
// on a testnet profile uses c.RunAsUser and includes the signed-by session line.
func TestRun_sessionIdentity_explicit(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{
		TxHash:  "0xsessrun1",
		Height:  8,
		Output:  "hello\n",
		GasUsed: 3500,
	})

	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", []string{"gno.land/r/test/blog"}, "1000000ugnot", true)
	})
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "testnet5",
		"code":     testCode,
		"identity": "session",
	})
	if err != nil {
		t.Fatalf("Run (session): %v", err)
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

// TestRun_defaultAgent_testnet verifies that on a testnet profile with no
// identity arg and no agent key, gno_run defaults to "agent" and returns
// agent_identity_unavailable (not authentication_required).
// Use identity=session to opt into the session path on testnet.
func TestRun_defaultAgent_testnet(t *testing.T) {
	s := newBaseTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "") // no agent key

	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5",
		"code":    testCode,
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

// TestRun_agentTestnet_insufficientFunds verifies that gno_run returns
// insufficient_funds when the agent's testnet account has zero balance.
func TestRun_agentTestnet_insufficientFunds(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake() // balance 0 by default
	mgr := noSessionMgr(t)
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	_, runErr := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet9999",
		"code":    testCode,
	})
	if runErr == nil {
		t.Fatal("expected insufficient_funds error")
	}
	te, ok := errors.AsType[*server.ToolError](runErr)
	if !ok {
		t.Fatalf("expected *server.ToolError, got %T: %v", runErr, runErr)
	}
	if te.Code != "insufficient_funds" {
		t.Errorf("expected code=insufficient_funds, got %q", te.Code)
	}
	if te.Extra["address"] != agentAddr {
		t.Errorf("Extra[address]=%v, want %q", te.Extra["address"], agentAddr)
	}
}

// TestRun_agentTestnet_funded verifies that a funded testnet agent account
// proceeds past the balance check and reaches the chain run.
func TestRun_agentTestnet_funded(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	agentAddr, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	if err != nil {
		t.Fatalf("GenerateForProfile: %v", err)
	}

	fake := chain.NewFake()
	fake.SetBalance(agentAddr, 10_000_000)
	fake.SetRun(testCode, chain.RunResult{
		TxHash:  "0xfundedrun",
		Height:  2,
		GasUsed: 2000,
	})

	mgr := noSessionMgr(t)
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	res, runErr := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet9999",
		"code":    testCode,
	})
	if runErr != nil {
		t.Fatalf("expected success for funded account, got: %v", runErr)
	}
	if !strings.Contains(res.Text, "0xfundedrun") {
		t.Errorf("expected tx hash in result text:\n%s", res.Text)
	}
}

// TestRun_bogusIdentity verifies that an unknown identity value returns an error.
func TestRun_bogusIdentity(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterRun(s, ks, mgr, constChainResolver(fake), alog)

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile":  "local",
		"code":     testCode,
		"identity": "bogus",
	})
	if err == nil {
		t.Fatal("expected error for bogus identity")
	}
	if !strings.Contains(err.Error(), "agent") || !strings.Contains(err.Error(), "session") {
		t.Errorf("expected error mentioning agent and session, got: %v", err)
	}
}

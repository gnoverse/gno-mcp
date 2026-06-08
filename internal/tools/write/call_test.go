package write

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err, "NewKeypair")
	scope := session.Scope{
		AllowPaths: allowPaths,
		AllowRun:   allowRun,
		SpendLimit: spendLimit,
	}
	meta, err := mgr.AddPending(profile, kp, scope, "g1master")
	require.NoError(t, err, "AddPending")
	status := chain.SessionStatus{
		Active:         true,
		AllowPaths:     scope.AllowPaths,
		AllowRun:       scope.AllowRun,
		SpendLimit:     scope.SpendLimit,
		SpendRemaining: scope.SpendLimit,
	}
	require.NoError(t, mgr.MarkActive(profile, meta.SessionAddress, status), "MarkActive")
	return meta.SessionAddress
}

// parseAuditEntries decodes all JSON-line entries from buf.
func parseAuditEntries(t *testing.T, buf *bytes.Buffer) []audit.Entry {
	t.Helper()
	var entries []audit.Entry
	dec := json.NewDecoder(buf)
	for dec.More() {
		var e audit.Entry
		require.NoError(t, dec.Decode(&e), "decode audit entry")
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
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "0xabc")
	assert.Equal(t, "0xabc", res.StructuredContent["tx_hash"])
	assert.Equal(t, false, res.StructuredContent["simulated"])

	// Audit entry written.
	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1)
	assert.Equal(t, "gno_call", entries[0].Tool)
	assert.Equal(t, "ok", entries[0].Result)
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
	require.Error(t, err, "expected error for missing realm")
	assert.Contains(t, err.Error(), "realm")
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
	require.Error(t, err, "expected error for missing func")
	assert.Contains(t, err.Error(), "func")
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
	require.Error(t, err, "expected type error for non-string args element")
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
	require.Error(t, err, "expected authentication_required error")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "authentication_required", te.Code)
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
	require.Error(t, err, "expected scope_mismatch error")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "scope_mismatch", te.Code)
	// available_paths must be present in Extra.
	assert.Contains(t, te.Extra, "available_paths")
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
	require.Error(t, err, "expected authentication_required for simulate without session")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "authentication_required", te.Code)
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
	require.NoError(t, err, "simulate Call")
	assert.Equal(t, true, res.StructuredContent["simulated"])

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1)
	assert.Equal(t, "sim", entries[0].Result)
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
	require.Error(t, err, "expected simulate_unsupported error")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "simulate_unsupported", te.Code)
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
	require.NoError(t, err, "Call")

	meta := mgr.Get("testnet5", sessionAddr)
	require.NotNil(t, meta, "session not found after call")
	// The chain bills the full GasFee (10M), not GasUsed (3000): 100M - 10M = 90M.
	// Guards #5: local spend tracking must match the chain's GasFee accounting.
	assert.Equal(t, "90000000ugnot", meta.SpendRemaining, "deduct GasFee, not GasUsed")
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
	require.NoError(t, err, "Call")

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1)
	e := entries[0]
	assert.Equal(t, "gno_call", e.Tool)
	assert.Equal(t, "testnet5", e.Profile)
	assert.Equal(t, sessionAddr, e.SessionAddress)
	assert.Equal(t, "ok", e.Result)
	assert.GreaterOrEqual(t, e.Duration, int64(0))
	// ArgsSummary should contain the realm.
	assert.Contains(t, e.ArgsSummary, "gno.land/r/test/counter")
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
	require.Error(t, err, "expected simulate error to propagate")
	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1)
	assert.Equal(t, "sim_err", entries[0].Result)
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
	require.Error(t, err)
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "authentication_required", te.Code)
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
	require.Error(t, err, "expected broadcast error to propagate")

	entries := parseAuditEntries(t, &auditBuf)
	require.Len(t, entries, 1, "expected 1 audit entry even on broadcast error")
	assert.Equal(t, "broadcast_err", entries[0].Result)
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
	require.NoError(t, err, "Call (agent)")
	assert.Contains(t, res.Text, "0xagent1")
	assert.Contains(t, res.Text, "Signed by: agent test1 ("+keystore.Test1Address+")")
	assert.Equal(t, "agent", res.StructuredContent["identity"])
	assert.Equal(t, keystore.Test1Address, res.StructuredContent["signer_address"])
	_, hasMaster := res.StructuredContent["master_address"]
	assert.False(t, hasMaster, "expected no master_address for agent identity")
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
	require.NoError(t, err, "Call (session)")
	assert.Contains(t, res.Text, "Signed by: session")
	assert.Equal(t, "session", res.StructuredContent["identity"])
	assert.NotNil(t, res.StructuredContent["master_address"])
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
	require.Error(t, err, "expected agent_identity_unavailable error (default=agent on testnet, no key generated)")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "agent_identity_unavailable", te.Code)
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
	require.Error(t, err, "expected agent_identity_unavailable error")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "agent_identity_unavailable", te.Code)
	assert.Contains(t, te.Message, "gno_key_generate")
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
	require.Error(t, err, "expected authentication_required (session path, no active session)")
	var te *server.ToolError
	require.ErrorAs(t, err, &te)
	assert.Equal(t, "authentication_required", te.Code)
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
	require.NoError(t, err, "GenerateForProfile")

	// Fake balance is 0 (never-funded) by default.
	fake := chain.NewFake()
	mgr := noSessionMgr(t)
	RegisterCall(s, ks, mgr, constChainResolver(fake), alog)

	_, callErr := s.Registry().Call(context.Background(), "gno_call", map[string]any{
		"profile": "testnet9999",
		"realm":   "gno.land/r/test/counter",
		"func":    "Increment",
	})
	require.Error(t, callErr, "expected insufficient_funds error")
	var te *server.ToolError
	require.ErrorAs(t, callErr, &te)
	assert.Equal(t, "insufficient_funds", te.Code)
	assert.Equal(t, agentAddr, te.Extra["address"])
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
	require.NoError(t, err, "GenerateForProfile")

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
	require.NoError(t, callErr, "expected success for funded account")
	assert.Contains(t, res.Text, "0xfunded")
}

// TestCall_agentTestnet_simulate_skipsBalanceCheck verifies that simulate=true
// bypasses the balance pre-check (dry-runs should not require funds).
func TestCall_agentTestnet_simulate_skipsBalanceCheck(t *testing.T) {
	s := newTestnetTestServer(t)
	var auditBuf bytes.Buffer
	alog := audit.NewLog(&auditBuf)
	ks := keystore.New(t.TempDir(), "")

	_, err := ks.GenerateForProfile("testnet9999", testnet9999Profile())
	require.NoError(t, err, "GenerateForProfile")

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
	require.NoError(t, callErr, "simulate with zero balance should succeed")
	assert.Equal(t, true, res.StructuredContent["simulated"])
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
	require.Error(t, err, "expected error for bogus identity")
	assert.True(t, strings.Contains(err.Error(), "agent") && strings.Contains(err.Error(), "session"),
		"expected error mentioning agent and session, got: %v", err)
}

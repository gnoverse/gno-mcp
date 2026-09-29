package write

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

func TestRun_agentOutsideRunSubmittersIsRefusedBeforeSigning(t *testing.T) {
	s := newLocalTestServer(t)
	var auditBuf bytes.Buffer
	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{TxHash: "0xmust-not-run"})
	fake.SetRunSubmitters([]string{"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m"})
	RegisterRun(s, keystore.New(t.TempDir(), "", 5), noSessionMgr(t), constChainResolver(fake), audit.NewLog(&auditBuf))

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{"profile": "local", "code": testCode})

	te, ok := errors.AsType[*server.ToolError](err)
	require.True(t, ok, "want a ToolError, got %T: %v", err, err)
	assert.Equal(t, "run_not_allowed", te.Code)
	assert.Contains(t, te.Message, keystore.Test1Address)
	assert.Contains(t, te.Message, "run_submitters")
	assert.Contains(t, te.Message, "gno_call", "the refusal names the route that still works")
	assert.Equal(t, keystore.Test1Address, te.Extra["address"])
	assert.Equal(t, "tool_err", parseAuditEntries(t, &auditBuf)[0].Result, "a refusal before signing is a denial, not a broadcast error")
}

// Asking an agent to fund a key the chain will refuse anyway spends a faucet
// grant for nothing: the allowlist answers first.
func TestRun_unfundedAgentOutsideRunSubmittersIsRefusedNotSentToTheFaucet(t *testing.T) {
	s := newTestnetTestServer(t)
	ks := keystore.New(t.TempDir(), "", 5)
	_, err := ks.GenerateForProfile("testnet9999", "", testnet9999Profile())
	require.NoError(t, err)
	fake := chain.NewFake() // balance 0
	fake.SetRunSubmitters([]string{"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m"})
	RegisterRun(s, ks, noSessionMgr(t), constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	_, err = s.Registry().Call(context.Background(), "gno_run", map[string]any{"profile": "testnet9999", "code": testCode})

	te, ok := errors.AsType[*server.ToolError](err)
	require.True(t, ok, "want a ToolError, got %T: %v", err, err)
	assert.Equal(t, "run_not_allowed", te.Code)
}

func TestRun_listedAgentRuns(t *testing.T) {
	s := newLocalTestServer(t)
	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{TxHash: "0xran"})
	fake.SetRunSubmitters([]string{"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m", keystore.Test1Address})
	RegisterRun(s, keystore.New(t.TempDir(), "", 5), noSessionMgr(t), constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	res, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{"profile": "local", "code": testCode})

	require.NoError(t, err)
	assert.Contains(t, res.Text, "0xran")
}

// A session-signed MsgRun names the master as its caller, so the chain checks
// the master against the allowlist.
func TestRun_sessionWhoseMasterIsNotARunSubmitterIsRefused(t *testing.T) {
	s := newBaseTestServer(t)
	fake := chain.NewFake()
	fake.SetRunAsUser(testCode, chain.RunResult{TxHash: "0xmust-not-run"})
	fake.SetRunSubmitters([]string{"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m"})
	mgr := constSessionMgr(t, func(m *session.Manager) {
		seedActiveSessionWithRun(t, m, "testnet5", nil, "1000000ugnot", true)
	})
	RegisterRun(s, keystore.New(t.TempDir(), "", 5), mgr, constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{
		"profile": "testnet5", "code": testCode, "identity": "session",
	})

	te, ok := errors.AsType[*server.ToolError](err)
	require.True(t, ok, "want a ToolError, got %T: %v", err, err)
	assert.Equal(t, "run_not_allowed", te.Code)
	assert.Equal(t, "g1master", te.Extra["address"])
}

func TestRun_runSubmittersReadFailureStopsBeforeSigning(t *testing.T) {
	s := newLocalTestServer(t)
	fake := chain.NewFake()
	fake.SetRun(testCode, chain.RunResult{TxHash: "0xmust-not-run"})
	fake.SetRunSubmittersErr(errors.New("rpc down"))
	RegisterRun(s, keystore.New(t.TempDir(), "", 5), noSessionMgr(t), constChainResolver(fake), audit.NewLog(&bytes.Buffer{}))

	_, err := s.Registry().Call(context.Background(), "gno_run", map[string]any{"profile": "local", "code": testCode})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "run_submitters")
	assert.NotContains(t, err.Error(), "0xmust-not-run")
}

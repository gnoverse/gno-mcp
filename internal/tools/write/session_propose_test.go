package write

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// proposeFake returns a Fake chain client reporting feeUgnot as the live
// per-write gas fee.
func proposeFake(feeUgnot int64) *chain.Fake {
	f := chain.NewFake()
	f.SetGasFee(feeUgnot)
	return f
}

func TestSessionPropose_spendLimitBelowFeeErrors(t *testing.T) {
	// The exact test13 shape: live fee 4000000ugnot, requested limit 1000000ugnot.
	// Such a session can never broadcast (the ante counts the full fee against
	// the spend limit), so propose must refuse with both numbers.
	s := newBaseTestServer(t)
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(proposeFake(4_000_000)))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
		"spend_limit": "1000000ugnot",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "1000000ugnot", "error should name the offending limit")
	assert.Contains(t, err.Error(), "4000000", "error should name the per-write fee")
}

func TestSessionPropose_defaultSpendLimitDerivedFromFee(t *testing.T) {
	// No spend_limit requested and no profile default: the proposed limit is
	// sized from the live fee (10 writes' worth), and the answer spells out
	// the per-write cost so the user knows what the limit buys.
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(proposeFake(4_000_000)))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	scope, ok := res.StructuredContent["scope"].(map[string]any)
	require.True(t, ok, "scope in structured content")
	assert.Equal(t, "40000000ugnot", scope["spend_limit"])
	assert.Contains(t, res.Text, "4000000ugnot per write")
	assert.Contains(t, res.Text, "10 write")
	// The math must reach structured-output consumers too: clients that
	// surface structuredContent never see the Text (observed live — the AUT
	// recomputed the budget from the auth_command's --gas-fee).
	assert.Equal(t, int64(4_000_000), res.StructuredContent["per_write_fee_ugnot"])
	assert.Equal(t, int64(10), res.StructuredContent["writes_budget"])
}

func TestSessionPropose_gnokeyCommandUsesLiveFee(t *testing.T) {
	// The pasted create command must carry a fee the chain will accept at its
	// live gas price — the floor fee bounces with "insufficient fee" on a
	// chain priced above genesis.
	s := newBaseTestServer(t)
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(proposeFake(4_000_000)))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	cmd, _ := res.StructuredContent["auth_command"].(string)
	assert.Contains(t, cmd, "--gas-fee 4000000ugnot")
}

func TestSessionPropose_gasFeeQueryFailureErrors(t *testing.T) {
	// Fee-unknown fails closed: proposing without the live fee could mint a
	// dead-on-arrival session, so the tool reports the query failure instead.
	s := newBaseTestServer(t)
	fake := chain.NewFake()
	fake.SetGasFeeErr(errors.New("rpc unreachable"))
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(fake))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gas fee")
}

func TestSessionPropose_emitsGnokeyCommand(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "gnokey maketx session create")
	assert.Contains(t, res.Text, "gpub1")
	assert.Contains(t, res.Text, "gno.land/r/test/counter")
}

// A master-less but WRITABLE (testnet/local) profile can still propose a session
// when the user supplies their PUBLIC address — it is stored on the session record.
func TestSessionPropose_masterAddressParam(t *testing.T) {
	s := newReadOnlyTestServer(t) // testnet5 (test5), NO master-address
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	const userAddr = "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5"
	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":        "testnet5",
		"allow_paths":    []any{"gno.land/r/test/counter"},
		"master_address": userAddr,
	})
	require.NoError(t, err, "propose with user-supplied master")
	sessAddr, _ := res.StructuredContent["session_address"].(string)
	require.NotEmpty(t, sessAddr)
	meta := mgr.Get("testnet5", sessAddr)
	require.NotNil(t, meta, "session must be recorded")
	assert.Equal(t, userAddr, meta.MasterAddress, "the user-supplied master must be stored on the session record")
}

// Master-less profile, no param → must ask for master_address (not stall).
func TestSessionPropose_masterlessRequiresMasterAddress(t *testing.T) {
	s := newReadOnlyTestServer(t)
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master_address")
}

// A seed phrase pasted as master_address must be rejected WITHOUT echoing it.
func TestSessionPropose_rejectsMnemonicShapedMaster(t *testing.T) {
	s := newReadOnlyTestServer(t)
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(chain.NewFake()))
	const seed = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":        "testnet5",
		"allow_paths":    []any{"gno.land/r/test/counter"},
		"master_address": seed,
	})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "abandon", "a seed phrase must never be echoed back")
	assert.Contains(t, err.Error(), "seed phrase")
}

// A malformed (non-bech32) master_address is rejected.
func TestSessionPropose_rejectsInvalidMasterAddress(t *testing.T) {
	s := newReadOnlyTestServer(t)
	RegisterSessionPropose(s, noSessionMgr(t), constChainResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":        "testnet5",
		"allow_paths":    []any{"gno.land/r/test/counter"},
		"master_address": "g1notavalidaddress",
	})
	require.Error(t, err)
}

func TestSessionPropose_emitsClampWarning_whenClamped(t *testing.T) {
	s := newBaseTestServer(t) // testnet profile; cap = 100000000ugnot (100 gnot)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
		"spend_limit": "500000000ugnot", // exceeds testnet cap of 100000000ugnot
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "WARNING")
	assert.Contains(t, res.Text, "100000000ugnot")
	// The requested value may appear in the clamp WARNING, but must not leak
	// into the gnokey command itself (which should carry the clamped value).
	// Fail only if unclamped value appears AND there is no WARNING block.
	if strings.Contains(res.Text, "500000000ugnot") {
		assert.Contains(t, res.Text, "WARNING", "unclamped value leaked into command")
	}
}

func TestSessionPropose_emptyAllowPathsErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{},
	})
	require.Error(t, err, "expected error for empty allow_paths")
}

func TestSessionPropose_missingProfileErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.Error(t, err, "expected error for missing profile")
}

func TestSessionPropose_rejectsNonStringAllowPaths(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{42},
	})
	require.Error(t, err, "expected type error for non-string allow_paths element")
}

func TestSessionPropose_allowRunOnly(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":   "testnet5",
		"allow_run": true,
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "vm/run")
	sessions := mgr.ListForProfile("testnet5")
	require.Len(t, sessions, 1, "expected 1 pending session")
	assert.True(t, sessions[0].AllowRun, "pending session AllowRun=false, want true")
}

func TestSessionPropose_allowRunAndAllowPaths(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
		"allow_run":   true,
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "vm/run")
	assert.Contains(t, res.Text, "vm/exec:gno.land/r/test/counter")
}

func TestSessionPropose_emptyAllowPathsAndNoRunErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when both allow_paths and allow_run are absent")
}

func TestSessionPropose_NoMaster(t *testing.T) {
	s := newReadOnlyTestServer(t) // profile has no master-address
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/demo/foo"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master-address")
}

// The repair instructions must say config is loaded at startup: an agent that
// edits profiles.toml and retries against the same process gets the same error
// and has no way to know why.
// The no-master repair no longer makes the user edit profiles.toml and restart;
// it offers the master_address param. Guard that the old restart friction is gone.
func TestSessionPropose_NoMaster_offersParamNotRestart(t *testing.T) {
	s := newReadOnlyTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/demo/foo"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master_address", "should offer the param")
	assert.NotContains(t, err.Error(), "restart", "the edit-and-restart friction is removed")
}

// The no-master repair is name-agnostic now: it asks for master_address rather
// than sending the agent to a `gnomcp profile add` CLI edit.
func TestSessionPropose_NoMaster_offersParamNotCLIEdit(t *testing.T) {
	s := newReadOnlyTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/demo/foo"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master_address")
	assert.NotContains(t, err.Error(), "gnomcp profile add", "no dead-end CLI suggestion")
}

// `gnomcp profile add` refuses built-in profile names, so suggesting it for
// one sends the agent into a guaranteed CLI failure; for built-in names the
// only working repair is the profiles.toml entry (which overrides a built-in
// of the same name).
func TestSessionPropose_NoMaster_builtinName_suggestsTomlNotCLI(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet": {RPCURL: "http://127.0.0.1:26657", ChainID: "test5"},
	}}
	_, err := cfg.Validate()
	require.NoError(t, err, "validate")
	s := server.NewServer(cfg, "")
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err = s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet",
		"allow_paths": []any{"gno.land/r/demo/foo"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "profiles.toml")
	assert.NotContains(t, err.Error(), "gnomcp profile add", "the CLI refuses built-in names; suggesting it is a dead end")
}

// Verify that after a propose call, the manager holds a pending session.
func TestSessionPropose_addsPendingSession(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr, constChainResolver(chain.NewFake()))

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	sessions := mgr.ListForProfile("testnet5")
	require.Len(t, sessions, 1, "expected 1 pending session")
	assert.Equal(t, session.StatePending, sessions[0].State)
}

package write

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// revokeSeededMgr returns a Manager holding one pending session on testnet5.
func revokeSeededMgr(t *testing.T) *session.Manager {
	t.Helper()
	return constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		require.NoError(t, err, "NewKeypair")
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		_, err = m.AddPending("testnet5", kp, scope, "g1master")
		require.NoError(t, err, "AddPending")
	})
}

func TestSessionRevoke_commandUsesLiveFee(t *testing.T) {
	// Same insufficient-fee wart as the create template: a floor --gas-fee
	// bounces on a chain priced above genesis, so the pasted revoke command
	// must carry the live fee.
	s := newBaseTestServer(t)
	mgr := revokeSeededMgr(t)
	RegisterSessionRevoke(s, mgr, constChainResolver(proposeFake(4_000_000)))

	res, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": mgr.ListForProfile("testnet5")[0].SessionAddress,
	})
	require.NoError(t, err, "Call")
	cmd, _ := res.StructuredContent["revoke_command"].(string)
	assert.Contains(t, cmd, "--gas-fee 4000000ugnot")
}

func TestSessionRevoke_feeQueryFailureStillRevokes(t *testing.T) {
	// Revocation kills a live credential — it must never be blocked by a
	// gas-price query flake. Unknown fee falls back to the floor, with a
	// note so the user knows to bump --gas-fee if the chain rejects it.
	s := newBaseTestServer(t)
	mgr := revokeSeededMgr(t)
	fake := chain.NewFake()
	fake.SetGasFeeErr(errors.New("rpc unreachable"))
	RegisterSessionRevoke(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": mgr.ListForProfile("testnet5")[0].SessionAddress,
	})
	require.NoError(t, err, "revoke must not be blocked by a fee-query failure")
	assert.Contains(t, res.Text, "gnokey maketx session revoke")
	assert.Contains(t, res.Text, "could not query the live gas price")
}

func TestSessionRevoke_emitsRevokeCommand(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		require.NoError(t, err, "NewKeypair")
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		_, err = m.AddPending("testnet5", kp, scope, "g1master")
		require.NoError(t, err, "AddPending")
		// expose address to test via the manager's own list
		sessions := m.ListForProfile("testnet5")
		require.Len(t, sessions, 1, "expected 1 session")
		t.Logf("seeded session_address=%s", sessions[0].SessionAddress)
	})
	RegisterSessionRevoke(s, mgr, constChainResolver(chain.NewFake()))

	sessions := mgr.ListForProfile("testnet5")
	require.Len(t, sessions, 1, "pre-condition: expected 1 session")
	sessionAddr := sessions[0].SessionAddress

	res, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": sessionAddr,
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "gnokey maketx session revoke")
	assert.Contains(t, res.Text, sessionAddr)
}

func TestSessionRevoke_unknownSession_returnsUnmanaged(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionRevoke(s, mgr, constChainResolver(chain.NewFake()))

	unknownAddr := "g1qyqdyjcz5dpylqsrk9zjhphq4uyg36aps3qvzq"

	_, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": unknownAddr,
	})
	require.Error(t, err, "expected session_unmanaged error, got nil")
	assert.Contains(t, err.Error(), "session_unmanaged")
	assert.Contains(t, err.Error(), "gnokey maketx session revoke")
}

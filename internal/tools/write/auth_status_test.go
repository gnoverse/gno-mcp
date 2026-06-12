package write

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/session"
)

func TestAuthStatus_noSessions_suggestsPropose(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "no active session")
	assert.Contains(t, res.Text, "gno_session_propose")
	assert.Equal(t, true, res.StructuredContent["master_address_set"])
}

// A master-less profile must say so here: gno_auth_status is the natural first
// status check, and an empty-sessions success with no read-only signal leaves
// the caller planning a session flow that gno_session_propose will refuse.
func TestAuthStatus_noMaster_signalsReadOnly(t *testing.T) {
	s := newReadOnlyTestServer(t)
	mgr := noSessionMgr(t)
	RegisterAuthStatus(s, mgr, constChainResolver(chain.NewFake()))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "no master-address")
	assert.NotContains(t, res.Text, "gno_session_propose", "proposing would fail on a master-less profile — don't suggest it")
	assert.Equal(t, false, res.StructuredContent["master_address_set"])
}

func TestAuthStatus_activeSession_narrative(t *testing.T) {
	s := newBaseTestServer(t)
	var seededAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		require.NoError(t, err, "NewKeypair")
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		meta, err := m.AddPending("testnet5", kp, scope, "g1master")
		require.NoError(t, err, "AddPending")
		seededAddr = meta.SessionAddress
		// Transition to active in the manager.
		status := chain.SessionStatus{
			Active:         true,
			AllowPaths:     scope.AllowPaths,
			SpendLimit:     scope.SpendLimit,
			SpendRemaining: scope.SpendLimit,
		}
		require.NoError(t, m.MarkActive("testnet5", seededAddr, status), "MarkActive")
	})
	fake := chain.NewFake()
	// Chain confirms the session is still active (otherwise auth_status would
	// correctly downgrade it — see TestAuthStatus_detectsRevocation).
	fake.SetSession("g1master", seededAddr, chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/counter"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "1000000ugnot",
	})
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "[active]")
	assert.True(t, strings.Contains(res.Text, seededAddr), "expected session address in narrative")
	assert.Contains(t, res.Text, "gno.land/r/test/counter")
}

// TestAuthStatus_chainQueryRefreshes verifies that gno_auth_status flips a
// pending session to active when the chain confirms it.
func TestAuthStatus_chainQueryRefreshes(t *testing.T) {
	s := newBaseTestServer(t)
	const master = "g1master"
	var seededAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		require.NoError(t, err, "NewKeypair")
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/blog"},
			SpendLimit: "500000ugnot",
		}
		meta, err := m.AddPending("testnet5", kp, scope, master)
		require.NoError(t, err, "AddPending")
		seededAddr = meta.SessionAddress
	})

	fake := chain.NewFake()
	fake.SetSession(master, seededAddr, chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "500000ugnot",
		SpendRemaining: "500000ugnot",
	})
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "[active]", "expected '[active]' after chain refresh")
	assert.True(t, strings.Contains(res.Text, seededAddr), "expected session address in narrative")
	// Manager should now reflect active state.
	updated := mgr.Get("testnet5", seededAddr)
	require.NotNil(t, updated, "session not found after refresh")
	assert.Equal(t, session.StateActive, updated.State, "expected manager state active after chain refresh")
}

// TestAuthStatus_detectsRevocation verifies that a session which is active
// locally but which the chain no longer reports active (revoked on chain) is
// downgraded to [revoked] — not left showing [active].
func TestAuthStatus_detectsRevocation(t *testing.T) {
	s := newBaseTestServer(t)
	const master = "g1master"
	var seededAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		require.NoError(t, err, "NewKeypair")
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		meta, err := m.AddPending("testnet5", kp, scope, master)
		require.NoError(t, err, "AddPending")
		seededAddr = meta.SessionAddress
		require.NoError(t, m.MarkActive("testnet5", seededAddr, chain.SessionStatus{
			Active: true, AllowPaths: scope.AllowPaths, SpendLimit: scope.SpendLimit, SpendRemaining: scope.SpendLimit,
		}), "MarkActive")
	})

	// Fake chain has NO record of the session → QuerySession reports !Active,
	// as it would after an on-chain revoke.
	fake := chain.NewFake()
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "[revoked]", "expected '[revoked]' after chain reports inactive")
	assert.NotContains(t, res.Text, "[active]", "revoked session must not still show '[active]'")
	updated := mgr.Get("testnet5", seededAddr)
	require.NotNil(t, updated)
	assert.Equal(t, session.StateRevoked, updated.State, "expected manager state revoked")
}

func TestAuthStatus_missingProfileErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	_, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{})
	require.Error(t, err, "expected error for missing profile")
}

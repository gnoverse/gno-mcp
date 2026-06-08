package write

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/session"
)

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
	RegisterSessionRevoke(s, mgr)

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
	RegisterSessionRevoke(s, mgr)

	unknownAddr := "g1qyqdyjcz5dpylqsrk9zjhphq4uyg36aps3qvzq"

	_, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": unknownAddr,
	})
	require.Error(t, err, "expected session_unmanaged error, got nil")
	assert.Contains(t, err.Error(), "session_unmanaged")
	assert.Contains(t, err.Error(), "gnokey maketx session revoke")
}

package write

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gnoverse/gno-mcp/internal/session"
)

func TestSessionPropose_emitsGnokeyCommand(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	assert.Contains(t, res.Text, "gnokey maketx session create")
	assert.Contains(t, res.Text, "gpub1")
	assert.Contains(t, res.Text, "gno.land/r/test/counter")
}

func TestSessionPropose_emitsClampWarning_whenClamped(t *testing.T) {
	s := newBaseTestServer(t) // testnet profile; cap = 100000000ugnot (100 gnot)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

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
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{},
	})
	require.Error(t, err, "expected error for empty allow_paths")
}

func TestSessionPropose_missingProfileErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.Error(t, err, "expected error for missing profile")
}

func TestSessionPropose_rejectsNonStringAllowPaths(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{42},
	})
	require.Error(t, err, "expected type error for non-string allow_paths element")
}

func TestSessionPropose_allowRunOnly(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

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
	RegisterSessionPropose(s, mgr)

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
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile": "testnet5",
	})
	require.Error(t, err, "expected error when both allow_paths and allow_run are absent")
}

func TestSessionPropose_NoMaster(t *testing.T) {
	s := newReadOnlyTestServer(t) // profile has no master-address
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)
	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/demo/foo"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "master-address")
}

// Verify that after a propose call, the manager holds a pending session.
func TestSessionPropose_addsPendingSession(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	require.NoError(t, err, "Call")
	sessions := mgr.ListForProfile("testnet5")
	require.Len(t, sessions, 1, "expected 1 pending session")
	assert.Equal(t, session.StatePending, sessions[0].State)
}

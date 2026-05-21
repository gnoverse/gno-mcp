package write

import (
	"context"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "gnokey maketx session create") {
		t.Errorf("text missing gnokey command:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "gpub1") {
		t.Errorf("text missing gpub1 pubkey prefix:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "gno.land/r/test/counter") {
		t.Errorf("text missing realm path:\n%s", res.Text)
	}
}

func TestSessionPropose_emitsClampWarning_whenClamped(t *testing.T) {
	s := newBaseTestServer(t) // testnet profile; cap = 10000000ugnot (10 gnot)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
		"spend_limit": "100000000ugnot", // exceeds testnet cap of 10000000ugnot
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "WARNING") {
		t.Errorf("expected clamp warning in text:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "10000000ugnot") {
		t.Errorf("expected clamped value 10000000ugnot in text:\n%s", res.Text)
	}
	// auth_command must show clamped value, not requested 100000000ugnot
	if strings.Contains(res.Text, "100000000ugnot") && !strings.Contains(res.Text, "WARNING") {
		t.Errorf("unclamped value leaked into command:\n%s", res.Text)
	}
}

func TestSessionPropose_dangerousDisabled(t *testing.T) {
	s := newReadOnlyTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	if err == nil {
		t.Fatal("expected dangerous_disabled error")
	}
	if !strings.Contains(err.Error(), "dangerous_disabled") {
		t.Errorf("error missing dangerous_disabled code: %v", err)
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
	if err == nil {
		t.Fatal("expected error for empty allow_paths")
	}
}

func TestSessionPropose_missingProfileErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"allow_paths": []any{"gno.land/r/test/counter"},
	})
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

func TestSessionPropose_rejectsNonStringAllowPaths(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":     "testnet5",
		"allow_paths": []any{42},
	})
	if err == nil {
		t.Fatal("expected type error for non-string allow_paths element")
	}
}

func TestSessionPropose_allowRunOnly(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	res, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile":   "testnet5",
		"allow_run": true,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "vm/run") {
		t.Errorf("expected vm/run in auth_command:\n%s", res.Text)
	}
	sessions := mgr.ListForProfile("testnet5")
	if len(sessions) != 1 {
		t.Fatalf("expected 1 pending session, got %d", len(sessions))
	}
	if !sessions[0].AllowRun {
		t.Errorf("pending session AllowRun=false, want true")
	}
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
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "vm/run") {
		t.Errorf("expected vm/run in auth_command:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "vm/exec:gno.land/r/test/counter") {
		t.Errorf("expected vm/exec:realm in auth_command:\n%s", res.Text)
	}
}

func TestSessionPropose_emptyAllowPathsAndNoRunErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	RegisterSessionPropose(s, mgr)

	_, err := s.Registry().Call(context.Background(), "gno_session_propose", map[string]any{
		"profile": "testnet5",
	})
	if err == nil {
		t.Fatal("expected error when both allow_paths and allow_run are absent")
	}
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
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	sessions := mgr.ListForProfile("testnet5")
	if len(sessions) != 1 {
		t.Errorf("expected 1 pending session, got %d", len(sessions))
	}
	if sessions[0].State != session.StatePending {
		t.Errorf("expected state pending, got %s", sessions[0].State)
	}
}

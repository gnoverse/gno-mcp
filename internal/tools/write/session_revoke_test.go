package write

import (
	"context"
	"strings"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/session"
)

func TestSessionRevoke_emitsRevokeCommand(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		if err != nil {
			t.Fatalf("NewKeypair: %v", err)
		}
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		if _, err := m.AddPending("testnet5", kp, scope, "g1master"); err != nil {
			t.Fatalf("AddPending: %v", err)
		}
		// expose address to test via the manager's own list
		sessions := m.ListForProfile("testnet5")
		if len(sessions) != 1 {
			t.Fatalf("expected 1 session, got %d", len(sessions))
		}
		t.Logf("seeded session_address=%s", sessions[0].SessionAddress)
	})
	RegisterSessionRevoke(s, mgr)

	sessions := mgr.ListForProfile("testnet5")
	if len(sessions) != 1 {
		t.Fatalf("pre-condition: expected 1 session, got %d", len(sessions))
	}
	sessionAddr := sessions[0].SessionAddress

	res, err := s.Registry().Call(context.Background(), "gno_session_revoke", map[string]any{
		"profile":         "testnet5",
		"session_address": sessionAddr,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "gnokey maketx session revoke") {
		t.Errorf("text missing gnokey revoke command:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, sessionAddr) {
		t.Errorf("text missing session_address %q:\n%s", sessionAddr, res.Text)
	}
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
	if err == nil {
		t.Fatal("expected session_unmanaged error, got nil")
	}
	if !strings.Contains(err.Error(), "session_unmanaged") {
		t.Errorf("error missing session_unmanaged code: %v", err)
	}
	if !strings.Contains(err.Error(), "gnokey maketx session revoke") {
		t.Errorf("error missing manual gnokey hint: %v", err)
	}
}

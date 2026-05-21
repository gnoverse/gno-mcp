package write

import (
	"context"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "no active session") {
		t.Errorf("expected 'no active session' in narrative:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, "gno_session_propose") {
		t.Errorf("expected 'gno_session_propose' suggestion in narrative:\n%s", res.Text)
	}
}

func TestAuthStatus_activeSession_narrative(t *testing.T) {
	s := newBaseTestServer(t)
	var seededAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		if err != nil {
			t.Fatalf("NewKeypair: %v", err)
		}
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/counter"},
			SpendLimit: "1000000ugnot",
		}
		meta, err := m.AddPending("testnet5", kp, scope, "g1master")
		if err != nil {
			t.Fatalf("AddPending: %v", err)
		}
		seededAddr = meta.SessionAddress
		// Transition to active in the manager.
		status := chain.SessionStatus{
			Active:         true,
			AllowPaths:     scope.AllowPaths,
			SpendLimit:     scope.SpendLimit,
			SpendRemaining: scope.SpendLimit,
		}
		if err := m.MarkActive("testnet5", seededAddr, status); err != nil {
			t.Fatalf("MarkActive: %v", err)
		}
	})
	fake := chain.NewFake()
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	res, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{
		"profile": "testnet5",
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "[active]") {
		t.Errorf("expected '[active]' label in narrative:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, seededAddr) {
		t.Errorf("expected session address %q in narrative:\n%s", seededAddr, res.Text)
	}
	if !strings.Contains(res.Text, "gno.land/r/test/counter") {
		t.Errorf("expected realm path in narrative:\n%s", res.Text)
	}
}

// TestAuthStatus_chainQueryRefreshes verifies that gno_auth_status flips a
// pending session to active when the chain confirms it.
func TestAuthStatus_chainQueryRefreshes(t *testing.T) {
	s := newBaseTestServer(t)
	const master = "g1master"
	var seededAddr string
	mgr := constSessionMgr(t, func(m *session.Manager) {
		kp, err := session.NewKeypair()
		if err != nil {
			t.Fatalf("NewKeypair: %v", err)
		}
		scope := session.Scope{
			AllowPaths: []string{"gno.land/r/test/blog"},
			SpendLimit: "500000ugnot",
		}
		meta, err := m.AddPending("testnet5", kp, scope, master)
		if err != nil {
			t.Fatalf("AddPending: %v", err)
		}
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
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(res.Text, "[active]") {
		t.Errorf("expected '[active]' after chain refresh:\n%s", res.Text)
	}
	if !strings.Contains(res.Text, seededAddr) {
		t.Errorf("expected session address %q in narrative:\n%s", seededAddr, res.Text)
	}
	// Manager should now reflect active state.
	updated := mgr.Get("testnet5", seededAddr)
	if updated == nil {
		t.Fatal("session not found after refresh")
	}
	if updated.State != session.StateActive {
		t.Errorf("expected manager state active after chain refresh, got %s", updated.State)
	}
}

func TestAuthStatus_missingProfileErrors(t *testing.T) {
	s := newBaseTestServer(t)
	mgr := noSessionMgr(t)
	fake := chain.NewFake()
	RegisterAuthStatus(s, mgr, constChainResolver(fake))

	_, err := s.Registry().Call(context.Background(), "gno_auth_status", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing profile")
	}
}

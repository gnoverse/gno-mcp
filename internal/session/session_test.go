package session

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestNew_Unauthenticated(t *testing.T) {
	m := New(Options{Network: "staging.gno.land"})
	if m.State() != StateUnauthenticated {
		t.Errorf("state = %s, want unauthenticated", m.State())
	}
	if m.Address() != "" {
		t.Error("address must be empty before EnsurePending")
	}
	if m.Signer() != "" {
		t.Error("Signer must be empty while unauthenticated")
	}
}

func TestEnsurePending_GeneratesKeyAndAddress(t *testing.T) {
	m := New(Options{Network: "staging.gno.land"})
	if err := m.EnsurePending(); err != nil {
		t.Fatal(err)
	}
	if m.State() != StatePending {
		t.Errorf("state = %s, want pending", m.State())
	}
	addr := m.Address()
	if addr == "" {
		t.Fatal("address must be set")
	}
	if !strings.HasPrefix(addr, "gmcp1") {
		t.Errorf("address must start with gmcp1, got %s", addr)
	}
}

func TestEnsurePending_Idempotent(t *testing.T) {
	m := New(Options{})
	_ = m.EnsurePending()
	addr1 := m.Address()
	_ = m.EnsurePending()
	if addr1 != m.Address() {
		t.Error("second EnsurePending must not regenerate keypair")
	}
}

func TestRefresh_FundingFlipsToAuthenticated(t *testing.T) {
	m := New(Options{
		Network: "staging.gno.land",
		Balance: func(_ context.Context, _, _ string) (int64, error) {
			return FundThreshold, nil
		},
	})
	_ = m.EnsurePending()
	if err := m.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.State() != StateAuthenticated {
		t.Errorf("state = %s, want authenticated", m.State())
	}
	if m.Signer() != "mcp-session" {
		t.Errorf("Signer = %q, want mcp-session", m.Signer())
	}
}

func TestRefresh_BelowThresholdStaysPending(t *testing.T) {
	m := New(Options{
		Balance: func(_ context.Context, _, _ string) (int64, error) {
			return FundThreshold - 1, nil
		},
	})
	_ = m.EnsurePending()
	_ = m.Refresh(context.Background())
	if m.State() != StatePending {
		t.Errorf("state = %s, want pending", m.State())
	}
}

func TestRefresh_HysteresisOnExpiry(t *testing.T) {
	var bal int64 = FundThreshold * 2
	m := New(Options{
		Balance: func(_ context.Context, _, _ string) (int64, error) { return bal, nil },
	})
	_ = m.EnsurePending()
	_ = m.Refresh(context.Background()) // authenticated
	if m.State() != StateAuthenticated {
		t.Fatalf("expected authenticated, got %s", m.State())
	}
	// Drop to just under threshold — must NOT flip to expired (hysteresis).
	bal = FundThreshold - 1
	_ = m.Refresh(context.Background())
	if m.State() != StateAuthenticated {
		t.Errorf("hysteresis broken: dropped to expired at bal=%d (threshold=%d)", bal, FundThreshold)
	}
	// Drop below half — now expire.
	bal = FundThreshold/2 - 1
	_ = m.Refresh(context.Background())
	if m.State() != StateExpired {
		t.Errorf("state = %s, want expired", m.State())
	}
}

func TestRefresh_FetcherErrorDoesNotChangeState(t *testing.T) {
	m := New(Options{
		Balance: func(_ context.Context, _, _ string) (int64, error) {
			return 0, errors.New("transient RPC failure")
		},
	})
	_ = m.EnsurePending()
	_ = m.SetAuthenticated(FundThreshold * 10)
	_ = m.Refresh(context.Background())
	if m.State() != StateAuthenticated {
		t.Errorf("transient error must not deauthenticate: state=%s", m.State())
	}
}

func TestBuildAuthPayload_IncludesQR(t *testing.T) {
	m := New(Options{Network: "staging.gno.land"})
	_ = m.EnsurePending()
	pl := BuildAuthPayload(m.Status(), "demo-memo")
	if pl.Address == "" || pl.FundURL == "" || pl.QRASCII == "" {
		t.Errorf("payload missing fields: %+v", pl)
	}
	if !strings.Contains(pl.FundURL, "gnoland://send") {
		t.Errorf("fund URL shape: %s", pl.FundURL)
	}
	if !strings.Contains(pl.FundURL, "memo=demo-memo") {
		t.Errorf("memo not propagated: %s", pl.FundURL)
	}
}

func TestSetAuthenticated_RequiresPending(t *testing.T) {
	m := New(Options{})
	if err := m.SetAuthenticated(FundThreshold); err == nil {
		t.Error("SetAuthenticated must reject unauthenticated state")
	}
}

package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(t.TempDir(), "")
}

func activeSession(addr, pubkey string, allowPaths []string, createdAt int64) *SessionMeta {
	return &SessionMeta{
		Version:        1,
		SessionAddress: addr,
		SessionPubkey:  pubkey,
		Privkey:        make([]byte, 64),
		AllowPaths:     allowPaths,
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "1000000ugnot",
		ExpiresAt:      time.Now().Add(24 * time.Hour).Unix(),
		CreatedAt:      createdAt,
	}
}

func resolverFor(fake *chain.Fake) chain.Resolver {
	return func(profile string) chain.Client { return fake }
}

func nullResolver() chain.Resolver {
	return func(_ string) chain.Client { return chain.NewFake() }
}

func TestManager_addPendingPersists(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	scope := Scope{
		SpendLimit:  "500000ugnot",
		SpendPeriod: time.Hour,
		ExpiresIn:   time.Hour,
		AllowPaths:  []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("testnet", kp, scope)
	if err != nil {
		t.Fatalf("AddPending: %v", err)
	}
	got, err := m.store.Read("testnet", meta.SessionAddress)
	if err != nil {
		t.Fatalf("store.Read after AddPending: %v", err)
	}
	if got.SessionAddress != meta.SessionAddress {
		t.Errorf("stored addr = %q, want %q", got.SessionAddress, meta.SessionAddress)
	}
}

func TestManager_pickPicksMostRecentMatching(t *testing.T) {
	m := newTestManager(t)
	blog := []string{"gno.land/r/test/blog"}
	other := []string{"gno.land/r/test/other"}
	older := activeSession("g1older", "gpub1older", blog, 1000)
	newer := activeSession("g1newer", "gpub1newer", blog, 2000)
	unrelated := activeSession("g1other", "gpub1other", other, 3000)
	m.mu.Lock()
	m.insertStateLocked("p", older, StateActive)
	m.insertStateLocked("p", newer, StateActive)
	m.insertStateLocked("p", unrelated, StateActive)
	m.mu.Unlock()

	signer, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	if err != nil {
		t.Fatalf("PickSessionForProfile: %v", err)
	}
	if signer.Address() != "g1newer" {
		t.Errorf("picked %q, want g1newer (most recent matching)", signer.Address())
	}
}

func TestManager_pickSkipsExpired(t *testing.T) {
	m := newTestManager(t)
	expired := activeSession("g1exp", "gpub1exp", []string{"gno.land/r/test/blog"}, 1000)
	expired.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	m.mu.Lock()
	m.insertStateLocked("p", expired, StateActive)
	m.mu.Unlock()
	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	if err == nil {
		t.Fatal("expected error for expired session, got nil")
	}
	if !errors.Is(err, ErrNoActiveSession) {
		t.Errorf("error = %v, want ErrNoActiveSession", err)
	}
}

func TestManager_pickSkipsZeroSpend(t *testing.T) {
	m := newTestManager(t)
	broke := activeSession("g1broke", "gpub1broke", []string{"gno.land/r/test/blog"}, 1000)
	broke.SpendRemaining = "0ugnot"
	m.mu.Lock()
	m.insertStateLocked("p", broke, StateActive)
	m.mu.Unlock()
	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	if err == nil {
		t.Fatal("expected error for zero-spend session, got nil")
	}
}

func TestManager_pickReturnsErrScopeMismatch(t *testing.T) {
	m := newTestManager(t)
	blog := activeSession("g1blog", "gpub1blog", []string{"gno.land/r/test/blog"}, 1000)
	m.mu.Lock()
	m.insertStateLocked("p", blog, StateActive)
	m.mu.Unlock()
	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/forum")
	if err == nil {
		t.Fatal("expected *ErrScopeMismatch, got nil")
	}
	var mismatch *ErrScopeMismatch
	if !errors.As(err, &mismatch) {
		t.Fatalf("error type = %T, want *ErrScopeMismatch", err)
	}
	if len(mismatch.AvailablePaths) == 0 {
		t.Error("ErrScopeMismatch.AvailablePaths is empty")
	}
	if mismatch.AvailablePaths[0] != "gno.land/r/test/blog" {
		t.Errorf("AvailablePaths[0] = %q, want \"gno.land/r/test/blog\"", mismatch.AvailablePaths[0])
	}
}

func TestManager_pickReturnsErrNoActiveSession(t *testing.T) {
	m := newTestManager(t)
	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "empty-profile", "gno.land/r/test/blog")
	if !errors.Is(err, ErrNoActiveSession) {
		t.Fatalf("error = %v, want ErrNoActiveSession", err)
	}
}

func TestManager_pickActivatesPendingFromChain(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	scope := Scope{
		SpendLimit: "500000ugnot",
		ExpiresIn:  time.Hour,
		AllowPaths: []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("testnet", kp, scope)
	if err != nil {
		t.Fatalf("AddPending: %v", err)
	}
	fake := chain.NewFake()
	fake.SetSession(meta.SessionPubkey, chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "500000ugnot",
		SpendRemaining: "500000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	signer, err := m.PickSessionForProfile(context.Background(), resolverFor(fake), "testnet", "gno.land/r/test/blog")
	if err != nil {
		t.Fatalf("PickSessionForProfile after chain activation: %v", err)
	}
	if signer.Address() != meta.SessionAddress {
		t.Errorf("signer address = %q, want %q", signer.Address(), meta.SessionAddress)
	}
}

func TestManager_updateSpendPersists(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	if err != nil {
		t.Fatalf("NewKeypair: %v", err)
	}
	scope := Scope{
		SpendLimit: "1000000ugnot",
		ExpiresIn:  time.Hour,
		AllowPaths: []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("p", kp, scope)
	if err != nil {
		t.Fatalf("AddPending: %v", err)
	}
	m.mu.Lock()
	m.sessions["p"][meta.SessionAddress].state = StateActive
	m.mu.Unlock()
	if err := m.UpdateSpend("p", meta.SessionAddress, 42000); err != nil {
		t.Fatalf("UpdateSpend: %v", err)
	}
	sessions := m.ListForProfile("p")
	if len(sessions) != 1 {
		t.Fatalf("ListForProfile returned %d sessions, want 1", len(sessions))
	}
	if sessions[0].SpendRemaining != "958000ugnot" {
		t.Errorf("SpendRemaining = %q, want \"958000ugnot\"", sessions[0].SpendRemaining)
	}
}

func TestManager_hydrateLoadsActiveSkipsInactive(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir, "")
	activeKP, _ := NewKeypair()
	inactiveKP, _ := NewKeypair()
	activeMeta := activeSession(activeKP.Address(), activeKP.PubkeyBech32(),
		[]string{"gno.land/r/test/blog"}, 1000)
	activeMeta.Privkey = activeKP.Priv
	inactiveMeta := activeSession(inactiveKP.Address(), inactiveKP.PubkeyBech32(),
		[]string{"gno.land/r/test/other"}, 2000)
	inactiveMeta.Privkey = inactiveKP.Priv
	if err := m.store.Write("prof", activeMeta); err != nil {
		t.Fatalf("Write active: %v", err)
	}
	if err := m.store.Write("prof", inactiveMeta); err != nil {
		t.Fatalf("Write inactive: %v", err)
	}
	fake := chain.NewFake()
	fake.SetSession(activeKP.PubkeyBech32(), chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "1000000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	fake.SetSession(inactiveKP.PubkeyBech32(), chain.SessionStatus{Active: false})
	if err := m.Hydrate(context.Background(), resolverFor(fake)); err != nil {
		t.Fatalf("Hydrate: %v", err)
	}
	sessions := m.ListForProfile("prof")
	if len(sessions) != 1 {
		t.Fatalf("after Hydrate: %d sessions loaded, want 1", len(sessions))
	}
	if sessions[0].SessionAddress != activeMeta.SessionAddress {
		t.Errorf("loaded session addr = %q, want %q", sessions[0].SessionAddress, activeMeta.SessionAddress)
	}
	_, err := m.store.Read("prof", inactiveMeta.SessionAddress)
	if err == nil {
		t.Fatal("inactive session file still on disk after Hydrate; expected deletion")
	}
}

func TestManager_concurrentAddPickNoRace(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	fake := chain.NewFake()
	resolver := resolverFor(fake)
	var wg sync.WaitGroup
	const goroutines = 20
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			kp, err := NewKeypair()
			if err != nil {
				return
			}
			scope := Scope{
				SpendLimit: "100000ugnot",
				ExpiresIn:  time.Hour,
				AllowPaths: []string{"gno.land/r/test/blog"},
			}
			_, _ = m.AddPending("p", kp, scope)
			_, _ = m.PickSessionForProfile(context.Background(), resolver, "p", "gno.land/r/test/blog")
		}()
	}
	wg.Wait()
}

func TestCoversRealm_doesNotMatchSiblingName(t *testing.T) {
	if coversRealm([]string{"gno.land/r/test"}, "gno.land/r/testing") {
		t.Error("coversRealm must not match a realm that only shares a string prefix but is not a sub-path")
	}
}

func TestCoversRealm_matchesExactAndSubpath(t *testing.T) {
	if !coversRealm([]string{"gno.land/r/test"}, "gno.land/r/test") {
		t.Error("coversRealm must match exact realm")
	}
	if !coversRealm([]string{"gno.land/r/test"}, "gno.land/r/test/blog") {
		t.Error("coversRealm must match a sub-path")
	}
}

func TestManager_markActive_persistsAndUpdates(t *testing.T) {
	m := newTestManager(t)
	kp, _ := NewKeypair()
	_, err := m.AddPending("p", kp, Scope{AllowPaths: []string{"r/x"}, SpendLimit: "1000ugnot", ExpiresIn: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	err = m.MarkActive("p", kp.Address(), chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"r/x"},
		SpendLimit:     "1000ugnot",
		SpendRemaining: "1000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	metas := m.ListForProfile("p")
	if len(metas) != 1 || metas[0].State != StateActive {
		t.Errorf("expected one active session, got %+v", metas)
	}
}

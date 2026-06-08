package session

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		MasterAddress:  "g1master",
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
	require.NoError(t, err, "NewKeypair")

	scope := Scope{
		SpendLimit:  "500000ugnot",
		SpendPeriod: time.Hour,
		ExpiresIn:   time.Hour,
		AllowPaths:  []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("testnet", kp, scope, "g1master")
	require.NoError(t, err, "AddPending")

	got, err := m.store.Read("testnet", meta.SessionAddress)
	require.NoError(t, err, "store.Read after AddPending")
	assert.Equal(t, meta.SessionAddress, got.SessionAddress)
	assert.Equal(t, "g1master", got.MasterAddress)
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
	require.NoError(t, err, "PickSessionForProfile")
	assert.Equal(t, "g1newer", signer.Address(), "should pick most recent matching session")
}

func TestManager_pickSkipsExpired(t *testing.T) {
	m := newTestManager(t)
	expired := activeSession("g1exp", "gpub1exp", []string{"gno.land/r/test/blog"}, 1000)
	expired.ExpiresAt = time.Now().Add(-time.Hour).Unix()
	m.mu.Lock()
	m.insertStateLocked("p", expired, StateActive)
	m.mu.Unlock()

	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	require.Error(t, err, "expected error for expired session")
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestManager_pickSkipsZeroSpend(t *testing.T) {
	m := newTestManager(t)
	broke := activeSession("g1broke", "gpub1broke", []string{"gno.land/r/test/blog"}, 1000)
	broke.SpendRemaining = "0ugnot"
	m.mu.Lock()
	m.insertStateLocked("p", broke, StateActive)
	m.mu.Unlock()

	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	require.Error(t, err, "expected error for zero-spend session")
}

func TestManager_pickReturnsErrScopeMismatch(t *testing.T) {
	m := newTestManager(t)
	blog := activeSession("g1blog", "gpub1blog", []string{"gno.land/r/test/blog"}, 1000)
	m.mu.Lock()
	m.insertStateLocked("p", blog, StateActive)
	m.mu.Unlock()

	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/forum")
	require.Error(t, err, "expected *ErrScopeMismatch")

	var mismatch *ErrScopeMismatch
	require.True(t, errors.As(err, &mismatch), "error type = %T, want *ErrScopeMismatch", err)
	require.NotEmpty(t, mismatch.AvailablePaths, "ErrScopeMismatch.AvailablePaths is empty")
	assert.Equal(t, "gno.land/r/test/blog", mismatch.AvailablePaths[0])
}

func TestManager_pickReturnsErrNoActiveSession(t *testing.T) {
	m := newTestManager(t)
	_, err := m.PickSessionForProfile(context.Background(), nullResolver(), "empty-profile", "gno.land/r/test/blog")
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestManager_pickActivatesPendingFromChain(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	require.NoError(t, err, "NewKeypair")

	scope := Scope{
		SpendLimit: "500000ugnot",
		ExpiresIn:  time.Hour,
		AllowPaths: []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("testnet", kp, scope, "g1master")
	require.NoError(t, err, "AddPending")

	fake := chain.NewFake()
	fake.SetSession("g1master", meta.SessionAddress, chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "500000ugnot",
		SpendRemaining: "500000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	signer, err := m.PickSessionForProfile(context.Background(), resolverFor(fake), "testnet", "gno.land/r/test/blog")
	require.NoError(t, err, "PickSessionForProfile after chain activation")
	assert.Equal(t, meta.SessionAddress, signer.Address())
}

func TestManager_updateSpendPersists(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	require.NoError(t, err, "NewKeypair")

	scope := Scope{
		SpendLimit: "1000000ugnot",
		ExpiresIn:  time.Hour,
		AllowPaths: []string{"gno.land/r/test/blog"},
	}
	meta, err := m.AddPending("p", kp, scope, "g1master")
	require.NoError(t, err, "AddPending")

	m.mu.Lock()
	m.sessions["p"][meta.SessionAddress].state = StateActive
	m.mu.Unlock()

	require.NoError(t, m.UpdateSpend("p", meta.SessionAddress, 42000), "UpdateSpend")

	sessions := m.ListForProfile("p")
	require.Len(t, sessions, 1)
	assert.Equal(t, "958000ugnot", sessions[0].SpendRemaining)
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

	require.NoError(t, m.store.Write("prof", activeMeta), "Write active")
	require.NoError(t, m.store.Write("prof", inactiveMeta), "Write inactive")

	fake := chain.NewFake()
	fake.SetSession("g1master", activeKP.Address(), chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"gno.land/r/test/blog"},
		SpendLimit:     "1000000ugnot",
		SpendRemaining: "1000000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	fake.SetSession("g1master", inactiveKP.Address(), chain.SessionStatus{Active: false})

	require.NoError(t, m.Hydrate(context.Background(), resolverFor(fake)), "Hydrate")

	sessions := m.ListForProfile("prof")
	require.Len(t, sessions, 1, "after Hydrate: unexpected number of sessions loaded")
	assert.Equal(t, activeMeta.SessionAddress, sessions[0].SessionAddress)

	_, err := m.store.Read("prof", inactiveMeta.SessionAddress)
	require.Error(t, err, "inactive session file still on disk after Hydrate; expected deletion")
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
			_, _ = m.AddPending("p", kp, scope, "g1master")
			_, _ = m.PickSessionForProfile(context.Background(), resolver, "p", "gno.land/r/test/blog")
		}()
	}
	wg.Wait()
}

func TestManager_pickAnyActiveSessionWithEmptyRealm(t *testing.T) {
	m := newTestManager(t)
	s1 := activeSession("g1one", "gpub1one", []string{"gno.land/r/test/blog"}, 1000)
	m.mu.Lock()
	m.insertStateLocked("p", s1, StateActive)
	m.mu.Unlock()

	signer, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "")
	require.NoError(t, err, "expected success with empty realm wildcard")
	assert.Equal(t, "g1one", signer.Address())
}

func TestCoversRealm_doesNotMatchSiblingName(t *testing.T) {
	assert.False(t, coversRealm([]string{"gno.land/r/test"}, "gno.land/r/testing"),
		"coversRealm must not match a realm that only shares a string prefix but is not a sub-path")
}

func TestCoversRealm_matchesExactAndSubpath(t *testing.T) {
	assert.True(t, coversRealm([]string{"gno.land/r/test"}, "gno.land/r/test"),
		"coversRealm must match exact realm")
	assert.True(t, coversRealm([]string{"gno.land/r/test"}, "gno.land/r/test/blog"),
		"coversRealm must match a sub-path")
}

func TestManager_pickForRun_skipsSessionWithoutAllowRun(t *testing.T) {
	m := newTestManager(t)
	noRun := activeSession("g1norun", "gpub1norun", []string{"gno.land/r/test/blog"}, 1000)
	m.mu.Lock()
	m.insertStateLocked("p", noRun, StateActive)
	m.mu.Unlock()

	_, err := m.PickSessionForRun(context.Background(), nullResolver(), "p")
	require.Error(t, err, "expected error when no session has AllowRun=true")

	var mismatch *ErrScopeMismatch
	assert.True(t, errors.As(err, &mismatch) || errors.Is(err, ErrNoActiveSession),
		"error = %v, want ErrScopeMismatch or ErrNoActiveSession", err)
}

func TestManager_pickForRun_returnsSessionWithAllowRun(t *testing.T) {
	m := newTestManager(t)
	withRun := activeSession("g1run", "gpub1run", nil, 2000)
	withRun.AllowRun = true
	m.mu.Lock()
	m.insertStateLocked("p", withRun, StateActive)
	m.mu.Unlock()

	signer, err := m.PickSessionForRun(context.Background(), nullResolver(), "p")
	require.NoError(t, err, "PickSessionForRun")
	assert.Equal(t, "g1run", signer.Address())
}

func TestManager_pickForRun_emptyProfileReturnsNoActiveSession(t *testing.T) {
	m := newTestManager(t)
	_, err := m.PickSessionForRun(context.Background(), nullResolver(), "empty")
	require.ErrorIs(t, err, ErrNoActiveSession)
}

func TestManager_sessionWithBothAllowPathsAndRun_pickedByEither(t *testing.T) {
	m := newTestManager(t)
	both := activeSession("g1both", "gpub1both", []string{"gno.land/r/test/blog"}, 1000)
	both.AllowRun = true
	m.mu.Lock()
	m.insertStateLocked("p", both, StateActive)
	m.mu.Unlock()

	// Realm-based pick works.
	signer, err := m.PickSessionForProfile(context.Background(), nullResolver(), "p", "gno.land/r/test/blog")
	require.NoError(t, err, "PickSessionForProfile")
	assert.Equal(t, "g1both", signer.Address(), "PickSessionForProfile")

	// Run pick works.
	signer, err = m.PickSessionForRun(context.Background(), nullResolver(), "p")
	require.NoError(t, err, "PickSessionForRun")
	assert.Equal(t, "g1both", signer.Address(), "PickSessionForRun")
}

func TestManager_pickForRun_activatesPendingFromChainWithAllowRun(t *testing.T) {
	m := newTestManager(t)
	kp, err := NewKeypair()
	require.NoError(t, err, "NewKeypair")

	scope := Scope{
		SpendLimit: "500000ugnot",
		ExpiresIn:  time.Hour,
		AllowRun:   true,
	}
	meta, err := m.AddPending("testnet", kp, scope, "g1master")
	require.NoError(t, err, "AddPending")

	fake := chain.NewFake()
	fake.SetSession("g1master", meta.SessionAddress, chain.SessionStatus{
		Active:         true,
		AllowRun:       true,
		SpendLimit:     "500000ugnot",
		SpendRemaining: "500000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	signer, err := m.PickSessionForRun(context.Background(), resolverFor(fake), "testnet")
	require.NoError(t, err, "PickSessionForRun after chain activation")
	assert.Equal(t, meta.SessionAddress, signer.Address())
}

func TestManager_markActive_persistsAndUpdates(t *testing.T) {
	m := newTestManager(t)
	kp, _ := NewKeypair()
	_, err := m.AddPending("p", kp, Scope{AllowPaths: []string{"r/x"}, SpendLimit: "1000ugnot", ExpiresIn: time.Hour}, "g1master")
	require.NoError(t, err)

	err = m.MarkActive("p", kp.Address(), chain.SessionStatus{
		Active:         true,
		AllowPaths:     []string{"r/x"},
		SpendLimit:     "1000ugnot",
		SpendRemaining: "1000ugnot",
		ExpiresAt:      time.Now().Add(time.Hour).Unix(),
	})
	require.NoError(t, err)

	metas := m.ListForProfile("p")
	require.Len(t, metas, 1)
	assert.Equal(t, StateActive, metas[0].State, "expected one active session")
}

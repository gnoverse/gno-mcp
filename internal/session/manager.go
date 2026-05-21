package session

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
)

// ---- State constants

const (
	StatePending = "pending"
	StateActive  = "active"
	StateExpired = "expired"
	StateRevoked = "revoked"
)

// ---- sessionState

// sessionState pairs in-memory lifecycle state with the persisted metadata.
type sessionState struct {
	state string
	meta  *SessionMeta
}

// ---- signerAdapter

// signerAdapter reconstructs chain.Signer from a stored SessionMeta.
// It reads Address and Pubkey from meta fields and signs using the raw
// ed25519 private key stored in meta.Privkey (seed||pubkey, 64 bytes).
type signerAdapter struct {
	addr string
	priv []byte
}

func (s *signerAdapter) Address() string { return s.addr }

func (s *signerAdapter) Sign(payload []byte) ([]byte, error) {
	if len(s.priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("session: signerAdapter: invalid privkey length %d", len(s.priv))
	}
	sig := ed25519.Sign(ed25519.PrivateKey(s.priv), payload)
	return sig, nil
}

// ---- Manager

// Manager owns the in-memory session map (profile → sessionAddr → sessionState).
// It is the single source of truth for session lifecycle. All public methods
// are safe for concurrent use.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]map[string]*sessionState // profile → addr → state
	store    *Store
}

// NewManager constructs a Manager backed by a Store at rootDir.
// When passphrase is non-empty the store encrypts private keys at rest.
func NewManager(rootDir, passphrase string) *Manager {
	return &Manager{
		sessions: make(map[string]map[string]*sessionState),
		store:    NewStore(rootDir, passphrase),
	}
}

// ---- Hydrate

// Hydrate walks the store at startup. For each profile+session it queries the
// chain: if active, load into memory as StateActive; if inactive, delete from disk.
// Errors from individual sessions are logged and skipped; Hydrate returns an
// error only when listing profiles or sessions from disk fails.
func (m *Manager) Hydrate(ctx context.Context, resolver chain.Resolver) error {
	profiles, err := m.store.listProfiles()
	if err != nil {
		return fmt.Errorf("session/manager: hydrate: %w", err)
	}
	for _, profile := range profiles {
		metas, err := m.store.List(profile)
		if err != nil {
			return fmt.Errorf("session/manager: hydrate list %q: %w", profile, err)
		}
		for _, meta := range metas {
			result, err := queryChain(ctx, resolver, profile, meta.SessionPubkey)
			if err != nil {
				log.Printf("session/manager: hydrate: query chain for session %q in profile %q: %v (keeping local state)", meta.SessionAddress, profile, err)
				m.insertState(profile, meta, meta.State)
				continue
			}
			if result.Unsupported {
				// Chain build cannot confirm or deny — keep local state as-is.
				m.insertState(profile, meta, meta.State)
				continue
			}
			if !result.Active {
				if delErr := m.store.Delete(profile, meta.SessionAddress); delErr != nil {
					log.Printf("session/manager: hydrate: delete inactive session %q in profile %q: %v (will retry on next hydrate)", meta.SessionAddress, profile, delErr)
				}
				continue
			}
			// Sync chain-side scope fields into meta before loading.
			meta.AllowPaths = result.Status.AllowPaths
			meta.SpendLimit = result.Status.SpendLimit
			meta.SpendRemaining = result.Status.SpendRemaining
			meta.ExpiresAt = result.Status.ExpiresAt
			meta.State = StateActive
			m.insertState(profile, meta, StateActive)
		}
	}
	return nil
}

// ---- AddPending

// AddPending creates a new pending session from a freshly generated keypair and
// caller-supplied scope, persists it to disk, and registers it in-memory as
// StatePending. Returns the populated SessionMeta (without private key encrypted).
func (m *Manager) AddPending(profile string, kp *Keypair, scope Scope) (*SessionMeta, error) {
	now := time.Now()
	meta := &SessionMeta{
		Version:        1,
		SessionAddress: kp.Address(),
		SessionPubkey:  kp.PubkeyBech32(),
		Privkey:        kp.Priv,
		AllowPaths:     scope.AllowPaths,
		SpendLimit:     scope.SpendLimit,
		SpendRemaining: scope.SpendLimit,
		ExpiresAt:      now.Add(scope.ExpiresIn).Unix(),
		CreatedAt:      now.Unix(),
		State:          StatePending,
	}
	if err := m.store.Write(profile, meta); err != nil {
		return nil, fmt.Errorf("session/manager: persist pending: %w", err)
	}
	m.insertState(profile, meta, StatePending)
	return meta, nil
}

// ---- PickSessionForProfile

// PickSessionForProfile returns a chain.Signer for the best usable session
// covering realm for the given profile.
//
// Pick algorithm:
//  1. For each pending session, query the chain. If now active, transition to StateActive.
//  2. Collect all StateActive sessions that are unexpired and have spend remaining.
//  3. If none: return ErrNoActiveSession.
//  4. Filter to those whose AllowPaths covers realm.
//  5. If none cover realm but usable sessions exist: return *ErrScopeMismatch.
//  6. Among matching sessions, return the one with the greatest CreatedAt (most recent).
func (m *Manager) PickSessionForProfile(ctx context.Context, resolver chain.Resolver, profile, realm string) (chain.Signer, error) {
	// Step 1: snapshot pending sessions for promotion — no IO under lock.
	type pending struct {
		addr   string
		pubkey string
	}
	m.mu.RLock()
	var toPromote []pending
	for addr, ss := range m.sessions[profile] {
		if ss.state == StatePending {
			toPromote = append(toPromote, pending{addr: addr, pubkey: ss.meta.SessionPubkey})
		}
	}
	m.mu.RUnlock()

	// Step 2: promote outside the lock — queryChain + store.Write are IO.
	for _, p := range toPromote {
		res, err := queryChain(ctx, resolver, profile, p.pubkey)
		if err != nil || !res.Active {
			continue
		}
		m.mu.Lock()
		ss := m.sessions[profile][p.addr]
		if ss == nil || ss.state != StatePending {
			m.mu.Unlock()
			continue
		}
		ss.meta.AllowPaths = res.Status.AllowPaths
		ss.meta.SpendLimit = res.Status.SpendLimit
		ss.meta.SpendRemaining = res.Status.SpendRemaining
		ss.meta.ExpiresAt = res.Status.ExpiresAt
		ss.meta.State = StateActive
		ss.state = StateActive
		metaCopy := *ss.meta
		metaCopy.AllowPaths = slices.Clone(ss.meta.AllowPaths)
		metaCopy.Privkey = slices.Clone(ss.meta.Privkey)
		m.mu.Unlock()

		if err := m.store.Write(profile, &metaCopy); err != nil {
			log.Printf("session/manager: pick: persist activated %q: %v", p.addr, err)
		}
	}

	// Step 3: pick under lock — pure in-memory, no IO.
	m.mu.Lock()
	defer m.mu.Unlock()

	profileSessions := m.sessions[profile]
	if len(profileSessions) == 0 {
		return nil, ErrNoActiveSession
	}

	now := time.Now().Unix()
	var candidates []*sessionState
	var availablePaths []string

	for _, ss := range profileSessions {
		if ss.state != StateActive {
			continue
		}
		if ss.meta.ExpiresAt > 0 && now > ss.meta.ExpiresAt {
			ss.state = StateExpired
			continue
		}
		if isZeroSpend(ss.meta.SpendRemaining) {
			continue
		}
		availablePaths = append(availablePaths, ss.meta.AllowPaths...)
		if realm != "" && !coversRealm(ss.meta.AllowPaths, realm) {
			continue
		}
		candidates = append(candidates, ss)
	}

	if len(candidates) == 0 && len(availablePaths) == 0 {
		return nil, ErrNoActiveSession
	}
	if len(candidates) == 0 {
		return nil, &ErrScopeMismatch{AvailablePaths: dedup(availablePaths)}
	}

	// Pick most-recently created.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].meta.CreatedAt > candidates[j].meta.CreatedAt
	})
	best := candidates[0]

	return &signerAdapter{
		addr: best.meta.SessionAddress,
		priv: best.meta.Privkey,
	}, nil
}

// ---- UpdateSpend

// UpdateSpend subtracts gasUsed from the session's SpendRemaining and persists
// the updated meta to disk. Returns an error if the session is not found or
// not active.
func (m *Manager) UpdateSpend(profile, sessionAddr string, gasUsed int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ss := m.getStateLocked(profile, sessionAddr)
	if ss == nil {
		return fmt.Errorf("session/manager: UpdateSpend: session %q not found for profile %q", sessionAddr, profile)
	}
	if ss.state != StateActive {
		return fmt.Errorf("session/manager: UpdateSpend: session %q is not active (state=%s)", sessionAddr, ss.state)
	}

	remaining, denom, err := parseCoins(ss.meta.SpendRemaining)
	if err != nil {
		return fmt.Errorf("session/manager: UpdateSpend: parse SpendRemaining %q: %w", ss.meta.SpendRemaining, err)
	}
	remaining -= gasUsed
	if remaining < 0 {
		remaining = 0
	}
	ss.meta.SpendRemaining = strconv.FormatInt(remaining, 10) + denom

	if err := m.store.Write(profile, ss.meta); err != nil {
		return fmt.Errorf("session/manager: UpdateSpend: persist: %w", err)
	}
	return nil
}

// ---- ListForProfile

// ListForProfile returns a copy of the metadata for all sessions tracked for
// the given profile, regardless of state. Safe to call concurrently.
func (m *Manager) ListForProfile(profile string) []*SessionMeta {
	m.mu.Lock()
	defer m.mu.Unlock()

	profileSessions := m.sessions[profile]
	out := make([]*SessionMeta, 0, len(profileSessions))
	for _, ss := range profileSessions {
		cp := *ss.meta
		cp.AllowPaths = slices.Clone(ss.meta.AllowPaths)
		cp.Privkey = slices.Clone(ss.meta.Privkey)
		out = append(out, &cp)
	}
	return out
}

// ---- Get

// Get returns the SessionMeta for the given session address within a profile,
// or nil if not found.
func (m *Manager) Get(profile, sessionAddr string) *SessionMeta {
	m.mu.Lock()
	defer m.mu.Unlock()

	ss := m.getStateLocked(profile, sessionAddr)
	if ss == nil {
		return nil
	}
	cp := *ss.meta
	cp.AllowPaths = slices.Clone(ss.meta.AllowPaths)
	cp.Privkey = slices.Clone(ss.meta.Privkey)
	return &cp
}

// ---- MarkActive

// MarkActive transitions a known pending session to StateActive and refreshes
// its scope fields from the supplied chain status. It also persists the updated
// meta to disk. Returns an error if the session is not found.
func (m *Manager) MarkActive(profile, sessionAddr string, status chain.SessionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ss := m.getStateLocked(profile, sessionAddr)
	if ss == nil {
		return fmt.Errorf("session/manager: MarkActive: session %q not found for profile %q", sessionAddr, profile)
	}
	ss.meta.AllowPaths = status.AllowPaths
	ss.meta.SpendLimit = status.SpendLimit
	ss.meta.SpendRemaining = status.SpendRemaining
	ss.meta.ExpiresAt = status.ExpiresAt
	ss.meta.State = StateActive
	ss.state = StateActive

	if err := m.store.Write(profile, ss.meta); err != nil {
		return fmt.Errorf("session/manager: MarkActive: persist: %w", err)
	}
	return nil
}

// ---- Internal helpers

// insertState is the unlocked wrapper — acquires the lock, then calls insertStateLocked.
func (m *Manager) insertState(profile string, meta *SessionMeta, state string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.insertStateLocked(profile, meta, state)
}

// insertStateLocked registers meta under profile with the given state.
// Caller must hold m.mu.
func (m *Manager) insertStateLocked(profile string, meta *SessionMeta, state string) {
	if m.sessions[profile] == nil {
		m.sessions[profile] = make(map[string]*sessionState)
	}
	m.sessions[profile][meta.SessionAddress] = &sessionState{
		state: state,
		meta:  meta,
	}
}

// getStateLocked returns the sessionState for addr within profile, or nil.
// Caller must hold m.mu.
func (m *Manager) getStateLocked(profile, addr string) *sessionState {
	profileSessions := m.sessions[profile]
	if profileSessions == nil {
		return nil
	}
	return profileSessions[addr]
}

// coversRealm returns true when realm is a prefix of (or equals) at least one
// of the allowed paths. The check is path-prefix based: "gno.land/r/test"
// covers "gno.land/r/test/blog".
func coversRealm(allowPaths []string, realm string) bool {
	for _, p := range allowPaths {
		if p == realm || strings.HasPrefix(realm, p+"/") {
			return true
		}
	}
	return false
}

// dedup returns a new slice with duplicate strings removed, preserving order.
func dedup(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// isZeroSpend returns true when the coin magnitude is 0.
func isZeroSpend(coin string) bool {
	if coin == "" {
		return false
	}
	mag, _, err := parseCoins(coin)
	if err != nil {
		return false
	}
	return mag == 0
}

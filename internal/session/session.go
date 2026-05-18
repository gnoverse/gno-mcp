// Package session manages the MCP-owned signing identity.
//
// Model (v0.2 demo):
//   - On startup, the MCP server holds NO key. State = "unauthenticated".
//   - On first write attempt, the MCP returns an authentication_required
//     structured error carrying a fund link + ASCII QR. The user funds the
//     session address from their primary wallet.
//   - Once the session address has balance ≥ threshold, state flips to
//     "authenticated" and writes proceed under the session signer.
//   - Sessions are in-process by default. Set GNO_MCP_SESSION_FILE +
//     GNO_MCP_SESSION_PASSPHRASE to persist the keypair encrypted at rest.
//
// TODO(v0.3): when gnopie ships as a library, swap the stdlib ed25519
// keypair for tm2/pkg/crypto/keys (secp256k1, real gno bech32 addresses).
package session

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"
	"time"
)

// FundThreshold is the minimum ugnot balance that flips an
// unauthenticated/pending session to authenticated. Configurable per-server
// later; hard-coded for v0.2 demo.
const FundThreshold int64 = 1_000_000 // 1 GNOT

// State is the v0.2 session lifecycle.
type State string

const (
	StateUnauthenticated State = "unauthenticated" // no key generated yet
	StatePending         State = "pending"         // key generated, awaiting funding
	StateAuthenticated   State = "authenticated"   // funded, writes proceed
	StateExpired         State = "expired"         // depleted or revoked
)

// BalanceFetcher reports the on-chain balance for an address. The session
// manager uses it to flip pending → authenticated without coupling to the
// full GnopieClient interface. In tests this is a closure over a map; in
// prod it wraps the real client.
type BalanceFetcher func(ctx context.Context, network, addr string) (int64, error)

// Manager holds the session keypair + state. Safe for concurrent use.
type Manager struct {
	mu sync.RWMutex

	network   string // e.g. "staging.gno.land"
	pub       ed25519.PublicKey
	priv      ed25519.PrivateKey
	addr      string
	state     State
	createdAt time.Time
	lastCheck time.Time
	lastBal   int64
	balance   BalanceFetcher
}

// Options configure a new Manager.
type Options struct {
	Network string
	Balance BalanceFetcher
}

// New returns an unauthenticated Manager. A keypair is not generated until
// EnsurePending is called — that way fresh start (e.g. inside Docker)
// surfaces the unauthenticated state explicitly to the LLM.
func New(opts Options) *Manager {
	if opts.Network == "" {
		opts.Network = "staging.gno.land"
	}
	return &Manager{
		network: opts.Network,
		state:   StateUnauthenticated,
		balance: opts.Balance,
	}
}

// EnsurePending generates the keypair on first call and flips state to
// pending. Idempotent.
func (m *Manager) EnsurePending() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state != StateUnauthenticated {
		return nil
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	m.pub = pub
	m.priv = priv
	m.addr = addressFromPub(pub)
	m.state = StatePending
	m.createdAt = time.Now().UTC()
	return nil
}

// Status returns a snapshot of the manager. Cheap; lock-bound but read-only.
func (m *Manager) Status() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Snapshot{
		State:     m.state,
		Network:   m.network,
		Address:   m.addr,
		Balance:   m.lastBal,
		Threshold: FundThreshold,
		CreatedAt: m.createdAt,
		LastCheck: m.lastCheck,
	}
}

// Snapshot is the public read-only view of session state.
type Snapshot struct {
	State     State     `json:"state"`
	Network   string    `json:"network"`
	Address   string    `json:"address,omitempty"`
	Balance   int64     `json:"balance_ugnot"`
	Threshold int64     `json:"threshold_ugnot"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	LastCheck time.Time `json:"last_check,omitempty"`
}

// Signer returns the session signer name for use in audit logs. Empty if
// unauthenticated; tools use this to decide whether to gate.
func (m *Manager) Signer() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state != StateAuthenticated {
		return ""
	}
	return "mcp-session"
}

// Address returns the bech32 session address (empty when no keypair yet).
func (m *Manager) Address() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.addr
}

// State returns the current state without other snapshot fields.
func (m *Manager) State() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Refresh polls the balance fetcher and updates state. Called by tools that
// need an up-to-date authorization check (e.g. gno_auth_status, write tools).
//
// Errors from the fetcher do NOT change state — we want the session to stay
// authenticated through transient RPC failures.
func (m *Manager) Refresh(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateUnauthenticated || m.balance == nil {
		return nil
	}
	bal, err := m.balance(ctx, m.network, m.addr)
	if err != nil {
		return err
	}
	m.lastBal = bal
	m.lastCheck = time.Now().UTC()
	switch {
	case bal >= FundThreshold:
		m.state = StateAuthenticated
	case m.state == StateAuthenticated && bal < FundThreshold/2:
		// Hysteresis: only mark expired once we drop below half the threshold,
		// so a single broadcast doesn't immediately re-prompt the user.
		m.state = StateExpired
	}
	return nil
}

// SetAuthenticated is a test helper that bypasses the balance fetcher.
// Production code paths must go through Refresh.
func (m *Manager) SetAuthenticated(balance int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state == StateUnauthenticated {
		return errors.New("session: keypair not generated; call EnsurePending first")
	}
	m.lastBal = balance
	m.lastCheck = time.Now().UTC()
	m.state = StateAuthenticated
	return nil
}

package server

import (
	"errors"
	"fmt"
	"sync"

	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// Dynamic-profile name errors. Callers match with errors.Is to map them to
// structured tool errors.
var (
	ErrProfileNameInvalid = errors.New("profile name must be lowercase alphanumeric with internal '-' or '_'")
	ErrProfileReserved    = errors.New(`"default" is a reserved profile name`)
	ErrProfileImmutable   = errors.New("profile was loaded at startup and cannot be overridden dynamically")
)

// Server is the gnomcp MCP server. Holds the registry and config; transport
// is wired in cmd/gnomcp/main.go using the official MCP Go SDK.
//
// The config is mutable at runtime via AddDynamicProfile, with copy-on-write
// semantics: Config() returns an immutable snapshot, and an add swaps in a
// fresh *profiles.Config. Profiles present at construction (builtins, toml,
// -config) can never be overridden by a dynamic add.
type Server struct {
	mu              sync.RWMutex
	cfg             *profiles.Config
	initNames       map[string]struct{} // profile names loaded at startup; immutable thereafter
	discoveredLocal string              // profile name or ""
	registry        *Registry
}

func NewServer(cfg *profiles.Config, discoveredLocal string) *Server {
	s := &Server{
		cfg:             cfg,
		discoveredLocal: discoveredLocal,
		registry:        NewRegistry(),
		initNames:       map[string]struct{}{},
	}
	if cfg != nil {
		for name := range cfg.Profiles {
			s.initNames[name] = struct{}{}
		}
	}
	return s
}

func (s *Server) Registry() *Registry { return s.registry }

// Config returns the current profile-config snapshot. Snapshots are never
// mutated after publication, so the returned value is safe to read without
// holding any lock; it just may become stale after a dynamic add.
func (s *Server) Config() *profiles.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func (s *Server) DiscoveredLocal() string { return s.discoveredLocal }

// CheckDynamicProfileName reports whether name may be used for a dynamic
// profile add: syntactically valid, not reserved, and not an init-time
// profile. AddDynamicProfile re-checks under its lock; this exists so tools
// can fail fast before doing expensive validation or network I/O.
func (s *Server) CheckDynamicProfileName(name string) error {
	if !profiles.ValidProfileName(name) {
		return fmt.Errorf("profile %q: %w", name, ErrProfileNameInvalid)
	}
	if name == "default" {
		return ErrProfileReserved
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, isInit := s.initNames[name]; isInit {
		return fmt.Errorf("profile %q: %w", name, ErrProfileImmutable)
	}
	return nil
}

// AddDynamicProfile registers p under name for the rest of the process
// lifetime (in-memory only; lost on restart). Re-adding a name that was
// itself dynamically added replaces it; init-time names are rejected.
// The caller is responsible for validating p (chain-id allowlist, URL shape)
// before adding.
func (s *Server) AddDynamicProfile(name string, p profiles.Profile) error {
	if !profiles.ValidProfileName(name) {
		return fmt.Errorf("profile %q: %w", name, ErrProfileNameInvalid)
	}
	if name == "default" {
		return ErrProfileReserved
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, isInit := s.initNames[name]; isInit {
		return fmt.Errorf("profile %q: %w", name, ErrProfileImmutable)
	}
	next := make(map[string]profiles.Profile, len(s.cfg.Profiles)+1)
	for k, v := range s.cfg.Profiles {
		next[k] = v
	}
	next[name] = p
	s.cfg = &profiles.Config{Profiles: next}
	return nil
}

// AnyProfileHasIndexer returns true if any loaded profile sets tx-indexer-url.
// Used to gate registration of CapIndexerRead tools.
func (s *Server) AnyProfileHasIndexer() bool {
	for _, p := range s.Config().Profiles {
		if p.TxIndexerURL != "" {
			return true
		}
	}
	return false
}

// AnyProfileTestnet reports whether any profile is testnet — gates gno_faucet_fund
// (local uses test1 and needs no faucet).
func (s *Server) AnyProfileTestnet() bool {
	for _, p := range s.Config().Profiles {
		if p.IsTestnet() {
			return true
		}
	}
	return false
}

// ProfileSchema returns the profile-arg schema for the current config + discovery.
func (s *Server) ProfileSchema() ProfileSchema {
	return ProfileArgSchema(s.Config(), s.discoveredLocal)
}

package server

import (
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// Server is the gnomcp MCP server. Holds the registry and config; transport
// is wired in cmd/gnomcp/main.go using the official MCP Go SDK.
type Server struct {
	cfg             *profiles.Config
	discoveredLocal string // profile name or ""
	registry        *Registry
}

func NewServer(cfg *profiles.Config, discoveredLocal string) *Server {
	return &Server{
		cfg:             cfg,
		discoveredLocal: discoveredLocal,
		registry:        NewRegistry(),
	}
}

func (s *Server) Registry() *Registry      { return s.registry }
func (s *Server) Config() *profiles.Config { return s.cfg }
func (s *Server) DiscoveredLocal() string  { return s.discoveredLocal }

// AnyProfileHasIndexer returns true if any loaded profile sets tx-indexer-url.
// Used to gate registration of CapIndexerRead tools.
func (s *Server) AnyProfileHasIndexer() bool {
	for _, p := range s.cfg.Profiles {
		if p.TxIndexerURL != "" {
			return true
		}
	}
	return false
}

// AnyProfileAgentCapable reports whether any profile is local or testnet — i.e.
// the agent has or can generate its own signing key. Given the chain-id allowlist
// (dev|test*), this is effectively any configured profile.
// Gates agent-only tools: gno_addpkg, gno_key_address, gno_key_generate.
func (s *Server) AnyProfileAgentCapable() bool {
	for _, p := range s.cfg.Profiles {
		if p.ChainType == profiles.ChainTypeLocal || p.ChainType == profiles.ChainTypeTestnet {
			return true
		}
	}
	return false
}

// AnyProfileTestnet reports whether any profile is testnet — gates gno_faucet_fund
// (local uses test1 and needs no faucet; prod has no agent key).
func (s *Server) AnyProfileTestnet() bool {
	for _, p := range s.cfg.Profiles {
		if p.ChainType == profiles.ChainTypeTestnet {
			return true
		}
	}
	return false
}

// ProfileSchema returns the profile-arg schema for the current config + discovery.
func (s *Server) ProfileSchema() ProfileSchema {
	return ProfileArgSchema(s.cfg, s.discoveredLocal)
}

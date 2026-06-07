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

// AnyProfileLocal reports whether any profile is a local (dev) chain — i.e. the
// agent has a usable test1 key. Gates the agent-only tools (gno_addpkg, gno_key_address).
func (s *Server) AnyProfileLocal() bool {
	for _, p := range s.cfg.Profiles {
		if p.ChainType == profiles.ChainTypeLocal {
			return true
		}
	}
	return false
}

// ProfileSchema returns the profile-arg schema for the current config + discovery.
func (s *Server) ProfileSchema() ProfileSchema {
	return ProfileArgSchema(s.cfg, s.discoveredLocal)
}

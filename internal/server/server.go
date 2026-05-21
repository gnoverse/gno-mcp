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

// AnyProfileAllowsDangerous returns true if any loaded profile sets
// allow-dangerous-tools=true. Used to gate registration of write tools.
func (s *Server) AnyProfileAllowsDangerous() bool {
	for _, p := range s.cfg.Profiles {
		if p.AllowDangerousTools {
			return true
		}
	}
	return false
}

// ProfileSchema returns the profile-arg schema for the current config + discovery.
func (s *Server) ProfileSchema() ProfileSchema {
	return ProfileArgSchema(s.cfg, s.discoveredLocal)
}

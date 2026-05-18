package mcp

import (
	"context"
	"fmt"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/session"
	"github.com/gnolang/gno-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	s      *server.MCPServer
	client client.GnopieClient
}

// New constructs the MCP server. sess may be nil; tools that depend on it
// will respond with an "internal" error if so. Pass a real *session.Manager
// from main; tests use NewWithSession.
func New(c client.GnopieClient, a *audit.Log) *Server {
	return NewWithSession(c, a, nil)
}

// NewWithSession wires a session manager into the tool Deps. Production
// callers should always use this; tests opt-in.
func NewWithSession(c client.GnopieClient, a *audit.Log, sess *session.Manager) *Server {
	s := server.NewMCPServer("gno-mcp", "0.2.0", server.WithToolCapabilities(true))
	srv := &Server{s: s, client: c}
	tools.RegisterAll(s, tools.Deps{Client: c, Audit: a, Session: sess})
	return srv
}

func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.s)
}

func (s *Server) Dispatch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	t := s.s.GetTool(req.Params.Name)
	if t == nil {
		return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", req.Params.Name)), nil
	}
	return t.Handler(ctx, req)
}

// ListToolNames returns the names of all registered tools.
func (s *Server) ListToolNames() []string {
	all := s.s.ListTools()
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	return names
}

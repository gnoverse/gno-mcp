package mcp

import (
	"context"
	"fmt"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	s      *server.MCPServer
	client client.GnopieClient
}

func New(c client.GnopieClient, a *audit.Log) *Server {
	s := server.NewMCPServer("gno-mcp", "0.1.0", server.WithToolCapabilities(true))
	srv := &Server{s: s, client: c}
	tools.RegisterAll(s, tools.Deps{Client: c, Audit: a})
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

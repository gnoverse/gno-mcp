package mcp

import (
	"context"
	"fmt"

	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	s      *server.MCPServer
	client client.GnopieClient
}

func New(c client.GnopieClient) *Server {
	s := server.NewMCPServer("gno-mcp", "0.1.0", server.WithToolCapabilities(true))
	srv := &Server{s: s, client: c}
	srv.registerHello()
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

func (s *Server) registerHello() {
	tool := mcp.NewTool(
		"gno_hello",
		mcp.WithDescription("Smoke-test tool. Returns a greeting. Removed before v0.1."),
	)
	s.s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("hello from gno-mcp"), nil
	})
}

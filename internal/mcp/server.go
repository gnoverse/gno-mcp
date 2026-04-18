package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Server struct {
	s *server.MCPServer
}

func New() *Server {
	s := server.NewMCPServer(
		"gno-mcp",
		"0.1.0",
		server.WithToolCapabilities(true),
	)
	srv := &Server{s: s}
	srv.registerHello()
	return srv
}

func (s *Server) ServeStdio() error {
	return server.ServeStdio(s.s)
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

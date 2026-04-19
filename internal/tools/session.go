package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	Register(registerSessionCreate)
	Register(registerSessionRevoke)
	Register(registerSessionList)
}

func notImplementedResult(hint string) *mcp.CallToolResult {
	return mcp.NewToolResultError(`{"code":"not_implemented","hint":"` + hint + `"}`)
}

func registerSessionCreate(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_session_create",
		mcp.WithDescription("Create a session key (stub — blocked on upstream session-key PR)."),
		mcp.WithString("name", mcp.Description("Session name")),
		mcp.WithString("scope", mcp.Description("Session scope")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return notImplementedResult("blocked on upstream session-key PR"), nil
	})
}

func registerSessionRevoke(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_session_revoke",
		mcp.WithDescription("Revoke a session key (stub — blocked on upstream session-key PR)."),
		mcp.WithString("session_id", mcp.Description("Session ID to revoke")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return notImplementedResult("blocked on upstream session-key PR"), nil
	})
}

func registerSessionList(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_session_list",
		mcp.WithDescription("List active session keys (stub — blocked on upstream session-key PR)."),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return notImplementedResult("blocked on upstream session-key PR"), nil
	})
}

package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerAuditTail) }

func registerAuditTail(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_audit_tail",
		mcp.WithDescription("Show the most recent audit log entries. Useful for reviewing recent tool activity."),
		mcp.WithString("limit", mcp.Description("Maximum number of entries to return. Defaults to 50.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := req.GetInt("limit", 50)

		entries, err := d.Audit.Tail(limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		b, _ := json.Marshal(entries)
		return mcp.NewToolResultText(string(b)), nil
	})
}

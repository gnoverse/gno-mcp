package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerInspect) }

func registerInspect(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_inspect",
		mcp.WithDescription("Inspect a realm: list files and exported functions with their signatures."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land. Defaults to gno.land.")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Realm package path, e.g. gno.land/r/demo/boards")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		path := req.GetString("path", "")

		info, err := d.Client.Inspect(ctx, network, path)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_inspect", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_inspect", Network: network, Result: "ok"})
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	})
}

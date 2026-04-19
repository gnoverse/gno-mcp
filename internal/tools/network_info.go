package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerNetworkInfo) }

func registerNetworkInfo(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_network_info",
		mcp.WithDescription("Return chain metadata (chain id, RPC URL, height) for a gno.land network resolved from its domain. No key required. Default domain: gno.land."),
		mcp.WithString("domain", mcp.Description("Domain to resolve, e.g. gno.land, staging.gno.land. Defaults to gno.land (mainnet) if empty.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		domain := req.GetString("domain", "gno.land")
		ni, err := d.Client.NetworkInfo(ctx, domain)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_network_info", Network: domain, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}
		_ = d.Audit.Append(audit.Entry{Tool: "gno_network_info", Network: domain, Result: "ok"})
		b, _ := json.Marshal(ni)
		return mcp.NewToolResultText(string(b)), nil
	})
}

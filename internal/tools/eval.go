package tools

import (
	"context"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerEval) }

func registerEval(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_eval",
		mcp.WithDescription("Evaluate a Gno expression on-chain and return the result."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land or staging.gno.land. Defaults to gno.land.")),
		mcp.WithString("expr", mcp.Required(), mcp.Description("Gno expression to evaluate, e.g. gno.land/r/demo/boards.GetPost(1)")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		expr := req.GetString("expr", "")

		result, err := d.Client.Eval(ctx, network, expr)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_eval", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_eval", Network: network, Result: "ok"})
		return mcp.NewToolResultText(untrustedEnvelope("eval", expr, result)), nil
	})
}

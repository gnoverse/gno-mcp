package tools

import (
	"context"
	"strings"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerGet) }

func registerGet(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_get",
		mcp.WithDescription("Flexible getter for realm content or expression evaluation. If path contains '(' it is evaluated as a Gno expression; otherwise the realm is rendered."),
		mcp.WithString("path", mcp.Required(), mcp.Description("Realm path or Gno expression (e.g. gno.land/r/demo/boards or gno.land/r/foo.Bar())")),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land or staging.gno.land. Defaults to gno.land.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path := req.GetString("path", "")
		network := req.GetString("network", "gno.land")

		var result string
		var kind string
		var err error

		if strings.Contains(path, "(") {
			result, err = d.Client.Eval(ctx, network, path)
			kind = "eval"
		} else {
			result, err = d.Client.Render(ctx, network, path)
			kind = "render"
		}

		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_get", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		const maxLen = 4096
		truncated := false
		if len(result) > maxLen {
			result = result[:maxLen]
			truncated = true
		}

		wrapped := untrustedEnvelope(kind, path, result)
		if truncated {
			wrapped += "\n[output truncated, full content at https://" + network + "/" + path + "]"
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_get", Network: network, Result: "ok"})
		return mcp.NewToolResultText(wrapped), nil
	})
}

package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/budget"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerRead) }

func registerRead(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_read",
		mcp.WithDescription("Read realm source code. Optionally slice by symbol, file, or line range."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land. Defaults to gno.land.")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Realm package path, e.g. gno.land/r/demo/boards")),
		mcp.WithString("symbol", mcp.Description("Symbol name to extract (function, type, variable)")),
		mcp.WithString("file", mcp.Description("Specific file name within the realm, e.g. board.gno")),
		mcp.WithString("lines", mcp.Description("Line range to extract, e.g. 10-40")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		path := req.GetString("path", "")
		symbol := req.GetString("symbol", "")
		file := req.GetString("file", "")
		lines := req.GetString("lines", "")

		sliceRequested := symbol != "" || file != "" || lines != ""

		var lineStart, lineEnd int
		if lines != "" {
			parts := strings.SplitN(lines, "-", 2)
			if len(parts) == 2 {
				lineStart, _ = strconv.Atoi(parts[0])
				lineEnd, _ = strconv.Atoi(parts[1])
			}
		}

		result, err := d.Client.Read(ctx, network, path, symbol, file, lineStart, lineEnd)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_read", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_read", Network: network, Result: "ok"})

		gnowebURL := "https://" + network + "/" + path
		br := budget.Apply(result, gnowebURL, sliceRequested)

		if br.Truncated {
			summaryJSON, _ := json.Marshal(br)
			return mcp.NewToolResultText(string(summaryJSON)), nil
		}

		return mcp.NewToolResultText(untrustedEnvelope("source", path, br.Full)), nil
	})
}

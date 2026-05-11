package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerAddressInfo) }

func registerAddressInfo(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_address_info",
		mcp.WithDescription("Get account information for a Gno address: balance, sequence, and recent transactions (capped at 20)."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land. Defaults to gno.land.")),
		mcp.WithString("address", mcp.Required(), mcp.Description("Bech32 Gno address, e.g. g1...")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		address := req.GetString("address", "")

		info, err := d.Client.AddressInfo(ctx, network, address)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_address_info", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Cap recent transactions to 20
		if len(info.Txs) > 20 {
			info.Txs = info.Txs[:20]
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_address_info", Network: network, Result: "ok"})
		b, _ := json.Marshal(info)
		return mcp.NewToolResultText(string(b)), nil
	})
}

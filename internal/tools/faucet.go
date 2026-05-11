package tools

import (
	"context"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerFaucetRequest) }

func registerFaucetRequest(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_faucet_request",
		mcp.WithDescription("Request testnet funds from the network faucet. Not available on mainnet (gno.land)."),
		mcp.WithString("network", mcp.Required(), mcp.Description("Testnet domain, e.g. staging.gno.land. Cannot be gno.land (mainnet has no faucet).")),
		mcp.WithString("address", mcp.Required(), mcp.Description("Bech32 address to fund, e.g. g1...")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "")
		address := req.GetString("address", "")

		if network == "gno.land" {
			return mcp.NewToolResultError("mainnet has no faucet; switch to testnet (staging.gno.land)"), nil
		}

		if err := d.Client.FaucetRequest(ctx, network, address); err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_faucet_request", Network: network, Result: "err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{
			Tool:    "gno_faucet_request",
			Network: network,
			Result:  "ok",
			Args:    map[string]any{"address": address},
		})
		return mcp.NewToolResultText(`{"status":"ok","address":"` + address + `"}`), nil
	})
}

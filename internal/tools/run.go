package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerRun) }

func registerRun(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_run",
		mcp.WithDescription("Run a Gno code snippet on-chain. Simulates first; requires explicit confirm=true for mainnet. Requires a configured signer key."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land or staging.gno.land. Defaults to gno.land.")),
		mcp.WithString("signer", mcp.Description("Key name to sign the transaction")),
		mcp.WithString("code", mcp.Required(), mcp.Description("Gno code to run")),
		mcp.WithBoolean("confirm", mcp.Description("Set to true to broadcast (required for mainnet). Defaults to false.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		signer := req.GetString("signer", "")
		code := req.GetString("code", "")
		confirm := req.GetBool("confirm", false)

		if signer == "" {
			return mcp.NewToolResultError(`{"code":"onboarding_required","message":"no key configured; cannot gno_run","hint":"invoke the gno-onboarding skill to create a testnet key and request faucet funds"}`), nil
		}

		// Represent the run request as a call with __run__ sentinel values.
		runReq := client.CallRequest{
			Network: network,
			Signer:  signer,
			Path:    "__run__",
			Func:    "__run__",
			Args:    []string{code},
		}

		sim, err := d.Client.CallSimulate(ctx, runReq)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_run", Network: network, Signer: signer, Result: "sim_err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		sec := callSecurityBlock{
			Network:              network,
			Signer:               signer,
			SimulatedGas:         sim.GasEstimate,
			EstimatedCost:        sim.EstimatedCost,
			ConfirmationRequired: network == "gno.land" && !confirm,
		}

		if network == "gno.land" && !confirm {
			result := map[string]any{
				"security":   sec,
				"simulation": sim,
				"broadcast":  nil,
			}
			b, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(b)), nil
		}

		broadcast, err := d.Client.CallBroadcast(ctx, runReq)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_run", Network: network, Signer: signer, Result: "broadcast_err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{
			Tool:    "gno_run",
			Network: network,
			Signer:  signer,
			TxHash:  broadcast.TxHash,
			Result:  "ok",
		})

		result := map[string]any{
			"security":   sec,
			"simulation": sim,
			"broadcast":  broadcast,
		}
		b, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(b)), nil
	})
}

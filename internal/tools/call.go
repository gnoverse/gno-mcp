package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerCall) }

func registerCall(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_call",
		mcp.WithDescription("Call a realm function on-chain. Simulates first; requires explicit confirm=true for mainnet. If no signer is provided, signs with the MCP's own session key — but only if that session has been authorized by the user (see gno_auth_status)."),
		mcp.WithString("network", mcp.Description("Network domain, e.g. gno.land or staging.gno.land. Defaults to gno.land.")),
		mcp.WithString("signer", mcp.Description("Key name to sign with. When empty, uses the MCP session key (subject to authorization).")),
		mcp.WithString("path", mcp.Required(), mcp.Description("Realm package path, e.g. gno.land/r/demo/boards")),
		mcp.WithString("func", mcp.Required(), mcp.Description("Function name to call, e.g. NewBoard")),
		mcp.WithArray("args", mcp.Description("Function arguments as strings"), mcp.WithStringItems()),
		mcp.WithString("send", mcp.Description("Coins to send with the call, e.g. 1000000ugnot")),
		mcp.WithBoolean("confirm", mcp.Description("Set to true to broadcast (required for mainnet). Defaults to false.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		network := req.GetString("network", "gno.land")
		signer := req.GetString("signer", "")
		path := req.GetString("path", "")
		funcName := req.GetString("func", "")
		args := req.GetStringSlice("args", nil)
		send := req.GetString("send", "")
		confirm := req.GetBool("confirm", false)

		// Resolve signer: explicit > session > fail with authentication_required.
		if signer == "" {
			if d.Session == nil {
				return mcp.NewToolResultError(`{"code":"onboarding_required","message":"no key configured; cannot gno_call","hint":"invoke the gno-onboarding skill or supply signer="}`), nil
			}
			// Make sure the session has a keypair, then refresh balance before
			// deciding whether to allow the call.
			if err := d.Session.EnsurePending(); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			_ = d.Session.Refresh(ctx)
			state := d.Session.State()
			if state != session.StateAuthenticated {
				code := "authentication_required"
				if state == session.StateExpired {
					code = "authentication_expired"
				}
				payload := session.BuildAuthPayload(d.Session.Status(), "gno-mcp-auth")
				body := map[string]any{
					"code":    code,
					"message": "MCP session is " + string(state) + "; user must authorize the session before writes can proceed",
					"hint":    "open the fund_url (or scan qr_ascii) in your gno wallet to send the threshold balance to the session address; then retry",
					"data":    payload,
				}
				b, _ := json.Marshal(body)
				_ = d.Audit.Append(audit.Entry{Tool: "gno_call", Network: network, Result: "err:" + code})
				return mcp.NewToolResultError(string(b)), nil
			}
			signer = d.Session.Signer()
		}

		callReq := client.CallRequest{
			Network: network,
			Signer:  signer,
			Path:    path,
			Func:    funcName,
			Args:    args,
			Send:    send,
		}

		sim, err := d.Client.CallSimulate(ctx, callReq)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_call", Network: network, Signer: signer, Result: "sim_err:" + err.Error()})
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

		broadcast, err := d.Client.CallBroadcast(ctx, callReq)
		if err != nil {
			_ = d.Audit.Append(audit.Entry{Tool: "gno_call", Network: network, Signer: signer, Result: "broadcast_err:" + err.Error()})
			return mcp.NewToolResultError(err.Error()), nil
		}

		_ = d.Audit.Append(audit.Entry{
			Tool:    "gno_call",
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

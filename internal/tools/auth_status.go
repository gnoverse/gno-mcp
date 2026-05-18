package tools

import (
	"context"
	"encoding/json"

	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/session"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() { Register(registerAuthStatus) }

func registerAuthStatus(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_auth_status",
		mcp.WithDescription("Return the MCP session's authorization state and, when pending, the link/QR the user opens in their gno wallet to authorize this session. Read-only — never broadcasts. Call this any time the LLM needs to surface auth state to the user, or before a write to pre-check."),
		mcp.WithBoolean("ensure_pending", mcp.Description("If the session has no keypair yet, generate one and return the pending payload. Defaults to true so the first call always produces a fund link.")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if d.Session == nil {
			return mcp.NewToolResultError(`{"code":"internal","message":"session manager not wired"}`), nil
		}

		ensure := req.GetBool("ensure_pending", true)
		if ensure {
			if err := d.Session.EnsurePending(); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
		}

		// Best-effort balance refresh. We swallow errors so the user always
		// sees the current (possibly stale) state with a clear pointer at the
		// fund link rather than an opaque RPC error.
		_ = d.Session.Refresh(ctx)

		snap := d.Session.Status()
		out := map[string]any{
			"state":           snap.State,
			"network":         snap.Network,
			"session_address": snap.Address,
			"balance_ugnot":   snap.Balance,
			"threshold_ugnot": snap.Threshold,
			"created_at":      snap.CreatedAt,
			"last_check":      snap.LastCheck,
		}
		if snap.State == session.StatePending || snap.State == session.StateExpired {
			pl := session.BuildAuthPayload(snap, "gno-mcp-auth")
			out["fund_url"] = pl.FundURL
			out["web_fund_url"] = pl.WebFundURL
			out["qr_ascii"] = pl.QRASCII
			out["human_guidance"] = pl.HumanGuidance
		}

		_ = d.Audit.Append(audit.Entry{Tool: "gno_auth_status", Network: snap.Network, Result: "ok"})

		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	})
}

package write

import (
	"context"
	"fmt"
	"strings"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterSessionRevoke registers the gno_session_revoke tool.
// sessionMgr is queried to look up gnomcp-managed sessions; only sessions
// tracked by the manager can be revoked through this tool. resolver supplies
// the profile's chain client so the emitted command carries the live
// --gas-fee; a fee-query failure falls back to the floor rather than
// blocking revocation.
func RegisterSessionRevoke(s *server.Server, sessionMgr *session.Manager, resolver chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_session_revoke",
		Description: "Emits the gnokey command the user must run to revoke a gnomcp-managed " +
			"session on chain. Only sessions previously proposed through gno_session_propose " +
			"(and thus tracked by gnomcp) can be revoked via this tool. If the session is not " +
			"in gnomcp's records, returns session_unmanaged with the manual gnokey command. " +
			"Does NOT broadcast anything — the user must run the returned command. " +
			"Required args: profile, session_address.",
		InputSchema: sessionRevokeInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWritePrep,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  true,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return sessionRevokeHandler(ctx, args, s, sessionMgr, resolver)
		},
	})
}

func sessionRevokeHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	sessionMgr *session.Manager,
	resolver chain.Resolver,
) (server.Result, error) {
	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}

	sessionAddr, err := server.StringArg(args, "session_address")
	if err != nil {
		return server.Result{}, err
	}
	if sessionAddr == "" {
		return server.Result{}, fmt.Errorf("session_address: required")
	}

	meta := sessionMgr.Get(profileName, sessionAddr)
	if meta == nil {
		// Session not in gnomcp records — provide manual fallback.
		manualCmd := fmt.Sprintf(
			"gnokey maketx session revoke \\\n"+
				"  --pubkey <session-pubkey> \\\n"+
				"  --remote %s \\\n"+
				"  --chainid %s \\\n"+
				"  <your-master-key-name>",
			profile.RPCURL,
			profile.ChainID,
		)
		return server.Result{}, &server.ToolError{
			Code: "session_unmanaged",
			Message: fmt.Sprintf(
				"session %q is not managed by gnomcp for profile %q. "+
					"To revoke manually, run:\n\n```\n%s\n```",
				sessionAddr, profileName, manualCmd,
			),
			Extra: map[string]any{
				"profile":         profileName,
				"session_address": sessionAddr,
				"manual_command":  manualCmd,
			},
		}
	}

	// Unlike propose, a fee-query failure must not block revocation — it
	// kills a live credential. Fall back to the floor fee and say so.
	var feeUgnot int64
	var feeNote string
	if c := resolver(profileName); c != nil {
		if fee, ferr := c.GasFeeUgnot(ctx); ferr == nil {
			feeUgnot = fee
		} else {
			feeNote = "NOTE: could not query the live gas price; --gas-fee shows the floor — if the chain rejects the command with an insufficient-fee error, raise --gas-fee and rerun.\n\n"
		}
	}

	cmd := session.FormatGnokeyRevokeCommand(&profile, meta.SessionPubkey, feeUgnot)

	var b strings.Builder
	fmt.Fprintf(&b, "Session revoke command for profile %q.\n\n", profileName)
	fmt.Fprintf(&b, "Session address: %s\n", sessionAddr)
	fmt.Fprintf(&b, "\nTo revoke, run this in a terminal where your master key is available:\n\n")
	fmt.Fprintf(&b, "```\n%s\n```\n\n", cmd)
	b.WriteString(feeNote)
	fmt.Fprintf(&b,
		"After running the command, the session key will be invalidated on chain.\n",
	)

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"state":           "revoke_pending",
			"profile":         profileName,
			"session_address": sessionAddr,
			"session_pubkey":  meta.SessionPubkey,
			"revoke_command":  cmd,
		},
	}, nil
}

func sessionRevokeInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"session_address": map[string]any{
			"type":        "string",
			"description": "Bech32 address of the session to revoke. Use gno_auth_status to list managed sessions.",
		},
	}
	required := []string{"session_address"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

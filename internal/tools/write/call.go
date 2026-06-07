package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterCall registers the gno_call tool.
// sessionMgr provides active sessions for signing; resolver returns the chain
// client for a given profile; alog writes audit entries on every call attempt.
func RegisterCall(s *server.Server, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_call",
		Description: "Calls a public function in a deployed Gno realm (vm/MsgCall). Requires an " +
			"active gnomcp session that covers the target realm (use gno_session_propose if none " +
			"exists). Pass simulate=true to dry-run without spending gas. Still requires an active " +
			"session that covers the target realm. Required args: profile, realm, func. Optional: " +
			"args (array of strings), simulate (bool).",
		InputSchema: callInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return callHandler(ctx, args, s, sessionMgr, resolver, alog)
		},
	})
}

func callHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	sessionMgr *session.Manager,
	resolver chain.Resolver,
	alog *audit.Log,
) (server.Result, error) {
	start := time.Now()

	// ---- Validate args

	profileName, err := stringArg(args, "profile")
	if err != nil {
		return server.Result{}, err
	}
	if profileName == "" {
		return server.Result{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}

	realm, err := stringArg(args, "realm")
	if err != nil {
		return server.Result{}, err
	}
	if realm == "" {
		return server.Result{}, fmt.Errorf("realm: required")
	}

	fn, err := stringArg(args, "func")
	if err != nil {
		return server.Result{}, err
	}
	if fn == "" {
		return server.Result{}, fmt.Errorf("func: required")
	}

	fnArgs, err := stringSliceArg(args, "args")
	if err != nil {
		return server.Result{}, err
	}
	if fnArgs == nil {
		fnArgs = []string{}
	}

	simulate, err := boolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	// ---- Resolve profile + chain client

	profile, ok := s.Config().Profiles[profileName]
	if !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Build args summary for audit

	argsSummary := fmt.Sprintf("realm=%s func=%s args=%v", realm, fn, fnArgs)

	// ---- Pick session (same flow for simulate and broadcast)

	signer, pickErr := sessionMgr.PickSessionForProfile(ctx, resolver, profileName, realm)
	if pickErr != nil {
		if errors.Is(pickErr, session.ErrNoActiveSession) {
			return server.Result{}, &server.ToolError{
				Code: "authentication_required",
				Message: fmt.Sprintf(
					"no active session for profile %q — use gno_session_propose to create one",
					profileName,
				),
				Extra: map[string]any{"profile": profileName},
			}
		}
		if scopeErr, ok := errors.AsType[*session.ErrScopeMismatch](pickErr); ok {
			return server.Result{}, &server.ToolError{
				Code: "scope_mismatch",
				Message: fmt.Sprintf(
					"realm %q is not covered by any active session for profile %q — "+
						"use gno_session_propose with allow_paths=[%q]",
					realm, profileName, realm,
				),
				Extra: map[string]any{
					"profile":         profileName,
					"realm":           realm,
					"available_paths": scopeErr.AvailablePaths,
				},
			}
		}
		return server.Result{}, fmt.Errorf("gno_call: pick session: %w", pickErr)
	}

	sessionAddr := signer.Address()

	// ---- Call (simulate or broadcast)

	cr, callErr := c.Call(ctx, signer, profile.MasterAddress, realm, fn, fnArgs, simulate)
	if callErr != nil {
		if simulate && errors.Is(callErr, chain.ErrSimulateUnsupported) {
			return server.Result{}, &server.ToolError{
				Code:    "simulate_unsupported",
				Message: "this chain client does not support simulate; retry without simulate=true",
				Extra:   map[string]any{"profile": profileName},
			}
		}
		result := "broadcast_err"
		errPrefix := "gno_call broadcast"
		if simulate {
			result = "sim_err"
			errPrefix = "gno_call simulate"
		}
		_ = alog.Append(audit.Entry{
			Tool:           "gno_call",
			Profile:        profileName,
			ArgsSummary:    argsSummary,
			Result:         result,
			Duration:       time.Since(start).Milliseconds(),
			SessionAddress: sessionAddr,
		})
		return server.Result{}, fmt.Errorf("%s: %w", errPrefix, callErr)
	}

	// ---- Update spend + audit (simulate skips spend update)

	// The chain bills the session the full GasFee per tx, not GasUsed, so deduct
	// that to keep local SpendRemaining in sync with the chain (see chain.DefaultGasFeeUgnot).
	if !simulate {
		_ = sessionMgr.UpdateSpend(profileName, sessionAddr, chain.DefaultGasFeeUgnot)
	}

	result := "ok"
	if simulate {
		result = "sim"
	}
	_ = alog.Append(audit.Entry{
		Tool:           "gno_call",
		Profile:        profileName,
		ArgsSummary:    argsSummary,
		Result:         result,
		Duration:       time.Since(start).Milliseconds(),
		SessionAddress: sessionAddr,
	})

	return buildCallResult(cr), nil
}

// buildCallResult constructs the server.Result from a chain.CallResult.
func buildCallResult(cr chain.CallResult) server.Result {
	var b strings.Builder
	if cr.Simulated {
		fmt.Fprintf(&b, "Simulated call result\n\n")
	} else {
		fmt.Fprintf(&b, "Call succeeded\n\n")
		fmt.Fprintf(&b, "TxHash:  %s\n", cr.TxHash)
		fmt.Fprintf(&b, "Height:  %d\n", cr.Height)
	}
	fmt.Fprintf(&b, "GasUsed: %d\n", cr.GasUsed)
	if cr.Result != "" {
		fmt.Fprintf(&b, "Result:  %s\n", cr.Result)
	}

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"tx_hash":   cr.TxHash,
			"height":    cr.Height,
			"result":    cr.Result,
			"gas_used":  cr.GasUsed,
			"simulated": cr.Simulated,
		},
	}
}

func callInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"realm": map[string]any{
			"type":        "string",
			"description": "Fully-qualified Gno realm path to call (e.g. \"gno.land/r/myorg/blog\").",
		},
		"func": map[string]any{
			"type":        "string",
			"description": "Public function name to invoke in the realm.",
		},
		"args": map[string]any{
			"type":        "array",
			"items":       map[string]any{"type": "string"},
			"description": "Positional string arguments for the function. Optional; omit or pass [] for zero-argument functions.",
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the call without broadcasting or spending gas. Still requires an active session that covers the target realm.",
			"default":     false,
		},
	}
	required := []string{"realm", "func"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

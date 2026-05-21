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

// RegisterRun registers the gno_run tool.
// sessionMgr provides active sessions for signing; resolver returns the chain
// client for a given profile; alog writes audit entries on every run attempt.
func RegisterRun(s *server.Server, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_run",
		Description: "Executes ad-hoc Gno code via vm/MsgRun. The code must be a valid Gno package " +
			"(package main with a main() entry point). Requires an active gnomcp session for the " +
			"profile (use gno_session_propose if none exists). Pass simulate=true to dry-run without " +
			"a session or spending gas. Required args: profile, code. Optional: simulate (bool).",
		InputSchema: runInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		Annotations: server.Annotations{
			ReadOnly:    false,
			Destructive: true,
			Idempotent:  false,
			OpenWorld:   true,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return runHandler(ctx, args, s, sessionMgr, resolver, alog)
		},
	})
}

func runHandler(
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

	code, err := stringArg(args, "code")
	if err != nil {
		return server.Result{}, err
	}
	if code == "" {
		return server.Result{}, fmt.Errorf("code: required")
	}

	simulate, err := boolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	// ---- Gate on AllowDangerousTools

	profile, ok := s.Config().Profiles[profileName]
	if !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}
	if !profile.AllowDangerousTools {
		return server.Result{}, &server.ToolError{
			Code:    "dangerous_disabled",
			Message: fmt.Sprintf("profile %q: allow-dangerous-tools is not set — edit profiles.toml to enable write tools", profileName),
			Extra:   map[string]any{"profile": profileName},
		}
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Build args summary for audit

	argsSummary := fmt.Sprintf("code_len=%d", len(code))

	// ---- Simulate path

	if simulate {
		rr, runErr := c.Run(ctx, nil, code, true)
		if runErr != nil {
			if errors.Is(runErr, chain.ErrSimulateUnsupported) {
				return server.Result{}, &server.ToolError{
					Code:    "simulate_unsupported",
					Message: "this chain client does not support simulate; retry without simulate=true",
					Extra:   map[string]any{"profile": profileName},
				}
			}
			_ = alog.Append(audit.Entry{
				Tool:        "gno_run",
				Profile:     profileName,
				ArgsSummary: argsSummary,
				Result:      "sim_err",
				Duration:    time.Since(start).Milliseconds(),
			})
			return server.Result{}, fmt.Errorf("gno_run simulate: %w", runErr)
		}

		_ = alog.Append(audit.Entry{
			Tool:        "gno_run",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      "sim",
			Duration:    time.Since(start).Milliseconds(),
		})

		return buildRunResult(rr), nil
	}

	// ---- Real run path: pick any active session (realm="" = wildcard)

	signer, pickErr := sessionMgr.PickSessionForProfile(ctx, resolver, profileName, "")
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
		return server.Result{}, fmt.Errorf("gno_run: pick session: %w", pickErr)
	}

	sessionAddr := signer.Address()

	// ---- Broadcast

	rr, runErr := c.Run(ctx, signer, code, false)
	if runErr != nil {
		_ = alog.Append(audit.Entry{
			Tool:           "gno_run",
			Profile:        profileName,
			ArgsSummary:    argsSummary,
			Result:         "broadcast_err",
			Duration:       time.Since(start).Milliseconds(),
			SessionAddress: sessionAddr,
		})
		return server.Result{}, fmt.Errorf("gno_run broadcast: %w", runErr)
	}

	// ---- Update spend + audit

	_ = sessionMgr.UpdateSpend(profileName, sessionAddr, rr.GasUsed)

	_ = alog.Append(audit.Entry{
		Tool:           "gno_run",
		Profile:        profileName,
		ArgsSummary:    argsSummary,
		Result:         "ok",
		Duration:       time.Since(start).Milliseconds(),
		SessionAddress: sessionAddr,
	})

	return buildRunResult(rr), nil
}

// buildRunResult constructs the server.Result from a chain.RunResult.
func buildRunResult(rr chain.RunResult) server.Result {
	var b strings.Builder
	if rr.Simulated {
		fmt.Fprintf(&b, "Simulated run result\n\n")
	} else {
		fmt.Fprintf(&b, "Run succeeded\n\n")
		fmt.Fprintf(&b, "TxHash:  %s\n", rr.TxHash)
		fmt.Fprintf(&b, "Height:  %d\n", rr.Height)
	}
	fmt.Fprintf(&b, "GasUsed: %d\n", rr.GasUsed)
	if rr.Output != "" {
		fmt.Fprintf(&b, "Output:  %s\n", rr.Output)
	}

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"tx_hash":   rr.TxHash,
			"height":    rr.Height,
			"output":    rr.Output,
			"gas_used":  rr.GasUsed,
			"simulated": rr.Simulated,
		},
	}
}

func runInputSchema(s *server.Server) map[string]any {
	props := map[string]any{
		"code": map[string]any{
			"type":        "string",
			"description": "Gno source code to execute. Must be a valid Gno package with a package main declaration and a main() entry point.",
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the execution without broadcasting or spending gas. No active session required.",
			"default":     false,
		},
	}
	required := []string{"code"}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterRun registers the gno_run tool.
// ks provides agent signers for local profiles; sessionMgr provides active
// sessions for signing; resolver returns the chain client for a given profile;
// alog writes audit entries on every run attempt.
func RegisterRun(s *server.Server, ks *keystore.Keystore, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_run",
		Description: "Executes ad-hoc Gno code via vm/MsgRun. The code must be a valid Gno package " +
			"(package main with a main() entry point). " +
			"On local and testnet profiles the agent key signs by default (local: the built-in test1 account; " +
			"testnet: a key from gno_key_generate, funded via gno_faucet_fund). " +
			"On other profiles, or when you pass identity=session, an active gnomcp session covering the target realm is required (use gno_session_propose). " +
			"Pass simulate=true to dry-run without spending gas. Required args: " +
			"profile, code. Optional: simulate (bool), identity (\"agent\" or \"session\"). " +
			"The result reports which identity signed; always tell the user which account performed the write.",
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
			return runHandler(ctx, args, s, ks, sessionMgr, resolver, alog)
		},
	})
}

func runHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	ks *keystore.Keystore,
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

	// ---- Resolve identity (default by tier: local/testnet→agent, otherwise→session)

	identity, _ := stringArg(args, "identity")
	if identity == "" {
		if profile.ChainType == profiles.ChainTypeLocal || profile.ChainType == profiles.ChainTypeTestnet {
			identity = "agent"
		} else {
			identity = "session"
		}
	}

	// ---- Build args summary for audit

	argsSummary := fmt.Sprintf("code_len=%d", len(code))

	// ---- Dispatch by identity

	var rr chain.RunResult
	var signerAddr string
	var master string

	switch identity {
	case "agent":
		// ---- Agent branch: sign with the agent's own key (local test1 or testnet generated key)

		agentSigner, ksErr := ks.SignerForProfile(profileName, profile)
		if ksErr != nil {
			if errors.Is(ksErr, keystore.ErrNoAgentKey) {
				return server.Result{}, &server.ToolError{
					Code: "agent_identity_unavailable",
					Message: fmt.Sprintf(
						"no agent key for profile %q — run gno_key_generate (or pass identity=session to act as the user)",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			return server.Result{}, fmt.Errorf("gno_run: signer: %w", ksErr)
		}

		info, infoErr := agentSigner.Info()
		if infoErr != nil {
			return server.Result{}, fmt.Errorf("gno_run: signer info: %w", infoErr)
		}
		signerAddr = info.GetAddress().String()

		if profile.ChainType == profiles.ChainTypeTestnet && !simulate {
			bal, balErr := c.Balance(ctx, signerAddr)
			if balErr != nil {
				return server.Result{}, fmt.Errorf("gno_run: balance check: %w", balErr)
			}
			if bal == 0 {
				return server.Result{}, &server.ToolError{
					Code:    "insufficient_funds",
					Message: fmt.Sprintf("agent testnet account %s is unfunded — run gno_faucet_fund (or send it ugnot), then retry", signerAddr),
					Extra:   map[string]any{"profile": profileName, "address": signerAddr},
				}
			}
		}

		rr, err = c.Run(ctx, agentSigner, code, simulate)
		if err != nil {
			result := "broadcast_err"
			errPrefix := "gno_run broadcast"
			if simulate {
				result = "sim_err"
				errPrefix = "gno_run simulate"
			}
			_ = alog.Append(audit.Entry{
				Tool:        "gno_run",
				Profile:     profileName,
				ArgsSummary: argsSummary,
				Result:      result,
				Duration:    time.Since(start).Milliseconds(),
			})
			return server.Result{}, fmt.Errorf("%s: %w", errPrefix, err)
		}

		// Audit success (no UpdateSpend — agent pays from its own balance)
		auditResult := "ok"
		if simulate {
			auditResult = "sim"
		}
		_ = alog.Append(audit.Entry{
			Tool:        "gno_run",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      auditResult,
			Duration:    time.Since(start).Milliseconds(),
		})

	case "session":
		// ---- Session branch: existing flow preserved byte-for-byte (requires AllowRun=true)

		signer, pickErr := sessionMgr.PickSessionForRun(ctx, resolver, profileName)
		if pickErr != nil {
			if errors.Is(pickErr, session.ErrNoActiveSession) {
				return server.Result{}, &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"no active session with allow_run=true for profile %q — use gno_session_propose with allow_run=true",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			if _, ok := errors.AsType[*session.ErrScopeMismatch](pickErr); ok {
				return server.Result{}, &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"active sessions for profile %q do not authorize MsgRun — use gno_session_propose with allow_run=true",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			return server.Result{}, fmt.Errorf("gno_run: pick session: %w", pickErr)
		}

		sessionAddr := signer.Address()
		signerAddr = sessionAddr
		master = profile.MasterAddress

		rr, err = c.RunAsUser(ctx, signer, profile.MasterAddress, code, simulate)
		if err != nil {
			if simulate && errors.Is(err, chain.ErrSimulateUnsupported) {
				return server.Result{}, &server.ToolError{
					Code:    "simulate_unsupported",
					Message: "this chain client does not support simulate; retry without simulate=true",
					Extra:   map[string]any{"profile": profileName},
				}
			}
			result := "broadcast_err"
			errPrefix := "gno_run broadcast"
			if simulate {
				result = "sim_err"
				errPrefix = "gno_run simulate"
			}
			_ = alog.Append(audit.Entry{
				Tool:           "gno_run",
				Profile:        profileName,
				ArgsSummary:    argsSummary,
				Result:         result,
				Duration:       time.Since(start).Milliseconds(),
				SessionAddress: sessionAddr,
			})
			return server.Result{}, fmt.Errorf("%s: %w", errPrefix, err)
		}

		// Update spend + audit (simulate skips spend update).
		// The chain bills the session the full GasFee per tx, not GasUsed, so deduct
		// that to keep local SpendRemaining in sync with the chain (see chain.DefaultGasFeeUgnot).
		if !simulate {
			_ = sessionMgr.UpdateSpend(profileName, sessionAddr, chain.DefaultGasFeeUgnot)
		}

		auditResult := "ok"
		if simulate {
			auditResult = "sim"
		}
		_ = alog.Append(audit.Entry{
			Tool:           "gno_run",
			Profile:        profileName,
			ArgsSummary:    argsSummary,
			Result:         auditResult,
			Duration:       time.Since(start).Milliseconds(),
			SessionAddress: sessionAddr,
		})

	default:
		return server.Result{}, fmt.Errorf("identity: must be \"agent\" or \"session\", got %q", identity)
	}

	// ---- Build result with identity metadata

	out := buildRunResult(rr)
	out.Text = signedByLine(identity, signerAddr, master, profile.ChainType) + "\n\n" + out.Text
	if out.StructuredContent == nil {
		out.StructuredContent = map[string]any{}
	}
	out.StructuredContent["identity"] = identity
	out.StructuredContent["signer_address"] = signerAddr
	if identity == "session" {
		out.StructuredContent["master_address"] = master
	}
	return out, nil
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
			"description": "When true, dry-run the execution without broadcasting or spending gas. On testnet/mainnet still requires an active session for the profile; on local the agent key signs.",
			"default":     false,
		},
		"identity": map[string]any{
			"type":        "string",
			"enum":        []string{"agent", "session"},
			"description": "Who signs: agent (the agent's own key — local test1 or testnet generated key) or session (act as the user via a master-bound session). Default: agent on local and testnet, session otherwise.",
		},
	}
	required := []string{"code"}
	addWritableProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

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

// RegisterCall registers the gno_call tool.
// ks provides agent signers for local profiles; sessionMgr provides active
// sessions for signing; resolver returns the chain client for a given profile;
// alog writes audit entries on every call attempt.
func RegisterCall(s *server.Server, ks *keystore.Keystore, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_call",
		Description: "Calls a public function in a deployed Gno realm (vm/MsgCall). On local " +
			"profiles the agent key signs directly (no session required). On testnet/mainnet " +
			"profiles an active gnomcp session that covers the target realm is required (use " +
			"gno_session_propose if none exists). Pass simulate=true to dry-run without spending " +
			"gas. Required args: profile, realm, func. Optional: args (array of strings), " +
			"simulate (bool), identity (\"agent\" or \"session\"). " +
			"The result reports which identity signed; always tell the user which account performed the write.",
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
			return callHandler(ctx, args, s, ks, sessionMgr, resolver, alog)
		},
	})
}

func callHandler(
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

	argsSummary := fmt.Sprintf("realm=%s func=%s args=%v", realm, fn, fnArgs)

	// ---- Dispatch by identity

	var cr chain.CallResult
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
			return server.Result{}, fmt.Errorf("gno_call: signer: %w", ksErr)
		}

		info, infoErr := agentSigner.Info()
		if infoErr != nil {
			return server.Result{}, fmt.Errorf("gno_call: signer info: %w", infoErr)
		}
		signerAddr = info.GetAddress().String()

		if profile.ChainType == profiles.ChainTypeTestnet && !simulate {
			bal, balErr := c.Balance(ctx, signerAddr)
			if balErr != nil {
				return server.Result{}, fmt.Errorf("gno_call: balance check: %w", balErr)
			}
			if bal == 0 {
				return server.Result{}, &server.ToolError{
					Code:    "insufficient_funds",
					Message: fmt.Sprintf("agent testnet account %s is unfunded — run gno_faucet_fund (or send it ugnot), then retry", signerAddr),
					Extra:   map[string]any{"profile": profileName, "address": signerAddr},
				}
			}
		}

		cr, err = c.Call(ctx, agentSigner, realm, fn, fnArgs, simulate)
		if err != nil {
			result := "broadcast_err"
			errPrefix := "gno_call broadcast"
			if simulate {
				result = "sim_err"
				errPrefix = "gno_call simulate"
			}
			_ = alog.Append(audit.Entry{
				Tool:        "gno_call",
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
			Tool:        "gno_call",
			Profile:     profileName,
			ArgsSummary: argsSummary,
			Result:      auditResult,
			Duration:    time.Since(start).Milliseconds(),
		})

	case "session":
		// ---- Session branch: existing flow preserved byte-for-byte

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
		signerAddr = sessionAddr
		master = profile.MasterAddress

		cr, err = c.CallAsUser(ctx, signer, profile.MasterAddress, realm, fn, fnArgs, simulate)
		if err != nil {
			if simulate && errors.Is(err, chain.ErrSimulateUnsupported) {
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
			Tool:           "gno_call",
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

	out := buildCallResult(cr)
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
			"description": "When true, dry-run the call without broadcasting or spending gas. On testnet/mainnet still requires an active session covering the target realm; on local the agent key signs.",
			"default":     false,
		},
		"identity": map[string]any{
			"type":        "string",
			"enum":        []string{"agent", "session"},
			"description": "Who signs: agent (the agent's own key — local test1 or testnet generated key) or session (act as the user via a master-bound session). Default: agent on local and testnet, session otherwise.",
		},
	}
	required := []string{"realm", "func"}
	addWritableProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

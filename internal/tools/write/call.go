package write

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gnoclient "github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// RegisterCall registers the gno_call tool.
// ks provides agent signers for local profiles; sessionMgr provides active
// sessions for signing; resolver returns the chain client for a given profile;
// alog writes audit entries on every call attempt.
func RegisterCall(s *server.Server, ks *keystore.Keystore, sessionMgr *session.Manager, resolver chain.Resolver, alog *audit.Log) {
	s.Registry().Add(&server.Tool{
		Name: "gno_call",
		Description: "Calls a public function in a deployed Gno realm (vm/MsgCall). " +
			"On local and testnet profiles the agent key signs by default (local: the built-in test1 account; " +
			"testnet: a key from gno_key_generate, funded via gno_faucet_fund). " +
			"Pass identity=session to act as the user instead — that requires an active gnomcp session covering the target realm (use gno_session_propose). " +
			"Pass simulate=true to dry-run without spending gas. Required args: profile, realm, func. " +
			"Optional: args (array of strings), send (coins to attach for a payable function, e.g. \"5000000ugnot\"), " +
			"simulate (bool), identity (\"agent\" or \"session\"). " +
			"The result reports which identity signed (tell the user which account performed the write) and an " +
			"equivalent gnokey command for transparency — illustrative only, since gnomcp already signed and broadcast the tx.",
		InputSchema: callInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWrite,
		SelfAudited: true,
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

	// One audit record per invocation, written on every return path — including the
	// early validation and pre-check denials — because SelfAudited makes the MCP
	// adapter skip its generic line. auditResult defaults to a denial; the dispatch
	// paths overwrite it.
	var (
		profileName string
		argsSummary string
		sessionAddr string
		auditResult = "tool_err"
	)
	defer func() {
		alog.Record(audit.Entry{
			Tool:           "gno_call",
			Profile:        profileName,
			ArgsSummary:    argsSummary,
			Result:         auditResult,
			Duration:       time.Since(start).Milliseconds(),
			SessionAddress: sessionAddr,
		})
	}()

	// ---- Validate args

	profileName, profile, err := requireProfile(args, s)
	if err != nil {
		return server.Result{}, err
	}
	keyName, err := keyArg(args)
	if err != nil {
		return server.Result{}, err
	}

	realm, err := server.StringArg(args, "realm")
	if err != nil {
		return server.Result{}, err
	}
	if realm == "" {
		return server.Result{}, fmt.Errorf("realm: required")
	}

	fn, err := server.StringArg(args, "func")
	if err != nil {
		return server.Result{}, err
	}
	if fn == "" {
		return server.Result{}, fmt.Errorf("func: required")
	}

	fnArgs, err := server.StringSliceArg(args, "args")
	if err != nil {
		return server.Result{}, err
	}
	if fnArgs == nil {
		fnArgs = []string{}
	}

	simulate, err := server.BoolArg(args, "simulate")
	if err != nil {
		return server.Result{}, err
	}

	send, err := server.StringArg(args, "send")
	if err != nil {
		return server.Result{}, err
	}
	if err := chain.ValidateSendCoins(send); err != nil {
		return server.Result{}, &server.ToolError{
			Code:    "invalid_send",
			Message: fmt.Sprintf("send %q is not a valid coin amount — use an amount like \"5000000ugnot\"", send),
			Extra:   map[string]any{"send": send},
		}
	}

	// ---- Resolve chain client

	c := resolver(profileName)
	if c == nil {
		return server.Result{}, fmt.Errorf("profile %q: no chain client available", profileName)
	}

	// ---- Build args summary for audit (arg values are NOT logged — they routinely
	// carry addresses and amounts; only the count is recorded)

	argsSummary = fmt.Sprintf("realm=%s func=%s nargs=%d", realm, fn, len(fnArgs))
	if send != "" {
		argsSummary += " send=" + send
	}

	// ---- Dispatch by identity

	var cr chain.CallResult
	identityArg, _ := server.StringArg(args, "identity")
	identity, signerAddr, master, err := dispatchWriteTx(ctx, identityArg, writeTxDispatch{
		tool:        "gno_call",
		noKeyHint:   "run gno_key_generate (or pass identity=session to act as the user)",
		profileName: profileName,
		keyName:     keyName,
		profile:     profile,
		simulate:    simulate,
		c:           c,
		ks:          ks,
		sessionMgr:  sessionMgr,
		pickSession: func(ctx context.Context) (chain.Signer, string, error) {
			return sessionMgr.PickSessionForProfile(ctx, resolver, profileName, realm)
		},
		mapPickErr: func(pickErr error) error {
			if errors.Is(pickErr, session.ErrNoActiveSession) {
				return &server.ToolError{
					Code: "authentication_required",
					Message: fmt.Sprintf(
						"no active session for profile %q — use gno_session_propose to create one",
						profileName,
					),
					Extra: map[string]any{"profile": profileName},
				}
			}
			if scopeErr, ok := errors.AsType[*session.ErrScopeMismatch](pickErr); ok {
				return &server.ToolError{
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
			return nil
		},
		agentOp: func(ctx context.Context, signer gnoclient.Signer) error {
			var opErr error
			cr, opErr = c.Call(ctx, signer, realm, fn, fnArgs, send, simulate)
			return opErr
		},
		sessionOp: func(ctx context.Context, signer chain.Signer, master string) error {
			var opErr error
			cr, opErr = c.CallAsUser(ctx, signer, master, realm, fn, fnArgs, send, simulate)
			return opErr
		},
		auditResult: &auditResult,
		sessionAddr: &sessionAddr,
	})
	if err != nil {
		return server.Result{}, err
	}

	gkCmd := chain.GnokeyCmd{
		Sub: "call", PkgPath: realm, Func: fn, Args: fnArgs, Send: send,
		RPC: profile.RPCURL, ChainID: profile.ChainID,
		Signer: signerAddr, Master: master, Simulate: simulate,
	}.String()
	return attachGnokeyCmd(
		decorateWriteResult(buildCallResult(cr, realm), identity, signerAddr, master, profile.IsLocal()),
		gkCmd,
	), nil
}

// buildCallResult constructs the server.Result from a chain.CallResult. The
// realm function's return value is realm-authored, so the text rendering wraps
// it in the untrusted envelope; the structured "result" field stays raw (the
// machine-readable channel, documented in docs/security.md §4).
func buildCallResult(cr chain.CallResult, realm string) server.Result {
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
		fmt.Fprintf(&b, "Result:\n%s\n", untrusted.Wrap(cr.Result, "call_result", realm))
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
			"type":  "array",
			"items": map[string]any{"type": "string"},
			"description": "Positional string arguments for the function. Optional; omit or pass [] for zero-argument functions. " +
				"e.g. [\"42\", \"g1abc...\"] — all arguments are passed as strings; numbers, addresses, and booleans are stringified.",
		},
		"send": map[string]any{
			"type": "string",
			"description": "Coins to attach to the call for a payable function (one that reads std.OriginSend() / banker coins, e.g. a Bid or Buy). " +
				"Single-denomination amount, e.g. \"5000000ugnot\". Optional; omit or \"\" sends nothing. " +
				"For identity=session the coins are spent from the user's master account and count against the session spend_limit. " +
				"To move plain ugnot between your own keys without calling a function, use gno_key_send instead.",
		},
		"simulate": map[string]any{
			"type":        "boolean",
			"description": "When true, dry-run the call without broadcasting or spending gas. The acting identity still signs (agent key or session), but nothing is broadcast.",
			"default":     false,
		},
		"identity": map[string]any{
			"type":        "string",
			"enum":        []string{"agent", "session"},
			"description": "Who signs: agent (the agent's own key — local test1 or testnet generated key) or session (act as the user via a master-bound session). Default: agent; pass session to act as the user. The key arg applies to identity=agent only and is rejected with identity=session.",
		},
	}
	required := []string{"realm", "func"}
	addWritableProfileArg(s, props, &required)
	addOptionalKeyArg(props)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
